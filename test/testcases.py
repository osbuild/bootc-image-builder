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


def gen_testcases(what):
    # bootc containers that are tested by default
    CONTAINERS_TO_TEST = {
        "fedora": "quay.io/centos-bootc/fedora-bootc:eln",
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
        return CONTAINERS_TO_TEST.values()
    elif what == "ami-boot":
        return [cnt + ",ami" for cnt in CONTAINERS_TO_TEST.values()]
    elif what == "anaconda-iso":
        test_cases = []
        # only fedora right now, centos iso installer is broken right now:
        # https://github.com/osbuild/bootc-image-builder/issues/157
        cnt = CONTAINERS_TO_TEST["fedora"]
        for img_type in INSTALLER_IMAGE_TYPES:
            test_cases.append(f"{cnt},{img_type}")
        return test_cases
    elif what == "qemu-boot":
        test_cases = []
        for cnt in CONTAINERS_TO_TEST.values():
            for img_type in QEMU_BOOT_IMAGE_TYPES:
                test_cases.append(f"{cnt},{img_type}")
        # do a cross arch test too
        if platform.machine() == "x86_64":
            # todo: add fedora:eln
            test_cases.append(
                f'{CONTAINERS_TO_TEST["centos"]},raw,arm64')
        elif platform.machine() == "arm64":
            # TODO: add arm64->x86_64 cross build test too
            pass
        return test_cases
    elif what == "all":
        test_cases = []
        for cnt in CONTAINERS_TO_TEST.values():
            for img_type in QEMU_BOOT_IMAGE_TYPES + \
                    CLOUD_BOOT_IMAGE_TYPES + \
                    NON_QEMU_BOOT_IMAGE_TYPES + \
                    INSTALLER_IMAGE_TYPES:
                test_cases.append(f"{cnt},{img_type}")
        return test_cases
    elif what == "multidisk":
        # single test that specifies all image types
        test_cases = []
        for cnt in CONTAINERS_TO_TEST.values():
            img_type = "+".join(DISK_IMAGE_TYPES)
            test_cases.append(f"{cnt},{img_type}")
        return test_cases
    raise ValueError(f"unknown test-case type {what}")
