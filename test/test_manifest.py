import base64
import hashlib
import json
import pathlib
import platform
import subprocess
import textwrap

import pytest

import testutil
from containerbuild import build_container_fixture  # pylint: disable=unused-import
from containerbuild import make_container
from testcases import gen_testcases

if not testutil.has_executable("podman"):
    pytest.skip("no podman, skipping integration tests that required podman", allow_module_level=True)
if not testutil.can_start_rootful_containers():
    pytest.skip("tests require to be able to run rootful containers (try: sudo)", allow_module_level=True)


def find_image_size_from(manifest_str):
    manifest = json.loads(manifest_str)
    for pipl in manifest["pipelines"]:
        if pipl["name"] == "image":
            for st in pipl["stages"]:
                if st["type"] == "org.osbuild.truncate":
                    return st["options"]["size"]
    raise ValueError(f"cannot find disk size in manifest:\n{manifest_str}")


@pytest.mark.parametrize("tc", gen_testcases("manifest"))
def test_manifest_smoke(build_container, tc):
    testutil.pull_container(tc.container_ref, tc.target_arch)

    output = subprocess.check_output([
        *testutil.podman_run_common,
        build_container,
        "manifest",
        *tc.bib_rootfs_args(),
        f"{tc.container_ref}",
    ])
    manifest = json.loads(output)
    # just some basic validation
    assert manifest["version"] == "2"
    assert manifest["pipelines"][0]["name"] == "build"
    # default disk size is 10G
    disk_size = find_image_size_from(output)
    # default image size is 10G
    assert int(disk_size) == 10 * 1024 * 1024 * 1024


@pytest.mark.parametrize("tc", gen_testcases("anaconda-iso"))
def test_iso_manifest_smoke(build_container, tc):
    testutil.pull_container(tc.container_ref, tc.target_arch)

    output = subprocess.check_output([
        *testutil.podman_run_common,
        build_container,
        "manifest",
        "--type=anaconda-iso", f"{tc.container_ref}",
    ])
    manifest = json.loads(output)
    # just some basic validation
    expected_pipeline_names = ["build", "anaconda-tree", "efiboot-tree", "bootiso-tree", "bootiso"]
    assert manifest["version"] == "2"
    assert [pipeline["name"] for pipeline in manifest["pipelines"]] == expected_pipeline_names


@pytest.mark.parametrize("tc", gen_testcases("manifest"))
def test_manifest_disksize(tmp_path, build_container, tc):
    testutil.pull_container(tc.container_ref, tc.target_arch)

    # create derrived container with 6G silly file to ensure that
    # bib doubles the size to 12G+
    cntf_path = tmp_path / "Containerfile"
    cntf_path.write_text(textwrap.dedent(f"""\n
    FROM {tc.container_ref}
    RUN truncate -s 2G /big-file1
    RUN truncate -s 2G /big-file2
    RUN truncate -s 2G /big-file3
    """), encoding="utf8")

    print(f"building big size container from {tc.container_ref}")
    with make_container(tmp_path) as container_tag:
        print(f"using {container_tag}")
        manifest_str = subprocess.check_output([
            *testutil.podman_run_common,
            build_container,
            "manifest",
            *tc.bib_rootfs_args(),
            f"localhost/{container_tag}",
        ], encoding="utf8")
        # ensure disk size is bigger than the default 10G
        disk_size = find_image_size_from(manifest_str)
        assert int(disk_size) > 11_000_000_000


def test_manifest_local_checks_containers_storage_errors(build_container):
    # note that the
    #   "-v /var/lib/containers/storage:/var/lib/containers/storage"
    # is missing here
    res = subprocess.run([
        # not using *testutil.podman_run_common to test bad usage
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        build_container,
        "manifest", "arg-not-used",
    ], check=False, stdout=subprocess.PIPE, stderr=subprocess.PIPE, encoding="utf8")
    assert res.returncode == 1
    err = 'could not access container storage, ' + \
        'did you forget -v /var/lib/containers/storage:/var/lib/containers/storage?'
    assert err in res.stderr


