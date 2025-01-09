import os
import platform
import subprocess

import pytest
# pylint: disable=unused-import
from containerbuild import build_container_fixture, build_fake_container_fixture


@pytest.fixture(name="container_storage", scope="session")
def container_storage_fixture(tmp_path_factory):
    # share systemwide storage when running as root, this makes the GH
    # tests faster because they already have the test images used here
    if os.getuid() == 0:
        return "/var/lib/containers/storage"
    return tmp_path_factory.mktemp("storage")


@pytest.mark.parametrize("chown_opt,expected_uid_gid", [
    ([], (0, 0)),
    (["--chown", "1000:1000"], (1000, 1000)),
    (["--chown", "1000"], (1000, 0)),
])
def test_bib_chown_opts(tmp_path, container_storage, build_fake_container, chown_opt, expected_uid_gid):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    subprocess.check_call([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{container_storage}:/var/lib/containers/storage",
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


@pytest.mark.parametrize("tls_opt,expected_cmdline", [
    ([], "--tls-verify=true"),
    (["--tls-verify"], "--tls-verify=true"),
    (["--tls-verify=true"], "--tls-verify=true"),
    (["--tls-verify=false"], "--tls-verify=false"),
    (["--tls-verify=0"], "--tls-verify=false"),
])
def test_bib_tls_opts(tmp_path, container_storage, build_fake_container, tls_opt, expected_cmdline):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    subprocess.check_call([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{container_storage}:/var/lib/containers/storage",
        "-v", f"{output_path}:/output",
        build_fake_container,
        "quay.io/centos-bootc/centos-bootc:stream9"
    ] + tls_opt)
    podman_log = output_path / "podman.log"
    assert expected_cmdline in podman_log.read_text()


@pytest.mark.parametrize("with_debug", [False, True])
def test_bib_log_level_smoke(tmp_path, container_storage, build_fake_container, with_debug):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    log_debug = ["--log-level", "debug"] if with_debug else []
    res = subprocess.run([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{container_storage}:/var/lib/containers/storage",
        "-v", f"{output_path}:/output",
        build_fake_container,
        *log_debug,
        "quay.io/centos-bootc/centos-bootc:stream9"
    ], check=True, capture_output=True, text=True)
    assert ('level=debug' in res.stderr) == with_debug


def test_bib_help_hides_config(tmp_path, container_storage, build_fake_container):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    res = subprocess.run([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{container_storage}:/var/lib/containers/storage",
        "-v", f"{output_path}:/output",
        build_fake_container,
        "manifest", "--help",
    ], check=True, capture_output=True, text=True)
    # --config should not be user visible
    assert '--config' not in res.stdout
    # but other options should be
    assert '--log-level' in res.stdout


def test_bib_errors_only_once(tmp_path, container_storage, build_fake_container):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    res = subprocess.run([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{container_storage}:/var/lib/containers/storage",
        "-v", f"{output_path}:/output",
        build_fake_container,
        "localhost/no-such-image",
    ], check=False, capture_output=True, text=True)
    needle = "cannot build manifest: failed to pull container image:"
    assert res.stderr.count(needle) == 1


@pytest.mark.parametrize("version_argument", ["version", "--version", "-v"])
def test_bib_version(tmp_path, container_storage, build_fake_container, version_argument):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    res = subprocess.run([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{container_storage}:/var/lib/containers/storage",
        "-v", f"{output_path}:/output",
        build_fake_container,
        version_argument,
    ], check=True, capture_output=True, text=True)

    expected_rev = "unknown"
    git_res = subprocess.run(
        ["git", "describe", "--always"],
        capture_output=True, text=True, check=False)
    if git_res.returncode == 0:
        expected_rev = git_res.stdout.strip()
    assert f"build_revision: {expected_rev}" in res.stdout
    assert "build_time: " in res.stdout
    assert "build_tainted: " in res.stdout
    # we have a final newline
    assert res.stdout[-1] == "\n"


def test_bib_no_outside_container_warning_in_container(tmp_path, container_storage, build_fake_container):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    res = subprocess.run([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{container_storage}:/var/lib/containers/storage",
        "-v", f"{output_path}:/output",
        build_fake_container,
        "quay.io/centos-bootc/centos-bootc:stream9"
    ], check=True, capture_output=True, text=True)
    assert "running outside a container" not in res.stderr
