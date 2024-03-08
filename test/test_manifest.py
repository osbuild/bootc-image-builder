import json
import subprocess

import pytest

import testutil

if not testutil.has_executable("podman"):
    pytest.skip("no podman, skipping integration tests that required podman", allow_module_level=True)

from containerbuild import build_container_fixture  # noqa: F401
from testcases import gen_testcases


@pytest.mark.parametrize("testcase_ref", gen_testcases("manifest"))
def test_manifest_smoke(build_container, testcase_ref):
    # testcases_ref has the form "container_url,img_type1+img_type2,arch"
    container_ref = testcase_ref.split(",")[0]

    output = subprocess.check_output([
        "podman", "run", "--rm",
        f'--entrypoint=["/usr/bin/bootc-image-builder", "manifest", "{container_ref}"]',
        build_container,
    ])
    manifest = json.loads(output)
    # just some basic validation
    assert manifest["version"] == "2"
    assert manifest["pipelines"][0]["name"] == "build"
