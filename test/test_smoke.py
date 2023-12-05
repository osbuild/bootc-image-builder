import json
import os
import pathlib
import subprocess
import tempfile

import pytest

# local test utils
import testutil


@pytest.fixture(name="output_path")
def output_path_fixture():
    # quirky setup to workaround macos weirdness:
    # 1. we need a dir shared between "podman machine" and host to put the
    #    config.json and to get the resulting disk.qcow2 image
    # 2. *but* just sharing /var/tmp will result in errors because inside
    #    podman things like lchown in /var/tmp that happen during the container
    #    build which will affect the host and cause "operation not permitted"
    #    errors
    base_tmp = pathlib.Path("/var/tmp/bootc-tests")
    base_tmp.mkdir(exist_ok=True, mode=0o700)
    with tempfile.TemporaryDirectory(dir=base_tmp) as tmp_dir:
        # HACKKKKKK: macos with podman keeps giving "permission denied" errors
        # without this - however given that the parent is 0700 we should be ok
        os.chmod(tmp_dir, 0o777)
        tmp_path = pathlib.Path(tmp_dir)
        output_path = tmp_path / "output"
        output_path.mkdir(exist_ok=True)
        yield tmp_path


@pytest.fixture(name="config_json")
def config_json_fixture(output_path):
    CFG = {
        "blueprint": {
            "customizations": {
                "user": [
                    {
                        "name": "test",
                        "password": "password",
                        "groups": ["wheel"],
                    },
                ],
            },
        },
    }
    config_json_path = output_path / "config.json"
    config_json_path.write_text(json.dumps(CFG), encoding="utf-8")
    return config_json_path


@pytest.fixture(name="journal_cursor")
def journal_cursor_fixture():
    if not testutil.has_executable("journalctl"):
        return None
    return testutil.journal_cursor()


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
@pytest.mark.skipif(not testutil.has_executable("podman"), reason="need podman")
def test_smoke(output_path, journal_cursor, config_json):
    # build local container
    subprocess.check_call([
        "podman", "build",
        "-f", "Containerfile",
        "-t", "osbuild-deploy-container-test",
    ])
    # and run container to deploy an image into output/disk.qcow2
    subprocess.check_call([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{output_path}:/output",
        "osbuild-deploy-container-test",
        "quay.io/centos-bootc/centos-bootc:stream9",
        "--config", "/output/config.json",
    ])
    # check that there are no denials
    # TODO: actually check this once https://github.com/osbuild/images/pull/287
    #       is merged
    if journal_cursor:
        journal_output = testutil.journal_after_cursor(journal_cursor)
        assert journal_output != ""
    generated_img = pathlib.Path(output_path) / "qcow2/disk.qcow2"
    assert generated_img.exists(), f"output file missing, dir content: {os.listdir(os.fspath(output_path))}"
    # TODO: boot and do basic checks, see
    # https://github.com/osbuild/osbuild-deploy-container/compare/main...mvo5:integration-test?expand=1
