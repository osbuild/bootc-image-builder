import dataclasses
import inspect
import os
import platform

# supported images that can be directly booted
QEMU_BOOT_IMAGE_TYPES = ("qcow2", "raw")

# images that can *not* be booted directly from qemu
NON_QEMU_BOOT_IMAGE_TYPES = ("vmdk",)

# disk image types can be build from a single manifest
DISK_IMAGE_TYPES = QEMU_BOOT_IMAGE_TYPES + NON_QEMU_BOOT_IMAGE_TYPES

# supported images that can be booted in a cloud
CLOUD_BOOT_IMAGE_TYPES = ("ami",)

# supported images that require an install
INSTALLER_IMAGE_TYPES = ("anaconda-iso",)


@dataclasses.dataclass(frozen=True)
class TestCase:
    # container_ref to the bootc image, e.g. quay.io/fedora/fedora-bootc:40
    container_ref: str
    # image is the image type, e.g. "ami"
    image: str = ""
    # target_arch is the target archicture, empty means current arch
    target_arch: str = ""
    # local means that the container should be pulled locally ("--local" flag)
    local: bool = False

    def rootfs_args(self):
        # fedora has no default rootfs so it must be specified
        if "fedora-bootc" in self.container_ref:
            return ["--rootfs", "btrfs"]
        return []

    def __str__(self):
        return ",".join([
            attr
            for name, attr in inspect.getmembers(self)
            if not name.startswith("_") and not callable(attr) and attr
        ])


def gen_testcases(what):
    # bootc containers that are tested by default
    CONTAINERS_TO_TEST = {
        "fedora": "quay.io/fedora/fedora-bootc:40",
        "centos": "quay.io/centos-bootc/centos-bootc:stream9",
    }
    # allow commandline override, this is used when testing
    # custom images
    if os.getenv("BIB_TEST_BOOTC_CONTAINER_TAG"):
        # TODO: make this more elegant
        CONTAINERS_TO_TEST = {
            "centos": os.getenv("BIB_TEST_BOOTC_CONTAINER_TAG"),
            "fedora": [],
        }

    if what == "manifest":
        return [TestCase(container_ref=ref)
                for ref in CONTAINERS_TO_TEST.values()]
    elif what == "default-rootfs":
        # Fedora doesn't have a default rootfs
        return [TestCase(container_ref=CONTAINERS_TO_TEST["centos"])]
    elif what == "ami-boot":
        test_cases = []
        for ref in CONTAINERS_TO_TEST.values():
            test_cases.append(TestCase(container_ref=ref, image="ami"))
        return test_cases
    elif what == "anaconda-iso":
        test_cases = []
        for ref in CONTAINERS_TO_TEST.values():
            for img_type in INSTALLER_IMAGE_TYPES:
                test_cases.append(TestCase(container_ref=ref, image=img_type))
        return test_cases
    elif what == "qemu-boot":
        test_cases = []
        for distro, ref in CONTAINERS_TO_TEST.items():
            for img_type in QEMU_BOOT_IMAGE_TYPES:
                test_cases.append(
                    TestCase(container_ref=ref, image=img_type))
        # do a cross arch test too
        if platform.machine() == "x86_64":
            # todo: add fedora:eln
            test_cases.append(TestCase(container_ref=ref, image="raw", target_arch="arm64"))
        elif platform.machine() == "arm64":
            # TODO: add arm64->x86_64 cross build test too
            pass
        return test_cases
    elif what == "all":
        test_cases = []
        for ref in CONTAINERS_TO_TEST.values():
            for img_type in QEMU_BOOT_IMAGE_TYPES + \
                    CLOUD_BOOT_IMAGE_TYPES + \
                    NON_QEMU_BOOT_IMAGE_TYPES + \
                    INSTALLER_IMAGE_TYPES:
                test_cases.append(TestCase(container_ref=ref, image=img_type))
        return test_cases
    elif what == "multidisk":
        # single test that specifies all image types
        test_cases = []
        for ref in CONTAINERS_TO_TEST.values():
            test_cases.append(TestCase(container_ref=ref, image="+".join(DISK_IMAGE_TYPES)))
        return test_cases
    raise ValueError(f"unknown test-case type {what}")