@pytest.mark.parametrize("tc", gen_testcases("manifest"))
def test_manifest_local_checks_containers_storage_works(tmp_path, build_container, tc):
    testutil.pull_container(tc.container_ref, tc.target_arch)

    cntf_path = tmp_path / "Containerfile"
    cntf_path.write_text(textwrap.dedent(f"""\n
    FROM {tc.container_ref}
    """), encoding="utf8")

    with make_container(tmp_path) as container_tag:
        subprocess.run([
            *testutil.podman_run_common,
            build_container,
            "manifest",
            *tc.bib_rootfs_args(),
            f"localhost/{container_tag}",
        ], check=True, encoding="utf8")


@pytest.mark.skipif(platform.uname().machine != "x86_64", reason="cross build test only runs on x86")
def test_manifest_cross_arch_check(tmp_path, build_container):
    cntf_path = tmp_path / "Containerfile"
    cntf_path.write_text(textwrap.dedent("""\n
    # build for x86_64 only
    FROM quay.io/centos-bootc/centos-bootc:stream9
    """), encoding="utf8")

    with make_container(tmp_path, arch="x86_64") as container_tag:
        with pytest.raises(subprocess.CalledProcessError) as exc:
            subprocess.run([
                *testutil.podman_run_common,
                build_container,
                "manifest", "--target-arch=aarch64",
                f"localhost/{container_tag}"
            ], check=True, capture_output=True, encoding="utf8")
        assert 'image found is for unexpected architecture "x86_64"' in exc.value.stderr


def find_rootfs_type_from(manifest_str):
    manifest = json.loads(manifest_str)
    for pipl in manifest["pipelines"]:
        if pipl["name"] == "image":
            for st in pipl["stages"]:
                if st["type"].startswith("org.osbuild.mkfs."):
                    if st.get("options", {}).get("label") == "root":
                        return st["type"].rpartition(".")[2]
    raise ValueError(f"cannot find rootfs type in manifest:\n{manifest_str}")


@pytest.mark.parametrize("tc", gen_testcases("default-rootfs"))
def test_manifest_rootfs_respected(build_container, tc):
    # TODO: derive container and fake "bootc install print-configuration"?
    testutil.pull_container(tc.container_ref)
    output = subprocess.check_output([
        *testutil.podman_run_common,
        build_container,
        "manifest", f"{tc.container_ref}",
    ])
    rootfs_type = find_rootfs_type_from(output)
    match tc.container_ref:
        case "quay.io/centos-bootc/centos-bootc:stream9":
            assert rootfs_type == "xfs"
        case _:
            pytest.fail(f"unknown container_ref {tc.container_ref} please update test")


def test_manifest_rootfs_override(build_container):
    # no need to parameterize this test, --rootfs behaves same for all containers
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"

    output = subprocess.check_output([
        *testutil.podman_run_common,
        build_container,
        "manifest", "--rootfs", "btrfs", f"{container_ref}",
    ])
    rootfs_type = find_rootfs_type_from(output)
    assert rootfs_type == "btrfs"


def find_user_stage_from(manifest_str):
    manifest = json.loads(manifest_str)
    for pipl in manifest["pipelines"]:
        if pipl["name"] == "image":
            for st in pipl["stages"]:
                if st["type"] == "org.osbuild.users":
                    return st
    raise ValueError(f"cannot find users stage in manifest:\n{manifest_str}")


def test_manifest_user_customizations_toml(tmp_path, build_container):
    # no need to parameterize this test, toml is the same for all containers
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    testutil.pull_container(container_ref)

    config_toml_path = tmp_path / "config.toml"
    config_toml_path.write_text(textwrap.dedent("""\
    [[customizations.user]]
    name = "alice"
    password = "$5$xx$aabbccddeeffgghhiijj"  # notsecret
    key = "ssh-rsa AAA ... user@email.com"
    groups = ["wheel"]
    """))
    output = subprocess.check_output([
        *testutil.podman_run_common,
        "-v", f"{config_toml_path}:/config.toml:ro",
        build_container,
        "manifest", f"{container_ref}",
    ])
    user_stage = find_user_stage_from(output)
    assert user_stage["options"]["users"].get("alice") == {
        # use very fake password here, if it looks too real the
        # infosec "leak detect" get very nervous
        "password": "$5$xx$aabbccddeeffgghhiijj",  # notsecret
        "key": "ssh-rsa AAA ... user@email.com",
        "groups": ["wheel"],
    }


