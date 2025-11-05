import os
import random
import json
import platform
import string
import subprocess
import textwrap
from contextlib import ExitStack

import pytest
# local test utils
import testutil
from containerbuild import build_container_fixture, make_container    # pylint: disable=unused-import
from testcases import gen_testcases
from test_build_disk import (
    assert_kernel_args,
    ImageBuildResult,
)
from test_build_disk import (  # pylint: disable=unused-import
    gpg_conf_fixture,
    image_type_fixture,
    registry_conf_fixture,
    shared_tmpdir_fixture,
)
from vmtest.vm import QEMU


@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("image_type", gen_testcases("anaconda-iso"), indirect=["image_type"])
def test_iso_installs(image_type):
    installer_iso_path = image_type.img_path
    test_disk_path = installer_iso_path.with_name("test-disk.img")
    with open(test_disk_path, "w", encoding="utf8") as fp:
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
        assert_kernel_args(vm, image_type)


def osinfo_for(it: ImageBuildResult, arch: str) -> str:
    base = "Media is an installer for OS"
    if it.container_ref.endswith("/centos-bootc/centos-bootc:stream9"):
        return f"{base} 'CentOS Stream 9 ({arch})'\n"
    if it.container_ref.endswith("/centos-bootc/centos-bootc:stream10"):
        return f"Media is an installer for OS 'CentOS Stream 10 ({arch})'\n"
    if "/fedora/fedora-bootc:" in it.container_ref:
        ver = it.container_ref.rsplit(":", maxsplit=1)[1]
        return f"{base} 'Fedora Server {ver} ({arch})'\n"
    raise ValueError(f"unknown osinfo string for '{it.container_ref}'")


@pytest.mark.skipif(platform.system() != "Linux", reason="osinfo detect test only runs on linux right now")
@pytest.mark.parametrize("image_type", gen_testcases("anaconda-iso"), indirect=["image_type"])
def test_iso_os_detection(image_type):
    installer_iso_path = image_type.img_path
    arch = image_type.img_arch
    if not arch:
        arch = platform.machine()
    result = subprocess.run([
        "osinfo-detect",
        installer_iso_path,
    ], capture_output=True, text=True, check=True)
    osinfo_output = result.stdout
    expected_output = f"Media is bootable.\n{osinfo_for(image_type, arch)}"
    assert osinfo_output == expected_output


@pytest.mark.skipif(platform.system() != "Linux", reason="osinfo detect test only runs on linux right now")
@pytest.mark.skipif(not testutil.has_executable("unsquashfs"), reason="need unsquashfs")
@pytest.mark.parametrize("image_type", gen_testcases("anaconda-iso"), indirect=["image_type"])
def test_iso_install_img_is_squashfs(tmp_path, image_type):
    installer_iso_path = image_type.img_path
    with ExitStack() as cm:
        mount_point = tmp_path / "cdrom"
        mount_point.mkdir()
        subprocess.check_call(["mount", installer_iso_path, os.fspath(mount_point)])
        cm.callback(subprocess.check_call, ["umount", os.fspath(mount_point)])
        # ensure install.img is the "flat" squashfs, before PR#777 the content
        # was an intermediate ext4 image "squashfs-root/LiveOS/rootfs.img"
        output = subprocess.check_output(["unsquashfs", "-ls", mount_point / "images/install.img"], text=True)
        assert "usr/bin/bootc" in output


