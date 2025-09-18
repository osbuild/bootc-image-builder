import os
import random
import json
import platform
import string
import subprocess
from contextlib import ExitStack
import textwrap

import pytest
# local test utils
import testutil
from containerbuild import build_container_fixture    # pylint: disable=unused-import
from containerbuild import make_container
from testcases import gen_testcases
from vm import QEMU

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
def test_container_iso_installs(tmp_path, build_container):
    #container_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    container_ref = "quay.io/fedora/fedora-bootc:42"

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
                # XXX: console= needs to be default (why is it not?)
                # XXX2: add inst.text automatically (or include all deps for a graphical install)
                # XXX3: we need https://github.com/osbuild/images/pull/1786 or no kargs are added to anaconda
                "append": f"systemd.debug-shell=1 rd.systemd.debug-shell=1 inst.debug",
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
         anaconda \
         anaconda-install-env-deps \
         anaconda-dracut \
         dracut-config-generic \
         dracut-network \
         net-tools \
         squashfs-tools \
         grub2-efi-x64-cdboot \
         python3-mako \
         lorax-templates-* \
         biosdevname \
         prefixdevname \
         && dnf clean all
    # shim-x64 is marked installed but the files are not in the expected
    # place for https://github.com/osbuild/osbuild/blob/v160/stages/org.osbuild.grub2.iso#L91
    # workaround via reinstall, we could add a config to the grub2.iso
    # stage to allow a different prefix that then would be used by anaconda
    RUN dnf reinstall -y shim-x64
    # remove stange lorax template line (XXX: figure out what this is about or if there is a different workaround, it seems not needed)
    RUN sed -i 's,symlink ../run/install mnt/install,,' /usr/share/lorax/templates.d/*/runtime-postinstall.tmpl || true
    # the fedora version of anaconda needs a real /root dir (instead of the bootc default /root -> /var/roothome)
    # centos9 is fine, it seems https://github.com/weldr/lorax/blob/master/share/templates.d/99-generic/runtime-postinstall.tmpl#L73 is the reason but its unclear why the symlink is notworking
    RUN rm -f /root
    RUN mkdir -m0700 /root
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
            "--type", "iso",
            "--rootfs", "ext4",
            "--installer-payload", container_ref,
            f"localhost/{container_tag}",
        ]
        print(" ".join(cmd))
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
            #assert_kernel_args(vm, image_type)
            exit_status, output = vm.run("bootc status", user="root", keyfile=ssh_keyfile_private_path)
            assert exit_status == 0
            assert f"Booted image: {container_ref}" in output