def test_manifest_installer_customizations(tmp_path, build_container):
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    testutil.pull_container(container_ref)

    config_toml_path = tmp_path / "config.toml"
    config_toml_path.write_text(textwrap.dedent("""\
    [customizations.installer.kickstart]
    contents = \"\"\"
    autopart --type=lvm
    \"\"\"
    """))
    output = subprocess.check_output([
        *testutil.podman_run_common,
        "-v", f"{config_toml_path}:/config.toml:ro",
        build_container,
        "manifest", "--type=anaconda-iso", f"{container_ref}",
    ])
    manifest = json.loads(output)

    # expected values for the following inline file contents
    ks_content = textwrap.dedent("""\
    %include /run/install/repo/osbuild-base.ks
    autopart --type=lvm
    """).encode("utf8")
    expected_data = base64.b64encode(ks_content).decode()
    expected_content_hash = hashlib.sha256(ks_content).hexdigest()
    expected_content_id = f"sha256:{expected_content_hash}"   # hash with algo prefix

    # check the inline source for the custom kickstart contents
    assert expected_content_id in manifest["sources"]["org.osbuild.inline"]["items"]
    assert manifest["sources"]["org.osbuild.inline"]["items"][expected_content_id]["data"] == expected_data


def test_mount_ostree_error(tmpdir_factory, build_container):
    # no need to parameterize this test, toml is the same for all containers
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    testutil.pull_container(container_ref)

    cfg = {
        "blueprint": {
            "customizations": {
                "filesystem": [
                    {
                        "mountpoint": "/",
                        "minsize": "12GiB"
                    },
                    {
                        "mountpoint": "/var/log",
                        "minsize": "1GiB"
                    },
                    {
                        "mountpoint": "/ostree",
                        "minsize": "10GiB"
                    },
                ]
            },
        },
    }

    output_path = pathlib.Path(tmpdir_factory.mktemp("data")) / "output"
    output_path.mkdir(exist_ok=True)
    config_json_path = output_path / "config.json"
    config_json_path.write_text(json.dumps(cfg), encoding="utf-8")

    with pytest.raises(subprocess.CalledProcessError) as exc:
        subprocess.check_output([
            *testutil.podman_run_common,
            "-v", f"{output_path}:/output",
            build_container,
            "manifest", f"{container_ref}",
            "--config", "/output/config.json",
        ], stderr=subprocess.PIPE, encoding="utf8")
    assert 'The following errors occurred while validating custom mountpoints:\npath "/ostree" is not allowed' \
        in exc.value.stderr


@pytest.mark.parametrize(
    "container_ref,should_error,expected_error",
    [
        ("quay.io/centos/centos:stream9", True, "image quay.io/centos/centos:stream9 is not a bootc image"),
        ("quay.io/centos-bootc/centos-bootc:stream9", False, None),
    ],
)
def test_manifest_checks_build_container_is_bootc(build_container, container_ref, should_error, expected_error):
    def check_image_ref():
        testutil.pull_container(container_ref)
        subprocess.check_output([
            *testutil.podman_run_common,
            build_container,
            "manifest",
            container_ref,
        ], stderr=subprocess.PIPE, encoding="utf8")
    if should_error:
        with pytest.raises(subprocess.CalledProcessError) as exc:
            check_image_ref()
            assert expected_error in exc.value.stderr
    else:
        check_image_ref()


