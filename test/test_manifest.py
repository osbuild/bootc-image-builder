import json
import subprocess

import pytest

import testutil


if not testutil.has_executable("podman"):
    pytest.skip("no podman, skipping integration tests that required podman", allow_module_level=True)

from containerbuild import build_container_fixture, container_to_build_ref  # noqa: F401


def test_manifest_smoke(build_container):
    output = subprocess.check_output([
        "podman", "run", "--rm",
        f'--entrypoint=["/usr/bin/bootc-image-builder", "manifest", "{container_to_build_ref()}"]',
        build_container,
    ])
    manifest = json.loads(output)
    # just some basic validation
    assert manifest["version"] == "2"
    assert manifest["pipelines"][0]["name"] == "build"
