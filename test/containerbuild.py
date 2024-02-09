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


@pytest.fixture(name="build_ansible_container", scope="session")
def build_ansible_container_fixture():
    """Build the container to run ansible in"""
    if tag_from_env := os.getenv("BIB_TEST_ANSIBLE_CONTAINER_TAG"):
        return tag_from_env

    container_tag = "bootc-image-builder-ansible-runner"
    subprocess.check_call([
        "podman", "build",
        "-t", container_tag,
        "test/ansible-container"
    ])
    return container_tag
