import platform
import os


def gen_testcases(what):
    # supported images that can be directly booted
    DIRECT_BOOT_IMAGE_TYPES = ("qcow2", "ami", "raw")
    # supported images that require an install
    INSTALLER_IMAGE_TYPES = ("anaconda-iso",)

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
    elif what == "direct-boot":
        # skip some raw/ami tests (they are identical right now) to
        # avoid overlong test runs but revisit this later and maybe just
        # do more in parallel?
        test_cases = [
            CONTAINERS_TO_TEST["centos"] + "," + DIRECT_BOOT_IMAGE_TYPES[0],
            CONTAINERS_TO_TEST["fedora"] + "," + DIRECT_BOOT_IMAGE_TYPES[1],
            CONTAINERS_TO_TEST["centos"] + "," + DIRECT_BOOT_IMAGE_TYPES[2],
            CONTAINERS_TO_TEST["fedora"] + "," + DIRECT_BOOT_IMAGE_TYPES[0],
        ]
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
            for img_type in DIRECT_BOOT_IMAGE_TYPES + INSTALLER_IMAGE_TYPES:
                test_cases.append(f"{cnt},{img_type}")
        return test_cases
    raise ValueError(f"unknown test-case type {what}")
