import json
import os
import pathlib
import subprocess

import pytest

# local test utils
import testutil


@pytest.fixture(name="output_path")
def output_path_fixture(tmp_path):
    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)
    return output_path


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


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
@pytest.mark.skipif(not testutil.has_executable("podman"), reason="need podman")
def test_smoke(output_path, config_json):
    # build local container
    subprocess.check_call([
        "podman", "build",
        "-f", "Containerfile",
        "-t", "bootc-image-builder-test",
    ])
    cursor = testutil.journal_cursor()
    # and run container to deploy an image into output/disk.qcow2
    subprocess.check_call([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "-v", f"{output_path}:/output",
        "bootc-image-builder-test",
        "quay.io/centos-bootc/centos-bootc:stream9",
        "--config", "/output/config.json",
    ])
    # check that there are no denials
    # TODO: actually check this once https://github.com/osbuild/images/pull/287
    #       is merged
    journal_output = testutil.journal_after_cursor(cursor)
    assert journal_output != ""
    generated_img = pathlib.Path(output_path) / "qcow2/disk.qcow2"
    assert generated_img.exists(), f"output file missing, dir content: {os.listdir(os.fspath(output_path))}"
    # TODO: boot and do basic checks, see
    # https://github.com/osbuild/bootc-image-builder/compare/main...mvo5:integration-test?expand=1
