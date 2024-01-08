import json
import os
import pathlib
import platform
import re
import subprocess
import tempfile
import uuid
from typing import NamedTuple

import pytest

# local test utils
import testutil
from vm import AWS, QEMU

if not testutil.has_executable("podman"):
    pytest.skip("no podman, skipping integration tests that required podman", allow_module_level=True)

if not testutil.can_start_rootful_containers():
    pytest.skip("tests require to be able to run rootful containers (try: sudo)", allow_module_level=True)

# building an ELN image needs x86_64-v3 to work, we use avx2 as a proxy
# to detect if we have x86-64-v3 (not perfect but should be good enough)
if platform.system() == "Linux" and platform.machine() == "x86_64" and not testutil.has_x86_64_v3_cpu():
    pytest.skip("need x86_64-v3 capable CPU", allow_module_level=True)


@pytest.fixture(name="build_container", scope="session")
def build_container_fixture():
    """Build a container from the Containerfile and returns the name"""
    if tag_from_env := os.getenv("BIB_TEST_BUILD_CONTAINER_TAG"):
        return tag_from_env

    container_tag = "bootc-image-builder-test"
    subprocess.check_call([
        "podman", "build",
        "-f", "Containerfile",
        "-t", container_tag,
    ])
    return container_tag


# image types to test
SUPPORTED_IMAGE_TYPES = ["qcow2", "ami"]


class ImageBuildResult(NamedTuple):
    img_type: str
    img_path: str
    username: str
    password: str
    journal_output: str
    metadata: dict = {}


@pytest.fixture(name="image_type", scope="session")
def image_type_fixture(tmpdir_factory, build_container, request):
    """
    Build an image inside the passed build_container and return an
    ImageBuildResult with the resulting image path and user/password
    """
    # TODO: make this another indirect fixture input, e.g. by making
    # making "image_type" an "image" tuple (type, container_ref_to_test)
    container_to_build_ref = os.getenv(
        "BIB_TEST_BOOTC_CONTAINER_TAG",
        "quay.io/centos-bootc/fedora-bootc:eln",
    )

    # image_type is passed via special pytest parameter fixture
    image_type = request.param

    username = "test"
    password = "password"

    output_path = pathlib.Path(tmpdir_factory.mktemp("data")) / "output"
    output_path.mkdir(exist_ok=True)

    journal_log_path = output_path / "journal.log"
    artifact = {
        "qcow2": pathlib.Path(output_path) / "qcow2/disk.qcow2",
        "ami": pathlib.Path(output_path) / "image/disk.raw",
    }
    assert len(artifact) == len(SUPPORTED_IMAGE_TYPES), \
        "please keep artifact mapping and supported images in sync"
    generated_img = artifact[image_type]

    # if the fixture already ran and generated an image, use that
    if generated_img.exists():
        journal_output = journal_log_path.read_text(encoding="utf8")
        return ImageBuildResult(image_type, generated_img, username, password, journal_output)

    # no image yet, build it
    CFG = {
        "blueprint": {
            "customizations": {
                "user": [
                    {
                        "name": username,
                        "password": password,
                        "groups": ["wheel"],
                    },
                ],
            },
        },
    }

    config_json_path = output_path / "config.json"
    config_json_path.write_text(json.dumps(CFG), encoding="utf-8")

    cursor = testutil.journal_cursor()

    upload_args = []
    creds_args = []
    with tempfile.TemporaryDirectory() as tempdir:
        if image_type == "ami":
            upload_args = [
                f"--aws-ami-name=bootc-image-builder-test-{str(uuid.uuid4())}",
                f"--aws-region={testutil.AWS_REGION}",
                "--aws-bucket=bootc-image-builder-ci",
            ]

            creds_file = pathlib.Path(tempdir) / "aws.creds"
            testutil.write_aws_creds(creds_file)
            creds_args = ["-v", f"{creds_file}:/root/.aws/credentials:ro",
                          "--env", "AWS_PROFILE=default"]

        # run container to deploy an image into a bootable disk and upload to a cloud service if applicable
        subprocess.check_call([
            "podman", "run", "--rm",
            "--privileged",
            "--security-opt", "label=type:unconfined_t",
            "-v", f"{output_path}:/output",
            "-v", "/store",  # share the cache between builds
            *creds_args,
            build_container,
            container_to_build_ref,
            "--config", "/output/config.json",
            "--type", image_type,
            *upload_args,
        ])
    journal_output = testutil.journal_after_cursor(cursor)
    metadata = {}
    if image_type == "ami":
        metadata["ami_id"] = parse_ami_id_from_log(journal_output)

        def del_ami():
            testutil.deregister_ami(metadata["ami_id"])
        request.addfinalizer(del_ami)

    journal_log_path.write_text(journal_output, encoding="utf8")

    return ImageBuildResult(image_type, generated_img, username, password, journal_output, metadata)


