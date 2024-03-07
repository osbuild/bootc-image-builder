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
