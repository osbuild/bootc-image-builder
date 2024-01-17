import json
import os
import pathlib
import platform
import re
import shutil
import subprocess
import tempfile
import uuid
from typing import NamedTuple

import pytest

# local test utils
import testutil
from containerbuild import build_container_fixture  # noqa: F401
from testcases import gen_testcases
from vm import AWS, QEMU

if not testutil.has_executable("podman"):
    pytest.skip("no podman, skipping integration tests that required podman", allow_module_level=True)

if not testutil.can_start_rootful_containers():
    pytest.skip("tests require to be able to run rootful containers (try: sudo)", allow_module_level=True)

# building an ELN image needs x86_64-v3 to work, we use avx2 as a proxy
# to detect if we have x86-64-v3 (not perfect but should be good enough)
if platform.system() == "Linux" and platform.machine() == "x86_64" and not testutil.has_x86_64_v3_cpu():
    pytest.skip("need x86_64-v3 capable CPU", allow_module_level=True)


class ImageBuildResult(NamedTuple):
    img_type: str
    img_path: str
    img_arch: str
    username: str
    password: str
    bib_output: str
    journal_output: str
    metadata: dict = {}


@pytest.fixture(scope='session')
def shared_tmpdir(tmpdir_factory):
    tmp_path = pathlib.Path(tmpdir_factory.mktemp("shared"))
    yield tmp_path


