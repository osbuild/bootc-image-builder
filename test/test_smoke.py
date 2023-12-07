import json
import os
import pathlib
import re
import subprocess

import pytest

# local test utils
import testutil
from vm import VM


@pytest.fixture(name="output_path")
def output_path_fixture(tmp_path):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)
    return output_path


@pytest.fixture(name="config_json")
def config_json_fixture(output_path):
    CFG = {
        "blueprint": {
            "customizations": {
                "user": [
                    {
                        "name": "test",
                        "password": "password",
                        "groups": ["wheel"],
                    },
                ],
            },
        },
    }
    config_json_path = output_path / "config.json"
    config_json_path.write_text(json.dumps(CFG), encoding="utf-8")
    return config_json_path


def log_has_osbuild_selinux_denials(log):
    OSBUID_SELINUX_DENIALS_RE = re.compile(r"(?ms)avc:\ +denied.*osbuild")
    return re.search(OSBUID_SELINUX_DENIALS_RE, log)


def test_osbuild_selinux_denails_re_works():
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


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
@pytest.mark.skipif(not testutil.has_executable("podman"), reason="need podman")
def test_smoke(output_path, config_json):
    # build local container
    subprocess.check_call([
        "podman", "build",
        "-f", "Containerfile",
        "-t", "bootc-image-builder-test",
    ])
    cursor = testutil.journal_cursor()
    # and run container to deploy an image into output/disk.qcow2
    subprocess.check_call([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{output_path}:/output",
        "bootc-image-builder-test",
        "quay.io/centos-bootc/fedora-bootc:eln",
        "--config", "/output/config.json",
    ])
    generated_img = pathlib.Path(output_path) / "qcow2/disk.qcow2"
    assert generated_img.exists(), f"output file missing, dir content: {os.listdir(os.fspath(output_path))}"

    # check that there are no selinux denials
    journal_output = testutil.journal_after_cursor(cursor)
    assert journal_output != ""
    if testutil.has_executable("selinuxenabled") and subprocess.run("selinuxenabled").returncode == 0:
        assert not log_has_osbuild_selinux_denials(journal_output), f"denials in log {journal_output}"
    else:
        print("WARNING: selinux not enabled, cannot check for denials")

    with VM(generated_img) as test_vm:
        exit_status, _ = test_vm.run("true", user="test", password="password")
        assert exit_status == 0
        exit_status, output = test_vm.run("echo hello", user="test", password="password")
        assert exit_status == 0
        assert "hello" in output