@pytest.mark.parametrize("tc", gen_testcases("target-arch-smoke"))
def test_manifest_target_arch_smoke(build_container, tc):
    testutil.pull_container(tc.container_ref, tc.target_arch)

    # TODO: actually build an image too
    output = subprocess.check_output([
        *testutil.podman_run_common,
        build_container,
        "manifest",
        *tc.bib_rootfs_args(),
        f"--target-arch={tc.target_arch}",
        tc.container_ref,
    ])
    manifest = json.loads(output)
    # just minimal validation, we could in theory look at the partition
    # table be beside this there is relatively little that is different
    assert manifest["version"] == "2"
    assert manifest["pipelines"][0]["name"] == "build"


def find_image_anaconda_stage(manifest_str):
    manifest = json.loads(manifest_str)
    for pipl in manifest["pipelines"]:
        if pipl["name"] == "anaconda-tree":
            for st in pipl["stages"]:
                if st["type"] == "org.osbuild.anaconda":
                    return st
    raise ValueError(f"cannot find disk size in manifest:\n{manifest_str}")


@pytest.mark.parametrize("tc", gen_testcases("anaconda-iso"))
def test_manifest_anaconda_module_customizations(tmpdir_factory, build_container, tc):
    testutil.pull_container(tc.container_ref, tc.target_arch)

    cfg = {
        "customizations": {
            "installer": {
                "modules": {
                    "enable": [
                        "org.fedoraproject.Anaconda.Modules.Localization",
                        # disable takes precedence
                        "org.fedoraproject.Anaconda.Modules.Timezone",
                    ],
                    "disable": [
                        # defaults can be disabled as well
                        "org.fedoraproject.Anaconda.Modules.Users",
                        # disable takes precedence
                        "org.fedoraproject.Anaconda.Modules.Timezone",
                    ]
                },
            },
        },
    }
    output_path = pathlib.Path(tmpdir_factory.mktemp("data")) / "output"
    output_path.mkdir(exist_ok=True)
    config_json_path = output_path / "config.json"
    config_json_path.write_text(json.dumps(cfg), encoding="utf-8")

    output = subprocess.check_output([
        *testutil.podman_run_common,
        "-v", f"{output_path}:/output",
        build_container,
        "manifest",
        "--config", "/output/config.json",
        *tc.bib_rootfs_args(),
        "--type=anaconda-iso", tc.container_ref,
    ])
    st = find_image_anaconda_stage(output)
    assert "org.fedoraproject.Anaconda.Modules.Localization" in st["options"]["activatable-modules"]
    assert "org.fedoraproject.Anaconda.Modules.Users" not in st["options"]["activatable-modules"]
    assert "org.fedoraproject.Anaconda.Modules.Timezone" not in st["options"]["activatable-modules"]


def find_fstab_stage_from(manifest_str):
    manifest = json.loads(manifest_str)
    for pipeline in manifest["pipelines"]:
        # the fstab stage in cross-arch manifests is in the "ostree-deployment" pipeline
        if pipeline["name"] in ("image", "ostree-deployment"):
            for st in pipeline["stages"]:
                if st["type"] == "org.osbuild.fstab":
                    return st
    raise ValueError(f"cannot find fstab stage in manifest:\n{manifest_str}")


@pytest.mark.parametrize("fscustomizations,rootfs", [
    ({"/var/data": "2 GiB", "/var/stuff": "10 GiB"}, "xfs"),
    ({"/var/data": "2 GiB", "/var/stuff": "10 GiB"}, "ext4"),
    ({"/": "2 GiB", "/boot": "1 GiB"}, "ext4"),
    ({"/": "2 GiB", "/boot": "1 GiB", "/var/data": "42 GiB"}, "ext4"),
    ({"/": "2 GiB"}, "btrfs"),
    ({}, "ext4"),
    ({}, "xfs"),
    ({}, "btrfs"),
])
def test_manifest_fs_customizations(tmp_path, build_container, fscustomizations, rootfs):
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    testutil.pull_container(container_ref)

    config = {
        "customizations": {
            "filesystem": [{"mountpoint": mnt, "minsize": minsize} for mnt, minsize in fscustomizations.items()],
        },
    }
    config_path = tmp_path / "config.json"
    with config_path.open("w") as config_file:
        json.dump(config, config_file)
    output = subprocess.check_output([
        *testutil.podman_run_common,
        "-v", f"{config_path}:/config.json:ro",
        "--entrypoint=/usr/bin/bootc-image-builder",
        build_container,
        f"--rootfs={rootfs}",
        "manifest", f"{container_ref}",
    ])
    assert_fs_customizations(fscustomizations, rootfs, output)