@pytest.fixture(name="image_type", scope="session")
def image_type_fixture(shared_tmpdir, build_container, request, force_aws_upload):
    """
    Build an image inside the passed build_container and return an
    ImageBuildResult with the resulting image path and user/password
    """
    # image_type is passed via special pytest parameter fixture
    if request.param.count(",") == 2:
        container_ref, image_type, target_arch = request.param.split(",")
    elif request.param.count(",") == 1:
        container_ref, image_type = request.param.split(",")
        target_arch = None
    else:
        raise ValueError(f"cannot parse {request.param.count}")

    username = "test"
    password = "password"

    # image_type can be long and the qmp socket (that has a limit of 100ish
    # AF_UNIX) is derrived from the path
    # so hash the image_type instead of just using it
    for_image_type = request.param
    output_path = shared_tmpdir / format(abs(hash(for_image_type)), "x")
    output_path.mkdir(exist_ok=True)

    journal_log_path = output_path / "journal.log"
    bib_output_path = output_path / "bib-output.log"
    artifact = {
        "qcow2": pathlib.Path(output_path) / "qcow2/disk.qcow2",
        "ami": pathlib.Path(output_path) / "image/disk.raw",
        "raw": pathlib.Path(output_path) / "image/disk.raw",
        "anaconda-iso": pathlib.Path(output_path) / "bootiso/install.iso",
    }
    assert len(artifact) == len(set(t.split(",")[1] for t in gen_testcases("all"))), \
        "please keep artifact mapping and supported images in sync"
    generated_img = artifact[image_type]

    if generated_img.exists():
        print(f"NOTE: reusing cached image {generated_img}")
        journal_output = journal_log_path.read_text(encoding="utf8")
        bib_output = bib_output_path.read_text(encoding="utf8")
        yield ImageBuildResult(image_type, generated_img, target_arch, username, password, bib_output, journal_output)
        return

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
    target_arch_args = []
    if target_arch:
        target_arch_args = ["--target-arch", target_arch]

    with tempfile.TemporaryDirectory() as tempdir:
        if image_type == "ami":
            creds_file = pathlib.Path(tempdir) / "aws.creds"
            if testutil.write_aws_creds(creds_file):
                creds_args = ["-v", f"{creds_file}:/root/.aws/credentials:ro",
                              "--env", "AWS_PROFILE=default"]

                upload_args = [
                    f"--aws-ami-name=bootc-image-builder-test-{str(uuid.uuid4())}",
                    f"--aws-region={testutil.AWS_REGION}",
                    "--aws-bucket=bootc-image-builder-ci",
                ]
            elif force_aws_upload:
                # upload forced but credentials aren't set
                raise RuntimeError("AWS credentials not available (upload forced)")

        # run container to deploy an image into a bootable disk and upload to a cloud service if applicable
        p = subprocess.Popen([
            "podman", "run", "--rm",
            "--privileged",
            "--security-opt", "label=type:unconfined_t",
            "-v", f"{output_path}:/output",
            "-v", "/store",  # share the cache between builds
            *creds_args,
            build_container,
            container_ref,
            "--config", "/output/config.json",
            "--type", image_type,
            *upload_args,
            *target_arch_args,
        ], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        # not using subprocss.check_output() to ensure we get live output
        # during the text
        bib_output = ""
        while True:
            line = p.stdout.readline()
            if not line:
                break
            print(line, end="")
            bib_output += line
        p.wait(timeout=10)

    journal_output = testutil.journal_after_cursor(cursor)
    metadata = {}
    if image_type == "ami" and upload_args:
        metadata["ami_id"] = parse_ami_id_from_log(journal_output)

        def del_ami():
            testutil.deregister_ami(metadata["ami_id"])
        request.addfinalizer(del_ami)

    journal_log_path.write_text(journal_output, encoding="utf8")
    bib_output_path.write_text(bib_output, encoding="utf8")

    yield ImageBuildResult(
        image_type, generated_img, target_arch, username, password,
        bib_output, journal_output, metadata)
    # Try to cache as much as possible
    disk_usage = shutil.disk_usage(generated_img)
    print(f"NOTE: disk usage after {generated_img}: {disk_usage.free / 1_000_000} / {disk_usage.total / 1_000_000}")
    if disk_usage.free < 1_000_000_000:
        print(f"WARNING: running low on disk space, removing {generated_img}")
        generated_img.unlink()
    subprocess.run(["podman", "rmi", container_ref], check=False)
    return


def test_container_builds(build_container):
    output = subprocess.check_output([
        "podman", "images", "-n", build_container], encoding="utf-8")
    assert build_container in output


@pytest.mark.parametrize("image_type", gen_testcases("direct-boot"), indirect=["image_type"])
def test_image_is_generated(image_type):
    assert image_type.img_path.exists(), "output file missing, dir "\
        f"content: {os.listdir(os.fspath(image_type.img_path))}"


@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("image_type", gen_testcases("direct-boot"), indirect=["image_type"])
def test_image_boots(image_type):
    with QEMU(image_type.img_path, arch=image_type.img_arch) as test_vm:
        exit_status, _ = test_vm.run("true", user=image_type.username, password=image_type.password)
        assert exit_status == 0
        exit_status, output = test_vm.run("echo hello", user=image_type.username, password=image_type.password)
        assert exit_status == 0
        assert "hello" in output


@pytest.mark.parametrize("image_type", gen_testcases("ami-boot"), indirect=["image_type"])
def test_ami_boots_in_aws(image_type, force_aws_upload):
    if not testutil.write_aws_creds("/dev/null"):  # we don't care about the file, just the variables being there
        if force_aws_upload:
            # upload forced but credentials aren't set
            raise RuntimeError("AWS credentials not available")
        pytest.skip("AWS credentials not available (upload not forced)")

    # check that upload progress is in the output log. Uploads looks like:
    #
    # Uploading /output/image/disk.raw to bootc-image-builder-ci:aac64b64-6e57-47df-9730-54763061d84b-disk.raw
    #  0 B / 10.00 GiB    0.00%
    # In the tests with no pty no progress bar is shown in the output just
    # xx / yy zz%
    assert " 100.00%\n" in image_type.bib_output
    with AWS(image_type.metadata["ami_id"]) as test_vm:
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
@pytest.mark.parametrize("image_type", gen_testcases("direct-boot"), indirect=["image_type"])
def test_image_build_without_se_linux_denials(image_type):
    # the journal always contains logs from the image building
    assert image_type.journal_output != ""
    assert not log_has_osbuild_selinux_denials(image_type.journal_output), \
        f"denials in log {image_type.journal_output}"


@pytest.mark.skip(reason="see https://github.com/osbuild/bootc-image-builder/issues/233")
@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("image_type", gen_testcases("anaconda-iso"), indirect=["image_type"])
def test_iso_installs(image_type):
    installer_iso_path = image_type.img_path
    test_disk_path = installer_iso_path.with_name("test-disk.img")
    with open(test_disk_path, "w") as fp:
        fp.truncate(10_1000_1000_1000)
    # install to test disk
    with QEMU(test_disk_path, cdrom=installer_iso_path) as vm:
        vm.start(wait_event="qmp:RESET", snapshot=False, use_ovmf=True)
        vm.force_stop()
    # boot test disk and do extremly simple check
    with QEMU(test_disk_path) as vm:
        vm.start(use_ovmf=True)
        exit_status, _ = vm.run("true", user=image_type.username, password=image_type.password)
        assert exit_status == 0
