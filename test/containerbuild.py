import os
import platform
import random
import string
import subprocess
import textwrap
from contextlib import contextmanager

import pytest


@contextmanager
def make_container(container_path, arch=None):
    # BIB only supports container tags, not hashes
    container_tag = "bib-test-" + "".join(random.choices(string.digits, k=12))

    if not arch:
        # Always provide an architecture here because without that the default
        # behavior is to pull whatever arch was pulled for this image ref
        # last but we want "native" if nothing else is specified.
        #
        # Note: podman seems to translate kernel arch to go arches
        # automatically it seems.
        arch = platform.uname().machine

    subprocess.check_call([
        "podman", "build",
        "-t", container_tag,
        "--arch", arch,
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

    # see https://github.com/osbuild/osbuild/blob/main/osbuild/testutil/__init__.py#L91
    tracing_podman_path = tmp_path / "tracing-podman"
    tracing_podman_path.write_text(textwrap.dedent("""\
    #!/bin/sh -e

    TRACE_PATH=/output/"$(basename $0)".log
    for arg in "$@"; do
        echo "$arg" >> "$TRACE_PATH"
    done
    # extra separator to differenciate between calls
    echo >> "$TRACE_PATH"
    exec "$0".real "$@"
    """), encoding="utf8")

    fake_osbuild_path = tmp_path / "fake-osbuild"
    fake_osbuild_path.write_text(textwrap.dedent("""\
    #!/bin/bash -e

    # injest generated manifest from the images library, if we do not
    # do this images may fail with "broken" pipe errors
    cat - >/dev/null

    mkdir -p /output/qcow2
    echo "fake-disk.qcow2" > /output/qcow2/disk.qcow2

    """), encoding="utf8")

    cntf_path = tmp_path / "Containerfile"

    cntf_path.write_text(textwrap.dedent(f"""\n
    FROM {build_container}
    COPY fake-osbuild /usr/bin/osbuild
    RUN chmod 755 /usr/bin/osbuild
    COPY --from={build_container} /usr/bin/podman /usr/bin/podman.real
    COPY tracing-podman /usr/bin/podman
    RUN chmod 755 /usr/bin/podman
    """), encoding="utf8")

    container_tag = "bootc-image-builder-test-faked-osbuild"
    subprocess.check_call([
        "podman", "build",
        "-t", container_tag,
        tmp_path,
    ])
    return container_tag
