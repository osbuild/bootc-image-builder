import os
import subprocess

import pytest


@pytest.fixture(name="build_container", scope="session")
def build_container_fixture():
    """Build a container from the Containerfile and returns the name"""
    if tag_from_env := os.getenv("BIB_TEST_BUILD_CONTAINER_TAG"):
        return tag_from_env

    container_tag = "bootc-image-builder-test"
    subprocess.check_call([
        "podman", "build",
        "-f", "Containerfile",
        "-t", container_tag,
    ])
    return container_tag


def container_to_build_ref():
    # TODO: make this another indirect fixture input, e.g. by making
    # making "image_type" an "image" tuple (type, container_ref_to_test)
    return os.getenv(
        "BIB_TEST_BOOTC_CONTAINER_TAG",
        # using this tag instead of ":eln" until
        #  https://github.com/CentOS/centos-bootc/issues/184 and
        #  https://github.com/osbuild/bootc-image-builder/issues/149
        # are fixed
        "quay.io/centos-bootc/fedora-bootc:ed19452a30c50900be0b78db5f68d9826cc14a2e402f752535716cffd92b4445",
    )