def test_container_builds(build_container):
    output = subprocess.check_output([
        "podman", "images", "-n", build_container], encoding="utf-8")
    assert build_container in output


@pytest.mark.parametrize("image_type", SUPPORTED_IMAGE_TYPES, indirect=["image_type"])
def test_image_is_generated(image_type):
    assert image_type.img_path.exists(), "output file missing, dir "\
        f"content: {os.listdir(os.fspath(image_type.img_path))}"


@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("image_type", SUPPORTED_IMAGE_TYPES, indirect=["image_type"])
def test_image_boots(image_type):
    with QEMU(image_type.img_path) as test_vm:
        exit_status, _ = test_vm.run("true", user=image_type.username, password=image_type.password)
        assert exit_status == 0
        exit_status, output = test_vm.run("echo hello", user=image_type.username, password=image_type.password)
        assert exit_status == 0
        assert "hello" in output


def log_has_osbuild_selinux_denials(log):
    OSBUID_SELINUX_DENIALS_RE = re.compile(r"(?ms)avc:\ +denied.*osbuild")
    return re.search(OSBUID_SELINUX_DENIALS_RE, log)


def parse_ami_id_from_log(log_output):
    ami_id_re = re.compile(r"AMI registered: (?P<ami_id>ami-[a-z0-9]+)\n")
    ami_ids = ami_id_re.findall(log_output)
    assert len(ami_ids) > 0
    return ami_ids[0]


def test_osbuild_selinux_denials_re_works():
    fake_log = (
        'Dec 05 07:19:39 other log msg\n'
        'Dec 05 07:19:39 fedora audit: SELINUX_ERR'
        ' op=security_bounded_transition seresult=denied'
        ' oldcontext=system_u:system_r:install_t:s0:c42,c355'
        ' newcontext=system_u:system_r:mount_t:s0:c42,c355\n'
        'Dec 06 16:00:54 internal audit[14368]: AVC avc:  denied '
        '{ nnp_transition nosuid_transition } for  pid=14368 '
        'comm="org.osbuild.ost" scontext=system_u:system_r:install_t:s0:'
        'c516,c631 tcontext=system_u:system_r:mount_t:s0:c516,c631 '
        'tclass=process2 permissive=0'
    )
    assert log_has_osbuild_selinux_denials(fake_log)
    assert not log_has_osbuild_selinux_denials("some\nrandom\nlogs")


def has_selinux():
    return testutil.has_executable("selinuxenabled") and subprocess.run("selinuxenabled").returncode == 0


@pytest.mark.skipif(not has_selinux(), reason="selinux not enabled")
@pytest.mark.parametrize("image_type", SUPPORTED_IMAGE_TYPES, indirect=["image_type"])
def test_image_build_without_se_linux_denials(image_type):
    # the journal always contains logs from the image building
    assert image_type.journal_output != ""
    assert not log_has_osbuild_selinux_denials(image_type.journal_output), \
        f"denials in log {image_type.journal_output}"
