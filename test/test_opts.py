import platform
import subprocess

import pytest

from containerbuild import build_container_fixture, build_fake_container_fixture  # noqa: F401


@pytest.mark.parametrize("chown_opt,expected_uid_gid", [
    ([], (0, 0)),
    (["--chown", "1000:1000"], (1000, 1000)),
    (["--chown", "1000"], (1000, 0)),
])
def test_bib_chown_opts(tmp_path, build_fake_container, chown_opt, expected_uid_gid):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    subprocess.check_call([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{output_path}:/output",
        build_fake_container,
        "quay.io/centos-bootc/centos-bootc:stream9",
    ] + chown_opt)
    expected_output_disk = output_path / "qcow2/disk.qcow2"
    for p in output_path, expected_output_disk:
        assert p.exists()
        assert p.stat().st_uid == expected_uid_gid[0]
        assert p.stat().st_gid == expected_uid_gid[1]


@pytest.mark.parametrize("target_arch_opt, expected_err", [
    ([], ""),
    (["--target-arch=amd64"], ""),
    (["--target-arch=x86_64"], ""),
    (["--target-arch=arm64"], "cannot build iso for different target arches yet"),
])
@pytest.mark.skipif(platform.uname().machine != "x86_64", reason="cross build test only runs on x86")
def test_opts_arch_is_same_arch_is_fine(tmp_path, build_fake_container, target_arch_opt, expected_err):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    res = subprocess.run([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
        "-v", f"{output_path}:/output",
        build_fake_container,
        "--type=iso",
        "quay.io/centos-bootc/centos-bootc:stream9",
    ] + target_arch_opt, check=False, capture_output=True, text=True)
    if expected_err == "":
        assert res.returncode == 0
    else:
        assert res.returncode != 0
        assert expected_err in res.stderr
