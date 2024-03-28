import subprocess
import sys

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


def test_bib_config_errors_for_default(tmp_path, build_fake_container):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    ret = subprocess.run([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{output_path}:/output",
        build_fake_container,
        "--iso-config", "/some/random/config",
        "quay.io/centos-bootc/centos-bootc:stream9",
    ], check=False, encoding="utf8", stdout=sys.stdout, stderr=subprocess.PIPE)
    assert ret.returncode != 0
    assert "the --iso-config switch is only supported for ISO images" in ret.stderr


def test_bib_iso_config_is_parsed(tmp_path, build_fake_container):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    # check that config.json is tried to be loaded
    (tmp_path / "config.json").write_text("invalid-json", encoding="utf8")
    ret = subprocess.run([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{output_path}:/output",
        "-v", f"{tmp_path}/config.json:/config.json",
        build_fake_container,
        "--iso-config", "/config.json",
        "--type", "anaconda-iso",
        "quay.io/centos-bootc/centos-bootc:stream9",
    ], check=False, encoding="utf8", stdout=sys.stdout, stderr=subprocess.PIPE)
    assert ret.returncode != 0
    assert "cannot load config: invalid character" in ret.stderr
