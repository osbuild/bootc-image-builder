import subprocess

# pylint: disable=unused-import
from test_opts import container_storage_fixture
from containerbuild import build_container_fixture, build_fake_container_fixture


def bib_cmd(container_storage, output_path, build_fake_container):
    return [
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{container_storage}:/var/lib/containers/storage",
        "-v", f"{output_path}:/output",
        build_fake_container,
        "build",
        "quay.io/centos-bootc/centos-bootc:stream9",
    ]


def test_progress_debug(tmp_path, container_storage, build_fake_container):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    cmdline = bib_cmd(container_storage, output_path, build_fake_container)
    cmdline.append("--progress=debug")
    res = subprocess.run(cmdline, capture_output=True, check=True, text=True)
    assert res.stderr.count("Start progressbar") == 1
    assert res.stderr.count("Manifest generation step") == 1
    assert res.stderr.count("Image building step") == 1
    assert res.stderr.count("Build complete") == 1
    assert res.stderr.count("Stop progressbar") == 1
    assert res.stdout.strip() == ""
