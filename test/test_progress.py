import os
import subprocess

import pytest

# pylint: disable=unused-import
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
    assert res.stderr.count("Image generation step") == 1
    assert res.stderr.count("Build complete") == 1
    assert res.stderr.count("Stop progressbar") == 1
    assert res.stdout.strip() == ""


def test_progress_term(tmp_path, container_storage, build_fake_container):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)

    # XXX: we cannot use RawTerminal mode (which Pb requires) with podman,
    # except when using "--log-driver=passthrough-tty"
    cmdline = [
        "podman", "run", "--rm",
        "--privileged",
        # Note that this is needed to get the pb.ProgressBar support
        "--log-driver=passthrough-tty",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{container_storage}:/var/lib/containers/storage",
        "-v", f"{output_path}:/output",
        build_fake_container,
        "build",
        # this should not be needed but we add it to ensure it breaks early
        # if it cannot access the tty
        "--progress=term",
        "quay.io/centos-bootc/centos-bootc:stream9",
    ]
    # simulate running in a pty (subprocess.run() won't cut it)
    cmdline = ["systemd-run", "--pty"] + cmdline
    res = subprocess.run(cmdline, capture_output=True, text=True, check=False)
    assert res.returncode == 0
    # systemd-run gives us stderr on stdout (i.e. it just combines the
    # two streams)
    # smoke test that we see a progress
    assert "[|] Manifest generation step" in res.stdout