def test_manifest_fs_customizations_smoke_toml(tmp_path, build_container):
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    rootfs = "xfs"

    expected_fs_customizations = {
        "/": 10 * 1024 * 1024 * 1024,
        "/var/data": 20 * 1024 * 1024 * 1024,
    }

    config_toml_path = tmp_path / "config.toml"
    config_toml_path.write_text(textwrap.dedent("""\
    [[customizations.filesystem]]
    mountpoint = "/"
    minsize = "10 GiB"

    [[customizations.filesystem]]
    mountpoint = "/var/data"
    minsize = "20 GiB"
    """))
    output = subprocess.check_output([
        *testutil.podman_run_common,
        "-v", f"{config_toml_path}:/config.toml:ro",
        "--entrypoint=/usr/bin/bootc-image-builder",
        build_container,
        f"--rootfs={rootfs}",
        "manifest", f"{container_ref}",
    ])
    assert_fs_customizations(expected_fs_customizations, rootfs, output)


def assert_fs_customizations(customizations, fstype, manifest):
    # use the fstab stage to get filesystem types for each mountpoint
    fstab_stage = find_fstab_stage_from(manifest)
    filesystems = fstab_stage["options"]["filesystems"]

    manifest_mountpoints = set()
    for fs in filesystems:
        manifest_mountpoints.add(fs["path"])
        if fs["path"] == "/boot/efi":
            assert fs["vfs_type"] == "vfat"
            continue

        if fstype == "btrfs" and fs["path"] == "/boot":
            # /boot keeps its default fstype when using btrfs
            assert fs["vfs_type"] == "ext4"
            continue

        assert fs["vfs_type"] == fstype, f"incorrect filesystem type for {fs['path']}"

    # check that all fs customizations appear in fstab
    for custom_mountpoint in customizations:
        assert custom_mountpoint in manifest_mountpoints


@pytest.mark.skipif(platform.uname().machine != "x86_64", reason="cross arch test only runs on x86")
@pytest.mark.parametrize("fscustomizations,rootfs", [
    ({"/var/data": "2 GiB", "/var/stuff": "10 GiB"}, "xfs"),
    ({"/var/data": "2 GiB", "/var/stuff": "10 GiB"}, "ext4"),
    ({"/": "2 GiB", "/boot": "1 GiB"}, "ext4"),
    ({"/": "2 GiB", "/boot": "1 GiB", "/var/data": "42 GiB"}, "ext4"),
    ({"/": "2 GiB"}, "btrfs"),
    ({}, "ext4"),
    ({}, "xfs"),
    ({}, "btrfs"),
])
def test_manifest_fs_customizations_xarch(tmp_path, build_container, fscustomizations, rootfs):
    target_arch = "aarch64"
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    testutil.pull_container(container_ref, target_arch)

    config = {
        "customizations": {
            "filesystem": [{"mountpoint": mnt, "minsize": minsize} for mnt, minsize in fscustomizations.items()],
        },
    }
    config_path = tmp_path / "config.json"
    with config_path.open("w") as config_file:
        json.dump(config, config_file)
    output = subprocess.check_output([
        *testutil.podman_run_common,
        "-v", f"{config_path}:/config.json:ro",
        "--entrypoint=/usr/bin/bootc-image-builder",
        build_container,
        f"--rootfs={rootfs}",
        f"--target-arch={target_arch}",
        "manifest", f"{container_ref}",
    ])

    # cross-arch builds only support ext4 (for now)
    assert_fs_customizations(fscustomizations, "ext4", output)