@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("container_ref", [
    "quay.io/centos-bootc/centos-bootc:stream10",
    "quay.io/fedora/fedora-bootc:42",
    "quay.io/centos-bootc/centos-bootc:stream9",
])
# pylint: disable=too-many-locals
def test_bootc_installer_iso_installs(tmp_path, build_container, container_ref):
    # XXX: duplicated from test_build_disk.py
    username = "test"
    password = "".join(
        random.choices(string.ascii_uppercase + string.digits, k=18))
    ssh_keyfile_private_path = tmp_path / "ssh-keyfile"
    ssh_keyfile_public_path = ssh_keyfile_private_path.with_suffix(".pub")
    if not ssh_keyfile_private_path.exists():
        subprocess.run([
            "ssh-keygen",
            "-N", "",
            # be very conservative with keys for paramiko
            "-b", "2048",
            "-t", "rsa",
            "-f", os.fspath(ssh_keyfile_private_path),
        ], check=True)
    ssh_pubkey = ssh_keyfile_public_path.read_text(encoding="utf8").strip()
    cfg = {
        "customizations": {
            "user": [
                {
                    "name": "root",
                    "key": ssh_pubkey,
                    # note that we have no "home" here for ISOs
                }, {
                    "name": username,
                    "password": password,
                    "groups": ["wheel"],
                },
            ],
            "kernel": {
                # XXX: we need https://github.com/osbuild/images/pull/1786 or no kargs are added to anaconda
                # XXX2: drop a bunch of the debug flags
                #
                # Use console=ttyS0 so that we see output in our debug
                # logs. by default anaconda prints to the last console=
                # from the kernel commandline
                "append": "systemd.debug-shell=1 rd.systemd.debug-shell=1 inst.debug console=ttyS0",
            },
        },
    }
    config_json_path = tmp_path / "config.json"
    config_json_path.write_text(json.dumps(cfg), encoding="utf-8")
    # create anaconda iso from base
    cntf_path = tmp_path / "Containerfile"
    cntf_path.write_text(textwrap.dedent(f"""\n
    FROM {container_ref}
    RUN dnf install -y \
         anaconda-core \
         anaconda-dracut \
         anaconda-install-img-deps \
         biosdevname \
         grub2-efi-x64-cdboot \
         net-tools \
         prefixdevname \
         python3-mako \
         lorax-templates-* \
         squashfs-tools \
         && dnf clean all
    # shim-x64 is marked installed but the files are not in the expected
    # place for https://github.com/osbuild/osbuild/blob/v160/stages/org.osbuild.grub2.iso#L91, see
    # workaround via reinstall, we could add a config to the grub2.iso
    # stage to allow a different prefix that then would be used by
    # anaconda.
    # If https://github.com/osbuild/osbuild/pull/2204 would get merged we
    # can update images/ to set the correct efi_src_dirs and this can
    # be removed (but its rather ugly).
    # See also https://bugzilla.redhat.com/show_bug.cgi?id=1750708
    RUN dnf reinstall -y shim-x64
    # lorax wants to create a symlink in /mnt which points to /var/mnt
    # on bootc but /var/mnt does not exist on some images.
    #
    # If https://gitlab.com/fedora/bootc/base-images/-/merge_requests/294
    # gets merged this will be no longer needed
    RUN mkdir /var/mnt
    """), encoding="utf8")
    output_path = tmp_path / "output"
    output_path.mkdir()
    with make_container(tmp_path) as container_tag:
        cmd = [
            *testutil.podman_run_common,
            "-v", f"{config_json_path}:/config.json:ro",
            "-v", f"{output_path}:/output",
            "-v", "/var/tmp/osbuild-test-store:/store",  # share the cache between builds
            "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
            build_container,
            "--type", "bootc-installer",
            "--rootfs", "ext4",
            "--installer-payload-ref", container_ref,
            f"localhost/{container_tag}",
        ]
        subprocess.check_call(cmd)
        installer_iso_path = output_path / "bootiso" / "install.iso"
        test_disk_path = installer_iso_path.with_name("test-disk.img")
        with open(test_disk_path, "w", encoding="utf8") as fp:
            fp.truncate(10_1000_1000_1000)
        # install to test disk
        with QEMU(test_disk_path, cdrom=installer_iso_path) as vm:
            vm.start(wait_event="qmp:RESET", snapshot=False, use_ovmf=True)
            vm.force_stop()
        # boot test disk and do extremly simple check
        with QEMU(test_disk_path) as vm:
            vm.start(use_ovmf=True)
            exit_status, _ = vm.run("true", user=username, password=password)
            assert exit_status == 0
            exit_status, output = vm.run("bootc status", user="root", keyfile=ssh_keyfile_private_path)
            assert exit_status == 0
            assert f"Booted image: {container_ref}" in output
