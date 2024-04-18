import os
import random
import string
import subprocess
import textwrap
from contextlib import contextmanager

import pytest


@contextmanager
def make_container(container_path):
    # BIB only supports container tags, not hashes
    container_tag = "bib-test-" + "".join(random.choices(string.digits, k=12))
    subprocess.check_call([
        "podman", "build",
        "-t", container_tag,
        container_path], encoding="utf8")
    yield container_tag
    subprocess.check_call(["podman", "rmi", container_tag])


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


@pytest.fixture(name="build_fake_container", scope="session")
def build_fake_container_fixture(tmpdir_factory, build_container):
    """Build a container with a fake osbuild and returns the name"""
    tmp_path = tmpdir_factory.mktemp("build-fake-container")

    fake_osbuild_path = tmp_path / "fake-osbuild"
    fake_osbuild_path.write_text(textwrap.dedent("""\
    #!/bin/sh -e

    # injest generated manifest from the images library, if we do not
    # do this images may fail with "broken" pipe errors
    cat -

    mkdir -p /output/qcow2
    echo "fake-disk.qcow2" > /output/qcow2/disk.qcow2

    echo "Done"
    """), encoding="utf8")

    cntf_path = tmp_path / "Containerfile"

    cntf_path.write_text(textwrap.dedent(f"""\n
    FROM {build_container}
    COPY fake-osbuild /usr/bin/osbuild
    RUN chmod 755 /usr/bin/osbuild
    """), encoding="utf8")

    container_tag = "bootc-image-builder-test-faked-osbuild"
    subprocess.check_call([
        "podman", "build",
        "-t", container_tag,
        tmp_path,
    ])
    return container_tag