def find_grub2_iso_stage_from(manifest_str):
    manifest = json.loads(manifest_str)
    for pipl in manifest["pipelines"]:
        for st in pipl["stages"]:
            if st["type"] == "org.osbuild.grub2.iso":
                return st
    raise ValueError(f"cannot find grub2.iso stage in manifest:\n{manifest_str}")


def test_manifest_fips_customization(tmp_path, build_container):
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    testutil.pull_container(container_ref)

    config = {
        "customizations": {
            "fips": True,
        },
    }
    config_path = tmp_path / "config.json"
    with config_path.open("w") as config_file:
        json.dump(config, config_file)
    output = subprocess.check_output([
        *testutil.podman_run_common,
        "-v", f"{config_path}:/config.json:ro",
        build_container,
        # XXX: test for qcow2 too
        "--type=anaconda-iso",
        "manifest", f"{container_ref}",
    ], text=True)
    st = find_grub2_iso_stage_from(output)
    assert "fips=1" in st["options"]["kernel"]["opts"]


def find_bootc_install_to_fs_stage_from(manifest_str):
    manifest = json.loads(manifest_str)
    for pipeline in manifest["pipelines"]:
        # the fstab stage in cross-arch manifests is in the "ostree-deployment" pipeline
        if pipeline["name"] == "image":
            for st in pipeline["stages"]:
                if st["type"] == "org.osbuild.bootc.install-to-filesystem":
                    return st
    raise ValueError(f"cannot find bootc.install-to-filesystem stage in manifest:\n{manifest_str}")


def test_manifest_disk_customization_lvm(tmp_path, build_container):
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    testutil.pull_container(container_ref)

    config = {
        "customizations": {
            "disk": {
                "partitions": [
                    {
                        "type": "lvm",
                        "minsize": "10 GiB",
                        "logical_volumes": [
                            {
                                "minsize": "10 GiB",
                                "fs_type": "ext4",
                                "mountpoint": "/",
                            }
                        ]
                    }
                ]
            }
        }
    }
    config_path = tmp_path / "config.json"
    with config_path.open("w") as config_file:
        json.dump(config, config_file)

    testutil.pull_container(container_ref)
    output = subprocess.check_output([
        *testutil.podman_run_common,
        "-v", f"{config_path}:/config.json:ro",
        build_container,
        "manifest", f"{container_ref}",
    ])
    st = find_bootc_install_to_fs_stage_from(output)
    assert st["devices"]["rootlv"]["type"] == "org.osbuild.lvm2.lv"


def test_manifest_disk_customization_btrfs(tmp_path, build_container):
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"

    config = {
        "customizations": {
            "disk": {
                "partitions": [
                    {
                        "type": "btrfs",
                        "minsize": "10 GiB",
                        "subvolumes": [
                            {
                                "name": "root",
                                "mountpoint": "/",
                            }
                        ]
                    }
                ]
            }
        }
    }
    config_path = tmp_path / "config.json"
    with config_path.open("w") as config_file:
        json.dump(config, config_file)

    testutil.pull_container(container_ref)
    output = subprocess.check_output([
        *testutil.podman_run_common,
        "-v", f"{config_path}:/config.json:ro",
        build_container,
        "manifest", f"{container_ref}",
    ])
    st = find_bootc_install_to_fs_stage_from(output)
    assert st["mounts"][0]["type"] == "org.osbuild.btrfs"
    assert st["mounts"][0]["target"] == "/"


def find_mkswap_stage_from(manifest_str):
    manifest = json.loads(manifest_str)
    for pipeline in manifest["pipelines"]:
        if pipeline["name"] == "image":
            for st in pipeline["stages"]:
                if st["type"] == "org.osbuild.mkswap":
                    return st
    raise ValueError(f"cannot find mkswap stage in manifest:\n{manifest_str}")


