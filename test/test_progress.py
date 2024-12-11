import subprocess

# pylint: disable=unused-import,duplicate-code
from test_opts import container_storage_fixture
from containerbuild import build_container_fixture, build_fake_container_fixture


def test_progress_debug(tmp_path, container_storage, build_fake_container):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    cmdline = [
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{container_storage}:/var/lib/containers/storage",
        "-v", f"{output_path}:/output",
        build_fake_container,
        "build",
        "quay.io/centos-bootc/centos-bootc:stream9",
    ]
    cmdline.append("--progress=debug")
    res = subprocess.run(cmdline, capture_output=True, check=True, text=True)
    assert res.stderr.count("Start progressbar") == 1
    assert res.stderr.count("Manifest generation step") == 1
    assert res.stderr.count("Image building step") == 1
    assert res.stderr.count("Build complete") == 1
    assert res.stderr.count("Stop progressbar") == 1
    assert res.stdout.strip() == ""


def test_progress_term(tmp_path, container_storage, build_fake_container):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    cmdline = [
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{container_storage}:/var/lib/containers/storage",
        "-v", f"{output_path}:/output",
        build_fake_container,
        "build",
        # explicitly select term progress
        "--progress=term",
        "quay.io/centos-bootc/centos-bootc:stream9",
    ]
    res = subprocess.run(cmdline, capture_output=True, text=True, check=False)
    assert res.returncode == 0
    assert "[|] Manifest generation step" in res.stderr
