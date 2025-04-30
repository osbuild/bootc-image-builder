import platform

import pytest

from testcases import gen_testcases

from test_build_disk import (  # pylint: disable=unused-import
    assert_disk_image_boots,
    build_container_fixture,
    gpg_conf_fixture,
    image_type_fixture,
    registry_conf_fixture,
    shared_tmpdir_fixture,
)


# This testcase is not part of "test_build_disk.py:test_image_boots"
# because it takes ~30min on the GH runners so moving it into a
# separate file ensures it is run in parallel on GH.
@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("image_type", gen_testcases("qemu-cross"), indirect=["image_type"])
def test_image_boots_cross(image_type):
    assert_disk_image_boots(image_type)