def test_manifest_disk_customization_swap(tmp_path, build_container):
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"

    config = {
        "customizations": {
            "disk": {
                "partitions": [
                    {
                        "minsize": "2 GiB",
                        "fs_type": "swap",
                    }
                ]
            }
        }
    }
    config_path = tmp_path / "config.json"
    with config_path.open("w") as config_file:
        json.dump(config, config_file)

    testutil.pull_container(container_ref)
    output = subprocess.check_output([
        *testutil.podman_run_common,
        "-v", f"{config_path}:/config.json:ro",
        build_container,
        "manifest", f"{container_ref}",
    ])
    mkswap_stage = find_mkswap_stage_from(output)
    assert mkswap_stage["options"].get("uuid")
    swap_uuid = mkswap_stage["options"]["uuid"]
    fstab_stage = find_fstab_stage_from(output)
    filesystems = fstab_stage["options"]["filesystems"]
    assert {
        'uuid': swap_uuid,
        "vfs_type": "swap",
        "path": "none",
        "options": "defaults",
    } in filesystems


def test_manifest_disk_customization_lvm_swap(tmp_path, build_container):
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"

    config = {
        "customizations": {
            "disk": {
                "partitions": [
                    {
                        "type": "lvm",
                        "minsize": "10 GiB",
                        "logical_volumes": [
                            {
                                "minsize": "2 GiB",
                                "fs_type": "swap",
                            }
                        ]
                    }
                ]
            }
        }
    }
    config_path = tmp_path / "config.json"
    with config_path.open("w") as config_file:
        json.dump(config, config_file)

    testutil.pull_container(container_ref)
    output = subprocess.check_output([
        *testutil.podman_run_common,
        "-v", f"{config_path}:/config.json:ro",
        build_container,
        "manifest", f"{container_ref}",
    ])
    mkswap_stage = find_mkswap_stage_from(output)
    assert mkswap_stage["options"].get("uuid")
    swap_uuid = mkswap_stage["options"]["uuid"]
    fstab_stage = find_fstab_stage_from(output)
    filesystems = fstab_stage["options"]["filesystems"]
    assert {
        'uuid': swap_uuid,
        "vfs_type": "swap",
        "path": "none",
        "options": "defaults",
    } in filesystems
    # run osbuild schema validation, see gh#748
    if not testutil.has_executable("osbuild"):
        pytest.skip("no osbuild executable")
    osbuild_manifest_path = tmp_path / "manifest.json"
    osbuild_manifest_path.write_bytes(output)
    subprocess.run(["osbuild", osbuild_manifest_path.as_posix()], check=True)


@pytest.mark.parametrize("use_librepo", [False, True])
def test_iso_manifest_use_librepo(build_container, use_librepo):
    # no need to parameterize this test, --use-librepo behaves same for all containers
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    testutil.pull_container(container_ref)

    output = subprocess.check_output([
        *testutil.podman_run_common,
        build_container,
        "manifest",
        "--type=anaconda-iso",
        container_ref,
        f"--use-librepo={use_librepo}",
    ])
    manifest = json.loads(output)
    if use_librepo:
        assert "org.osbuild.librepo" in manifest["sources"]
    else:
        assert "org.osbuild.curl" in manifest["sources"]


def test_manifest_customization_custom_file_smoke(tmp_path, build_container):
    # no need to parameterize this test, toml is the same for all containers
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"
    testutil.pull_container(container_ref)

    cfg = {
        "blueprint": {
            "customizations": {
                "files": [
                    {
                        "path": "/etc/custom_file",
                        "data": "hello world"
                    },
                ],
                "directories": [
                    {
                        "path": "/etc/custom_dir",
                    },
                ],
            },
        },
    }

    output_path = tmp_path / "output"
    output_path.mkdir(exist_ok=True)
    config_json_path = output_path / "config.json"
    config_json_path.write_text(json.dumps(cfg), encoding="utf-8")

    output = subprocess.check_output([
        *testutil.podman_run_common,
        "-v", f"{output_path}:/output",
        build_container,
        "manifest", f"{container_ref}",
        "--config", "/output/config.json",
    ], stderr=subprocess.PIPE, encoding="utf8")
    osbuild_manifest = json.loads(output)
    assert '"to":"tree:///etc/custom_file"' in output
    assert '{"type":"org.osbuild.mkdir","options":{"paths":[{"path":"/etc/custom_dir","exist_ok":true}]}}' in output
