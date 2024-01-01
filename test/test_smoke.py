import json
import os
import pathlib
import re
import subprocess
from typing import NamedTuple

import pytest

# local test utils
import testutil
from vm import VM

if not testutil.has_executable("podman"):
    pytest.skip("no podman, skipping integration tests that required podman", allow_module_level=True)

if os.getuid() != 0:
    pytest.skip("tests require root to run", allow_module_level=True)

# building an ELN image needs x86_64-v3 to work, we use avx2 as a proxy
# to detect if we have x86-64-v3 (not perfect but should be good enough)
if not testutil.has_x86_64_v3_cpu():
    pytest.skip("need x86_64-v3 capable CPU", allow_module_level=True)


@pytest.fixture(name="build_container", scope="session")
def build_container_fixture():
    """Build a container from the Containerfile and returns the name"""
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
    img_path: str
    username: str
    password: str
    journal_output: str


@pytest.fixture(name="image_type", scope="session")
def image_type_fixture(tmpdir_factory, build_container, request):
    """
    Build an image inside the passed build_container and return an
    ImageBuildResult with the resulting image path and user/password
    """
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
        return ImageBuildResult(generated_img, username, password, journal_output)

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
    # run container to deploy an image into output/qcow2/disk.qcow2
    subprocess.check_call([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{output_path}:/output",
        "-v", "/store",  # share the cache between builds
        build_container,
        "quay.io/centos-bootc/fedora-bootc:eln",
        "--config", "/output/config.json",
        "--type", image_type,
    ])
    journal_output = testutil.journal_after_cursor(cursor)
    journal_log_path.write_text(journal_output, encoding="utf8")

    return ImageBuildResult(generated_img, username, password, journal_output)


def test_container_builds(build_container):
    output = subprocess.check_output([
        "podman", "images", "-n", build_container], encoding="utf-8")
    assert build_container in output


@pytest.mark.parametrize("image_type", SUPPORTED_IMAGE_TYPES, indirect=["image_type"])
def test_image_is_generated(image_type):
    assert image_type.img_path.exists(), "output file missing, dir "\
        f"content: {os.listdir(os.fspath(image_type.img_path))}"


@pytest.mark.parametrize("image_type", SUPPORTED_IMAGE_TYPES, indirect=["image_type"])
def test_image_boots(image_type):
    with VM(image_type.img_path) as test_vm:
        exit_status, _ = test_vm.run("true", user=image_type.username, password=image_type.password)
        assert exit_status == 0
        exit_status, output = test_vm.run("echo hello", user=image_type.username, password=image_type.password)
        assert exit_status == 0
        assert "hello" in output


def log_has_osbuild_selinux_denials(log):
    OSBUID_SELINUX_DENIALS_RE = re.compile(r"(?ms)avc:\ +denied.*osbuild")
    return re.search(OSBUID_SELINUX_DENIALS_RE, log)


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
