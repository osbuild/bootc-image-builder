import dataclasses
import inspect
import os
import platform

# disk image types can be build from a single manifest
DISK_IMAGE_TYPES = ["qcow2", "raw", "vmdk", "vhd", "gce"]

# supported images that can be booted in a cloud
CLOUD_BOOT_IMAGE_TYPES = ["ami"]


@dataclasses.dataclass
class TestCase:
    # container_ref to the bootc image, e.g. quay.io/fedora/fedora-bootc:40
    container_ref: str = ""
    # image is the image type, e.g. "ami"
    image: str = ""
    # target_arch is the target archicture, empty means current arch
    target_arch: str = ""
    # local means that the container should be pulled locally ("--local" flag)
    local: bool = False
    # rootfs to use (e.g. ext4), some containers like fedora do not
    # have a default rootfs. If unset the container default is used.
    rootfs: str = ""

    def bib_rootfs_args(self):
        if self.rootfs:
            return ["--rootfs", self.rootfs]
        return []

    def __str__(self):
        return ",".join([
            attr
            for name, attr in inspect.getmembers(self)
            if not name.startswith("_") and not callable(attr) and attr
        ])


@dataclasses.dataclass
class TestCaseFedora(TestCase):
    container_ref: str = "quay.io/fedora/fedora-bootc:40"
    rootfs: str = "btrfs"


@dataclasses.dataclass
class TestCaseFedora42(TestCase):
    container_ref: str = "quay.io/fedora/fedora-bootc:42"
    rootfs: str = "btrfs"


@dataclasses.dataclass
class TestCaseCentos(TestCase):
    container_ref: str = os.getenv(
        "BIB_TEST_BOOTC_CONTAINER_TAG",
        "quay.io/centos-bootc/centos-bootc:stream9")


def test_testcase_nameing():
    """
    Ensure the testcase naming does not change without us knowing as those
    are visible when running "pytest --collect-only"
    """
    tc = TestCaseFedora()
    expected = "quay.io/fedora/fedora-bootc:40,btrfs"
    assert f"{tc}" == expected, f"{tc} != {expected}"


def gen_testcases(what):  # pylint: disable=too-many-return-statements
    if what == "manifest":
        return [TestCaseCentos(), TestCaseFedora()]
    if what == "default-rootfs":
        # Fedora doesn't have a default rootfs
        return [TestCaseCentos()]
    if what == "ami-boot":
        return [TestCaseCentos(image="ami"), TestCaseFedora(image="ami")]
    if what == "anaconda-iso":
        return [TestCaseCentos(image="anaconda-iso"), TestCaseFedora(image="anaconda-iso")]
    if what == "qemu-boot":
        test_cases = [
            klass(image=img)
            for klass in (TestCaseCentos, TestCaseFedora)
            for img in ("raw", "qcow2")
        ]
        # do a cross arch test too
        if platform.machine() == "x86_64":
            # TODO: re-enable once
            # https://github.com/osbuild/bootc-image-builder/issues/619
            # is resolved
            # test_cases.append(
            #    TestCaseCentos(image="raw", target_arch="arm64"))
            pass
        elif platform.machine() == "arm64":
            # TODO: add arm64->x86_64 cross build test too
            pass
        return test_cases
    if what == "all":
        return [
            klass(image=img)
            for klass in (TestCaseCentos, TestCaseFedora)
            for img in CLOUD_BOOT_IMAGE_TYPES + DISK_IMAGE_TYPES + ["anaconda-iso"]
        ]
    if what == "multidisk":
        # single test that specifies all image types
        image = "+".join(DISK_IMAGE_TYPES)
        return [
            TestCaseCentos(image=image),
            TestCaseFedora(image=image),
        ]
    # Smoke test that all supported --target-arch architecture can
    # create a manifest
    if what == "target-arch-smoke":
        return [
            TestCaseCentos(target_arch="arm64"),
            # TODO: merge with TestCaseFedora once the arches are build there
            TestCaseFedora42(target_arch="ppc64le"),
            TestCaseFedora42(target_arch="s390x"),
        ]
    raise ValueError(f"unknown test-case type {what}")
