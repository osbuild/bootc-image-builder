import subprocess

import testutil
# pylint: disable=unused-import,duplicate-code
from test_opts import container_storage_fixture
from containerbuild import build_container_fixture, build_fake_container_fixture


def test_progress_debug(tmp_path, build_fake_container):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    cmdline = [
        *testutil.podman_run_common,
        build_fake_container,
        "build",
        "--progress=debug",
        "quay.io/centos-bootc/centos-bootc:stream9",
    ]
    res = subprocess.run(cmdline, capture_output=True, check=True, text=True)
    assert res.stderr.count("Start progressbar") == 1
    assert res.stderr.count("Manifest generation step") == 1
    assert res.stderr.count("Image building step") == 1
    assert res.stderr.count("Build complete") == 1
    assert res.stderr.count("Stop progressbar") == 1
    assert res.stdout.strip() == ""


def test_progress_term_works_without_tty(tmp_path, build_fake_container):
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    testutil.pull_container(container_ref)

    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    cmdline = [
        *testutil.podman_run_common,
        # note that "-t" is missing here
        build_fake_container,
        "build",
        # explicitly selecting term progress works even when there is no tty
        # (i.e. we just need ansi terminal support)
        "--progress=term",
        container_ref,
    ]
    res = subprocess.run(cmdline, capture_output=True, text=True, check=False)
    assert res.returncode == 0
    assert "[|] Manifest generation step" in res.stderr


def test_progress_term_autoselect(tmp_path, build_fake_container):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    cmdline = [
        *testutil.podman_run_common,
        # we have a terminal
        "-t",
        build_fake_container,
        "build",
        # note that we do not select a --progress here so auto-select is used
        "quay.io/centos-bootc/centos-bootc:stream9",
    ]
    res = subprocess.run(cmdline, capture_output=True, text=True, check=False)
    assert res.returncode == 0
    # its curious that we get the output on stdout here, podman weirdness?
    assert "[|] Manifest generation step" in res.stdout
