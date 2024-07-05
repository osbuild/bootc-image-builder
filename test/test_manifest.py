import base64
import hashlib
import json
import pathlib
import platform
import subprocess
import textwrap

import pytest

import testutil
from containerbuild import build_container_fixture  # noqa: F401
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
    output = subprocess.check_output([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "--entrypoint=/usr/bin/bootc-image-builder",
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
    output = subprocess.check_output([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "--entrypoint=/usr/bin/bootc-image-builder",
        build_container,
        "manifest",
        *tc.bib_rootfs_args(),
        "--type=anaconda-iso", f"{tc.container_ref}",
    ])
    manifest = json.loads(output)
    # just some basic validation
    expected_pipeline_names = ["build", "anaconda-tree", "rootfs-image", "efiboot-tree", "bootiso-tree", "bootiso"]
    assert manifest["version"] == "2"
    assert [pipeline["name"] for pipeline in manifest["pipelines"]] == expected_pipeline_names


@pytest.mark.parametrize("tc", gen_testcases("manifest"))
def test_manifest_disksize(tmp_path, build_container, tc):
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
            "podman", "run", "--rm",
            "--privileged",
            "--security-opt", "label=type:unconfined_t",
            # ensure local storage is here
            "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
            # need different entry point
            "--entrypoint", "/usr/bin/bootc-image-builder",
            build_container,
            "manifest", "--local",
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
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "--entrypoint=/usr/bin/bootc-image-builder",
        build_container,
        "manifest", "--local", "arg-not-used",
    ], check=False, stdout=subprocess.PIPE, stderr=subprocess.PIPE, encoding="utf8")
    assert res.returncode == 1
    err = 'local storage not working, did you forget -v /var/lib/containers/storage:/var/lib/containers/storage?'
    assert err in res.stderr


@pytest.mark.parametrize("tc", gen_testcases("manifest"))
def test_manifest_local_checks_containers_storage_works(tmp_path, build_container, tc):
    cntf_path = tmp_path / "Containerfile"
    cntf_path.write_text(textwrap.dedent(f"""\n
    FROM {tc.container_ref}
    """), encoding="utf8")

    with make_container(tmp_path) as container_tag:
        subprocess.run([
            "podman", "run", "--rm",
            "--privileged",
            "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
            "--security-opt", "label=type:unconfined_t",
            "--entrypoint=/usr/bin/bootc-image-builder",
            build_container,
            "manifest", "--local",
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
                "podman", "run", "--rm",
                "--privileged",
                "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
                "--security-opt", "label=type:unconfined_t",
                "--entrypoint=/usr/bin/bootc-image-builder",
                build_container,
                "manifest", "--target-arch=aarch64",
                "--local", f"localhost/{container_tag}"
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
    output = subprocess.check_output([
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "--entrypoint=/usr/bin/bootc-image-builder",
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
        "podman", "run", "--rm",
        "--privileged",
        "--security-opt", "label=type:unconfined_t",
        "--entrypoint=/usr/bin/bootc-image-builder",
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

    config_toml_path = tmp_path / "config.toml"
    config_toml_path.write_text(textwrap.dedent("""\
    [[customizations.user]]
    name = "alice"
    password = "$5$xx$aabbccddeeffgghhiijj"  # notsecret
    key = "ssh-rsa AAA ... user@email.com"
    groups = ["wheel"]
    """))
    output = subprocess.check_output([
        "podman", "run", "--rm",
        "--privileged",
        "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
        "-v", f"{config_toml_path}:/config.toml:ro",
        "--security-opt", "label=type:unconfined_t",
        "--entrypoint=/usr/bin/bootc-image-builder",
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

    config_toml_path = tmp_path / "config.toml"
    config_toml_path.write_text(textwrap.dedent("""\
    [customizations.installer.kickstart]
    contents = \"\"\"
    autopart --type=lvm
    \"\"\"
    """))
    output = subprocess.check_output([
        "podman", "run", "--rm",
        "--privileged",
        "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
        "-v", f"{config_toml_path}:/config.toml:ro",
        "--security-opt", "label=type:unconfined_t",
        "--entrypoint=/usr/bin/bootc-image-builder",
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

    CFG = {
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
    config_json_path.write_text(json.dumps(CFG), encoding="utf-8")

    with pytest.raises(subprocess.CalledProcessError) as exc:
        subprocess.check_output([
            "podman", "run", "--rm",
            "--privileged",
            "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
            "--security-opt", "label=type:unconfined_t",
            "-v", f"{output_path}:/output",
            "--entrypoint=/usr/bin/bootc-image-builder",
            build_container,
            "manifest", f"{container_ref}",
            "--config", "/output/config.json",
        ], stderr=subprocess.PIPE, encoding="utf8")
    assert 'The following custom mountpoints are not supported ["/ostree"]' in exc.value.stderr


@pytest.mark.parametrize(
    "container_ref,should_error,expected_error",
    [
        ("quay.io/centos/centos:stream9", True, "image quay.io/centos/centos:stream9 is not a bootc image"),
        ("quay.io/centos-bootc/centos-bootc:stream9", False, None),
    ],
)
def test_manifest_checks_build_container_is_bootc(build_container, container_ref, should_error, expected_error):
    def check_image_ref():
        subprocess.check_output([
            "podman", "run", "--rm",
            "--privileged",
            "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
            "--security-opt", "label=type:unconfined_t",
            f'--entrypoint=["/usr/bin/bootc-image-builder", "manifest", "{container_ref}"]',
            build_container,
        ], stderr=subprocess.PIPE, encoding="utf8")
    if should_error:
        with pytest.raises(subprocess.CalledProcessError) as exc:
            check_image_ref()
            assert expected_error in exc.value.stderr
    else:
        check_image_ref()


@pytest.mark.parametrize("tc", gen_testcases("target-arch-smoke"))
def test_manifest_target_arch_smoke(build_container, tc):
    # TODO: actually build an image too
    output = subprocess.check_output([
        "podman", "run", "--rm",
        "--privileged",
        "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
        "--security-opt", "label=type:unconfined_t",
        "--entrypoint=/usr/bin/bootc-image-builder",
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
