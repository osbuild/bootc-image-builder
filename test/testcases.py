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
    # optional build_container_ref to the bootc image, e.g. quay.io/fedora/fedora-bootc:40
    build_container_ref: str = ""
    # image is the image type, e.g. "ami"
    image: str = ""
    # target_arch is the target archicture, empty means current arch
    target_arch: str = ""
    # rootfs to use (e.g. ext4), some containers like fedora do not
    # have a default rootfs. If unset the container default is used.
    rootfs: str = ""
    # Sign the container_ref and use the new signed image instead of the original one
    sign: bool = False
    # use special disk_config like "lvm"
    disk_config: str = ""
    # use librepo for the downloading
    use_librepo: bool = False
    # podman_terminal enables the podman -t option to get progress
    podman_terminal: bool = False

    def bib_rootfs_args(self):
        if self.rootfs:
            return ["--rootfs", self.rootfs]
        return []

    def __str__(self):
        return ",".join([
            f"{name}={attr}"
            for name, attr in inspect.getmembers(self)
            if not name.startswith("_") and not callable(attr) and attr
        ])


@dataclasses.dataclass
class TestCaseFedora(TestCase):
    container_ref: str = "quay.io/fedora/fedora-bootc:42"
    rootfs: str = "btrfs"
    use_librepo: bool = True


@dataclasses.dataclass
class TestCaseFedora43(TestCase):
    container_ref: str = "quay.io/fedora/fedora-bootc:43"
    rootfs: str = "btrfs"
    use_librepo: bool = True


@dataclasses.dataclass
class TestCaseC9S(TestCase):
    container_ref: str = os.getenv(
        "BIB_TEST_BOOTC_CONTAINER_TAG",
        "quay.io/centos-bootc/centos-bootc:stream9")
    use_librepo: bool = True
    use_terminal: bool = True


@dataclasses.dataclass
class TestCaseC10S(TestCase):
    container_ref: str = os.getenv(
        "BIB_TEST_BOOTC_CONTAINER_TAG",
        "quay.io/centos-bootc/centos-bootc:stream10")
    use_librepo: bool = True


def test_testcase_nameing():
    """
    Ensure the testcase naming does not change without us knowing as those
    are visible when running "pytest --collect-only"
    """
    tc = TestCaseFedora()
    expected = "container_ref=quay.io/fedora/fedora-bootc:40,rootfs=btrfs"
    assert f"{tc}" == expected, f"{tc} != {expected}"


def gen_testcases(what):  # pylint: disable=too-many-return-statements
    if what == "manifest":
        return [TestCaseC9S(), TestCaseFedora(), TestCaseC10S()]
    if what == "default-rootfs":
        # Fedora doesn't have a default rootfs
        return [TestCaseC9S()]
    if what == "ami-boot":
        return [TestCaseC9S(image="ami"), TestCaseFedora(image="ami")]
    if what == "anaconda-iso":
        return [
            # 2024-12-19: disabled for now until the mirror situation becomes
            # a bit more stable
            # TestCaseFedora(image="anaconda-iso", sign=True),
            TestCaseC9S(image="anaconda-iso"),
            TestCaseC10S(image="anaconda-iso"),
        ]
    if what == "qemu-cross":
        test_cases = []
        if platform.machine() == "x86_64":
            test_cases.append(
                TestCaseC9S(image="raw", target_arch="arm64"))
        elif platform.machine() == "arm64":
            # TODO: add arm64->x86_64 cross build test too
            pass
        return test_cases
    if what == "qemu-boot":
        return [
            # test default partitioning
            TestCaseFedora(image="qcow2"),
            # test with custom disk configs
            TestCaseC9S(image="qcow2", disk_config="swap"),
            TestCaseFedora43(image="raw", disk_config="btrfs"),
            TestCaseC9S(image="raw", disk_config="lvm"),
        ]
    if what == "all":
        return [
            klass(image=img)
            for klass in (TestCaseC9S, TestCaseFedora)
            for img in CLOUD_BOOT_IMAGE_TYPES + DISK_IMAGE_TYPES + ["anaconda-iso"]
        ]
    if what == "multidisk":
        # single test that specifies all image types
        image = "+".join(DISK_IMAGE_TYPES)
        return [
            TestCaseC9S(image=image),
            TestCaseFedora(image=image),
        ]
    # Smoke test that all supported --target-arch architecture can
    # create a manifest
    if what == "target-arch-smoke":
        return [
            TestCaseC9S(target_arch="arm64"),
            TestCaseFedora(target_arch="ppc64le"),
            TestCaseFedora(target_arch="s390x"),
        ]
    if what == "build-container":
        return [
            TestCaseC9S(build_container_ref="quay.io/centos-bootc/centos-bootc:stream10", image="qcow2"),
        ]
    raise ValueError(f"unknown test-case type {what}")
