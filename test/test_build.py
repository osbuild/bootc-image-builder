import json
import os
import pathlib
import platform
import re
import shutil
import subprocess
import tempfile
import uuid
from contextlib import contextmanager, ExitStack
from typing import NamedTuple
from dataclasses import dataclass

import pytest
# local test utils
import testutil
from containerbuild import build_container_fixture    # pylint: disable=unused-import
from testcases import CLOUD_BOOT_IMAGE_TYPES, DISK_IMAGE_TYPES, gen_testcases
from vm import AWS, QEMU

if not testutil.has_executable("podman"):
    pytest.skip("no podman, skipping integration tests that required podman", allow_module_level=True)

if not testutil.can_start_rootful_containers():
    pytest.skip("tests require to be able to run rootful containers (try: sudo)", allow_module_level=True)

# building an ELN image needs x86_64-v3 to work, we use avx2 as a proxy
# to detect if we have x86-64-v3 (not perfect but should be good enough)
if platform.system() == "Linux" and platform.machine() == "x86_64" and not testutil.has_x86_64_v3_cpu():
    pytest.skip("need x86_64-v3 capable CPU", allow_module_level=True)


class ImageBuildResult(NamedTuple):
    img_type: str
    img_path: str
    img_arch: str
    container_ref: str
    rootfs: str
    disk_config: str
    username: str
    password: str
    ssh_keyfile_private_path: str
    kargs: str
    bib_output: str
    journal_output: str
    metadata: dict = {}


@dataclass
class GPGConf:
    email: str
    key_length: str
    home_dir: str
    pub_key_file: str
    key_params: str


@dataclass
class RegistryConf:
    local_registry: str
    sigstore_dir: str
    registries_d_dir: str
    policy_file: str
    lookaside_conf_file: str
    lookaside_conf: str


@pytest.fixture(name="shared_tmpdir", scope='session')
def shared_tmpdir_fixture(tmpdir_factory):
    tmp_path = pathlib.Path(tmpdir_factory.mktemp("shared"))
    yield tmp_path


@pytest.fixture(name="gpg_conf", scope='session')
def gpg_conf_fixture(shared_tmpdir):
    key_params_tmpl = """
      %no-protection
      Key-Type: RSA
      Key-Length: {key_length}
      Key-Usage: sign
      Name-Real: Bootc Image Builder Tests
      Name-Email: {email}
      Expire-Date: 0
    """
    email = "bib-tests@redhat.com"
    key_length = "3072"
    home_dir = f"{shared_tmpdir}/.gnupg"
    pub_key_file = f"{shared_tmpdir}/GPG-KEY-bib-tests"
    key_params = key_params_tmpl.format(key_length=key_length, email=email)

    os.makedirs(home_dir, mode=0o700, exist_ok=False)
    subprocess.run(
        ["gpg", "--gen-key", "--batch"],
        check=True, env={"GNUPGHOME": home_dir},
        input=key_params,
        text=True)
    subprocess.run(
        ["gpg", "--output", pub_key_file,
         "--armor", "--export", email],
        check=True, env={"GNUPGHOME": home_dir})

    yield GPGConf(email=email, home_dir=home_dir,
                  key_length=key_length, pub_key_file=pub_key_file, key_params=key_params)


@pytest.fixture(name="registry_conf", scope='session')
def registry_conf_fixture(shared_tmpdir, request):
    lookaside_conf_tmpl = """
    docker:
      {local_registry}:
        lookaside: file:///{sigstore_dir}
    """
    registry_port = testutil.get_free_port()
    # We cannot use localhost as we need to access the registry from both
    # the host system and the bootc-image-builder container.
    default_ip = testutil.get_ip_from_default_route()
    local_registry = f"{default_ip}:{registry_port}"
    sigstore_dir = f"{shared_tmpdir}/sigstore"
    registries_d_dir = f"{shared_tmpdir}/registries.d"
    policy_file = f"{shared_tmpdir}/policy.json"
    lookaside_conf_file = f"{registries_d_dir}/lookaside.yaml"
    lookaside_conf = lookaside_conf_tmpl.format(
        local_registry=local_registry,
        sigstore_dir=sigstore_dir
    )
    os.makedirs(registries_d_dir, mode=0o700, exist_ok=True)
    os.makedirs(sigstore_dir, mode=0o700, exist_ok=True)

    registry_container_name = f"registry_{registry_port}"

    registry_container_running = subprocess.run([
        "podman", "ps", "-a", "--filter", f"name={registry_container_name}", "--format", "{{.Names}}"
    ], check=True, capture_output=True, text=True).stdout.strip()
    if registry_container_running != registry_container_name:
        subprocess.run([
            "podman", "run", "-d",
            "-p", f"{registry_port}:5000",
            "--restart", "always",
            "--name", registry_container_name,
            "registry:2"
        ], check=True)

    registry_container_state = subprocess.run([
        "podman", "ps", "-a", "--filter", f"name={registry_container_name}", "--format", "{{.State}}"
    ], check=True, capture_output=True, text=True).stdout.strip()

    if registry_container_state in ("paused", "exited"):
        subprocess.run([
            "podman", "start", registry_container_name
        ], check=True)

    def remove_registry():
        subprocess.run([
            "podman", "rm", "--force", registry_container_name
        ], check=True)

    request.addfinalizer(remove_registry)
    yield RegistryConf(
        local_registry=local_registry,
        sigstore_dir=sigstore_dir,
        registries_d_dir=registries_d_dir,
        policy_file=policy_file,
        lookaside_conf=lookaside_conf,
        lookaside_conf_file=lookaside_conf_file,
    )


def get_signed_container_ref(local_registry: str, container_ref: str):
    container_ref_path = container_ref[container_ref.index('/'):]
    return f"{local_registry}{container_ref_path}"


def sign_container_image(gpg_conf: GPGConf, registry_conf: RegistryConf, container_ref):
    registry_policy = {
        "default": [{"type": "insecureAcceptAnything"}],
        "transports": {
            "docker": {
                f"{registry_conf.local_registry}": [
                    {
                        "type": "signedBy",
                        "keyType": "GPGKeys",
                        "keyPath": f"{gpg_conf.pub_key_file}"
                    }
                ]
            },
            "docker-daemon": {
                "": [{"type": "insecureAcceptAnything"}]
            }
        }
    }
    with open(registry_conf.policy_file, mode="w", encoding="utf-8") as f:
        f.write(json.dumps(registry_policy))

    with open(registry_conf.lookaside_conf_file, mode="w", encoding="utf-8") as f:
        f.write(registry_conf.lookaside_conf)

    signed_container_ref = get_signed_container_ref(registry_conf.local_registry, container_ref)
    cmd = [
        "skopeo", "--registries.d", registry_conf.registries_d_dir,
        "copy", "--dest-tls-verify=false", "--remove-signatures",
        "--sign-by", gpg_conf.email,
        f"docker://{container_ref}",
        f"docker://{signed_container_ref}",
    ]
    subprocess.run(cmd, check=True, env={"GNUPGHOME": gpg_conf.home_dir})


@pytest.fixture(name="image_type", scope="session")
# pylint: disable=too-many-arguments
def image_type_fixture(shared_tmpdir, build_container, request, force_aws_upload, gpg_conf, registry_conf):
    """
    Build an image inside the passed build_container and return an
    ImageBuildResult with the resulting image path and user/password
    In the case an image is being built from a local container, the
    function will build the required local container for the test.
    """
    testutil.pull_container(request.param.container_ref, request.param.target_arch)

    with build_images(shared_tmpdir, build_container,
                      request, force_aws_upload, gpg_conf, registry_conf) as build_results:
        yield build_results[0]


@pytest.fixture(name="images", scope="session")
# pylint: disable=too-many-arguments
def images_fixture(shared_tmpdir, build_container, request, force_aws_upload, gpg_conf, registry_conf):
    """
    Build one or more images inside the passed build_container and return an
    ImageBuildResult array with the resulting image path and user/password
    """
    testutil.pull_container(request.param.container_ref, request.param.target_arch)
    with build_images(shared_tmpdir, build_container,
                      request, force_aws_upload, gpg_conf, registry_conf) as build_results:
        yield build_results


# XXX: refactor
# pylint: disable=too-many-locals,too-many-branches,too-many-statements,too-many-arguments
@contextmanager
def build_images(shared_tmpdir, build_container, request, force_aws_upload, gpg_conf, registry_conf):
    """
    Build all available image types if necessary and return the results for
    the image types that were requested via :request:.

    Will return cached results of previous build requests.

    :request.param: has the form "container_url,img_type1+img_type2,arch,local"
    """
    # the testcases.TestCase comes from the request.parameter
    tc = request.param

    # images might be multiple --type args
    # split and check each one
    image_types = request.param.image.split("+")

    username = "test"
    password = "password"
    kargs = "systemd.journald.forward_to_console=1"

    container_ref = tc.container_ref

    if tc.sign:
        container_ref = get_signed_container_ref(registry_conf.local_registry, tc.container_ref)

    # params can be long and the qmp socket (that has a limit of 100ish
    # AF_UNIX) is derived from the path
    # hash the container_ref+target_arch, but exclude the image_type so that the output path is shared between calls to
    # different image type combinations
    output_path = shared_tmpdir / format(abs(hash(container_ref + str(tc.disk_config) + str(tc.target_arch))), "x")
    output_path.mkdir(exist_ok=True)

    # make sure that the test store exists, because podman refuses to start if the source directory for a volume
    # doesn't exist
    pathlib.Path("/var/tmp/osbuild-test-store").mkdir(exist_ok=True, parents=True)

    journal_log_path = output_path / "journal.log"
    bib_output_path = output_path / "bib-output.log"

    ssh_keyfile_private_path = output_path / "ssh-keyfile"
    ssh_keyfile_public_path = ssh_keyfile_private_path.with_suffix(".pub")

    artifact = {
        "qcow2": pathlib.Path(output_path) / "qcow2/disk.qcow2",
        "ami": pathlib.Path(output_path) / "image/disk.raw",
        "raw": pathlib.Path(output_path) / "image/disk.raw",
        "vmdk": pathlib.Path(output_path) / "vmdk/disk.vmdk",
        "vhd": pathlib.Path(output_path) / "vpc/disk.vhd",
        "gce": pathlib.Path(output_path) / "gce/image.tar.gz",
        "anaconda-iso": pathlib.Path(output_path) / "bootiso/install.iso",
    }
    assert len(artifact) == len(set(tc.image for tc in gen_testcases("all"))), \
        "please keep artifact mapping and supported images in sync"

    # this helper checks the cache
    results = []
    for image_type in image_types:
        # TODO: properly cache amis here. The issue right now is that
        # ami and raw are the same image on disk which means that if a test
        # like "boots_in_aws" requests an ami it will get the raw file on
        # disk. However that is not sufficient because part of the ami test
        # is the upload to AWS and the generated metadata. The fix could be
        # to make the boot-in-aws a new image type like "ami-aws" where we
        # cache the metadata instead of the disk image. Alternatively we
        # could stop testing ami locally at all and just skip any ami tests
        # if there are no AWS credentials.
        if image_type in CLOUD_BOOT_IMAGE_TYPES:
            continue
        generated_img = artifact[image_type]
        print(f"Checking for cached image {image_type} -> {generated_img}")
        if generated_img.exists():
            print(f"NOTE: reusing cached image {generated_img}")
            journal_output = journal_log_path.read_text(encoding="utf8")
            bib_output = bib_output_path.read_text(encoding="utf8")
            results.append(ImageBuildResult(
                image_type, generated_img, tc.target_arch,
                container_ref, tc.rootfs, tc.disk_config,
                username, password,
                ssh_keyfile_private_path, kargs, bib_output, journal_output))

    # generate new keyfile
    if not ssh_keyfile_private_path.exists():
        subprocess.run([
            "ssh-keygen",
            "-N", "",
            # be very conservative with keys for paramiko
            "-b", "2048",
            "-t", "rsa",
            "-f", os.fspath(ssh_keyfile_private_path),
        ], check=True)
    ssh_pubkey = ssh_keyfile_public_path.read_text(encoding="utf8").strip()

    # Because we always build all image types, regardless of what was requested, we should either have 0 results or all
    # should be available, so if we found at least one result but not all of them, this is a problem with our setup
    assert not results or len(results) == len(image_types), \
        f"unexpected number of results found: requested {image_types} but got {results}"

    if results:
        yield results
        return

    print(f"Requested {len(image_types)} images but found {len(results)} cached images. Building...")

    # not all requested image types are available - build them
    cfg = {
        "customizations": {
            "user": [
                {
                    "name": "root",
                    "key": ssh_pubkey,
                    # cannot use default /root as is on a read-only place
                    "home": "/var/roothome",
                }, {
                    "name": username,
                    "password": password,
                    "groups": ["wheel"],
                },
            ],
            "kernel": {
                "append": kargs,
            },
            "files": [
                {
                    "path": "/etc/some-file",
                    "data": "some-data",
                },
            ],
            "directories": [
                {
                    "path": "/etc/some-dir",
                },
            ],
        },
    }
    testutil.maybe_create_filesystem_customizations(cfg, tc)
    testutil.maybe_create_disk_customizations(cfg, tc)
    print(f"config for {output_path} {tc=}: {cfg=}")

    config_json_path = output_path / "config.json"
    config_json_path.write_text(json.dumps(cfg), encoding="utf-8")

    cursor = testutil.journal_cursor()

    upload_args = []
    creds_args = []
    target_arch_args = []
    if tc.target_arch:
        target_arch_args = ["--target-arch", tc.target_arch]

    with tempfile.TemporaryDirectory() as tempdir:
        if "ami" in image_types:
            creds_file = pathlib.Path(tempdir) / "aws.creds"
            if testutil.write_aws_creds(creds_file):
                creds_args = ["-v", f"{creds_file}:/root/.aws/credentials:ro",
                              "--env", "AWS_PROFILE=default"]

                upload_args = [
                    f"--aws-ami-name=bootc-image-builder-test-{str(uuid.uuid4())}",
                    f"--aws-region={testutil.AWS_REGION}",
                    "--aws-bucket=bootc-image-builder-ci",
                ]
            elif force_aws_upload:
                # upload forced but credentials aren't set
                raise RuntimeError("AWS credentials not available (upload forced)")

        # all disk-image types can be generated via a single build
        if image_types[0] in DISK_IMAGE_TYPES:
            types_arg = [f"--type={it}" for it in DISK_IMAGE_TYPES]
        else:
            types_arg = [f"--type={image_types[0]}"]

        # run container to deploy an image into a bootable disk and upload to a cloud service if applicable
        cmd = [
            *testutil.podman_run_common,
            "-v", f"{config_json_path}:/config.json:ro",
            "-v", f"{output_path}:/output",
            "-v", "/var/tmp/osbuild-test-store:/store",  # share the cache between builds
            "-v", "/var/lib/containers/storage:/var/lib/containers/storage",  # mount the host's containers storage
        ]
        if tc.podman_terminal:
            cmd.append("-t")

        if tc.sign:
            sign_container_image(gpg_conf, registry_conf, tc.container_ref)
            signed_image_args = [
                "-v", f"{registry_conf.policy_file}:/etc/containers/policy.json",
                "-v", f"{registry_conf.lookaside_conf_file}:/etc/containers/registries.d/bib-lookaside.yaml",
                "-v", f"{registry_conf.sigstore_dir}:{registry_conf.sigstore_dir}",
                "-v", f"{gpg_conf.pub_key_file}:{gpg_conf.pub_key_file}",
            ]
            cmd.extend(signed_image_args)

            # Pull the signed image
            testutil.pull_container(container_ref, tls_verify=False)

        cmd.extend([
            *creds_args,
            build_container,
            container_ref,
            *types_arg,
            *upload_args,
            *target_arch_args,
            *tc.bib_rootfs_args(),
            f"--use-librepo={tc.use_librepo}",
            *tc.bib_rootfs_args()
        ])

        # print the build command for easier tracing
        print(" ".join(cmd))
        p = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        # not using subprocss.check_output() to ensure we get live output
        # during the text
        bib_output = ""
        while True:
            line = p.stdout.readline()
            if not line:
                break
            print(line, end="")
            bib_output += line
        rc = p.wait(timeout=10)
        assert rc == 0, f"bootc-image-builder failed with return code {rc}"

    journal_output = testutil.journal_after_cursor(cursor)
    metadata = {}
    if "ami" in image_types and upload_args:
        metadata["ami_id"] = parse_ami_id_from_log(journal_output)

        def del_ami():
            testutil.deregister_ami(metadata["ami_id"])
        request.addfinalizer(del_ami)

    journal_log_path.write_text(journal_output, encoding="utf8")
    bib_output_path.write_text(bib_output, encoding="utf8")

    results = []
    for image_type in image_types:
        results.append(ImageBuildResult(
            image_type, artifact[image_type], tc.target_arch,
            container_ref, tc.rootfs, tc.disk_config,
            username, password,
            ssh_keyfile_private_path, kargs, bib_output, journal_output, metadata))
    yield results

    # Try to cache as much as possible
    for image_type in image_types:
        img = artifact[image_type]
        print(f"Checking disk usage for {img}")
        if os.path.exists(img):
            # might already be removed if we're deleting 'raw' and 'ami'
            disk_usage = shutil.disk_usage(img)
            print(f"NOTE: disk usage after {img}: {disk_usage.free / 1_000_000} / {disk_usage.total / 1_000_000}")
            if disk_usage.free < 1_000_000_000:
                print(f"WARNING: running low on disk space, removing {img}")
                img.unlink()
        else:
            print("does not exist")
    subprocess.run(["podman", "rmi", container_ref], check=False)
    return


def test_container_builds(build_container):
    output = subprocess.check_output([
        "podman", "images", "-n", build_container], encoding="utf-8")
    assert build_container in output


@pytest.mark.parametrize("image_type", gen_testcases("multidisk"), indirect=["image_type"])
def test_image_is_generated(image_type):
    assert image_type.img_path.exists(), "output file missing, dir "\
        f"content: {os.listdir(os.fspath(image_type.img_path))}"


def assert_kernel_args(test_vm, image_type):
    exit_status, kcmdline = test_vm.run("cat /proc/cmdline", user=image_type.username, password=image_type.password)
    assert exit_status == 0
    # the kernel arg string must have a space as the prefix and either a space
    # as suffix or be the last element of the kernel commandline
    assert re.search(f" {re.escape(image_type.kargs)}( |$)", kcmdline)


@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("image_type", gen_testcases("qemu-boot"), indirect=["image_type"])
def test_image_boots(image_type):
    with QEMU(image_type.img_path, arch=image_type.img_arch) as test_vm:
        # user/password login works
        exit_status, _ = test_vm.run("true", user=image_type.username, password=image_type.password)
        assert exit_status == 0
        # root/ssh login also works
        exit_status, output = test_vm.run("id", user="root", keyfile=image_type.ssh_keyfile_private_path)
        assert exit_status == 0
        assert "uid=0" in output
        # check generic image options
        assert_kernel_args(test_vm, image_type)
        # ensure bootc points to the right image
        _, output = test_vm.run("bootc status", user="root", keyfile=image_type.ssh_keyfile_private_path)
        # XXX: read the fully yaml instead?
        assert f"image: {image_type.container_ref}" in output

        if image_type.disk_config:
            assert_disk_customizations(image_type, test_vm)
        else:
            assert_fs_customizations(image_type, test_vm)

        # check file/dir customizations
        exit_status, output = test_vm.run("stat /etc/some-file", user=image_type.username)
        assert exit_status == 0
        assert "File: /etc/some-file" in output
        _, output = test_vm.run("stat /etc/some-dir", user=image_type.username)
        assert exit_status == 0
        assert "File: /etc/some-dir" in output


@pytest.mark.parametrize("image_type", gen_testcases("ami-boot"), indirect=["image_type"])
def test_ami_boots_in_aws(image_type, force_aws_upload):
    if not testutil.write_aws_creds("/dev/null"):  # we don't care about the file, just the variables being there
        if force_aws_upload:
            # upload forced but credentials aren't set
            raise RuntimeError("AWS credentials not available")
        pytest.skip("AWS credentials not available (upload not forced)")

    # check that upload progress is in the output log. Uploads looks like:
    # 4.30 GiB / 10.00 GiB [------------>____________] 43.02% 58.04 MiB p/s
    assert "] 100.00%" in image_type.bib_output
    with AWS(image_type.metadata["ami_id"]) as test_vm:
        exit_status, _ = test_vm.run("true", user=image_type.username, password=image_type.password)
        assert exit_status == 0
        exit_status, output = test_vm.run("echo hello", user=image_type.username, password=image_type.password)
        assert exit_status == 0
        assert "hello" in output


def log_has_osbuild_selinux_denials(log):
    osbuid_selinux_denials_re = re.compile(r"(?ms)avc:\ +denied.*osbuild")
    return re.search(osbuid_selinux_denials_re, log)


def parse_ami_id_from_log(log_output):
    ami_id_re = re.compile(r"AMI registered: (?P<ami_id>ami-[a-z0-9]+)\n")
    ami_ids = ami_id_re.findall(log_output)
    assert len(ami_ids) > 0
    return ami_ids[0]


def test_osbuild_selinux_denials_re_works():
    fake_log = (
        'Dec 05 07:19:39 other log msg\n'
        'Dec 05 07:19:39 fedora audit: SELINUX_ERR'
        ' op=security_bounded_transition seresult=denied'
        ' oldcontext=system_u:system_r:install_t:s0:c42,c355'
        ' newcontext=system_u:system_r:mount_t:s0:c42,c355\n'
        'Dec 06 16:00:54 internal audit[14368]: AVC avc:  denied '
        '{ nnp_transition nosuid_transition } for  pid=14368 '
        'comm="org.osbuild.ost" scontext=system_u:system_r:install_t:s0:'
        'c516,c631 tcontext=system_u:system_r:mount_t:s0:c516,c631 '
        'tclass=process2 permissive=0'
    )
    assert log_has_osbuild_selinux_denials(fake_log)
    assert not log_has_osbuild_selinux_denials("some\nrandom\nlogs")


def has_selinux():
    return testutil.has_executable("selinuxenabled") and subprocess.run("selinuxenabled", check=False).returncode == 0


@pytest.mark.skipif(not has_selinux(), reason="selinux not enabled")
@pytest.mark.parametrize("image_type", gen_testcases("qemu-boot"), indirect=["image_type"])
def test_image_build_without_se_linux_denials(image_type):
    # the journal always contains logs from the image building
    assert image_type.journal_output != ""
    assert not log_has_osbuild_selinux_denials(image_type.journal_output), \
        f"denials in log {image_type.journal_output}"


@pytest.mark.skipif(platform.system() != "Linux", reason="boot test only runs on linux right now")
@pytest.mark.parametrize("image_type", gen_testcases("anaconda-iso"), indirect=["image_type"])
def test_iso_installs(image_type):
    installer_iso_path = image_type.img_path
    test_disk_path = installer_iso_path.with_name("test-disk.img")
    with open(test_disk_path, "w", encoding="utf8") as fp:
        fp.truncate(10_1000_1000_1000)
    # install to test disk
    with QEMU(test_disk_path, cdrom=installer_iso_path) as vm:
        vm.start(wait_event="qmp:RESET", snapshot=False, use_ovmf=True)
        vm.force_stop()
    # boot test disk and do extremly simple check
    with QEMU(test_disk_path) as vm:
        vm.start(use_ovmf=True)
        exit_status, _ = vm.run("true", user=image_type.username, password=image_type.password)
        assert exit_status == 0
        assert_kernel_args(vm, image_type)


def osinfo_for(it: ImageBuildResult, arch: str) -> str:
    base = "Media is an installer for OS"
    if it.container_ref.endswith("/centos-bootc/centos-bootc:stream9"):
        return f"{base} 'CentOS Stream 9 ({arch})'\n"
    if it.container_ref.endswith("/centos-bootc/centos-bootc:stream10"):
        # XXX: uncomment once
        # https://gitlab.com/libosinfo/osinfo-db/-/commit/fc811ba5a792967e22a0108de5a245b23da3cc66
        # gets released
        # return f"CentOS Stream 10 ({arch})"
        return ""
    if "/fedora/fedora-bootc:" in it.container_ref:
        ver = it.container_ref.rsplit(":", maxsplit=1)[1]
        return f"{base} 'Fedora Server {ver} ({arch})'\n"
    raise ValueError(f"unknown osinfo string for '{it.container_ref}'")


@pytest.mark.skipif(platform.system() != "Linux", reason="osinfo detect test only runs on linux right now")
@pytest.mark.parametrize("image_type", gen_testcases("anaconda-iso"), indirect=["image_type"])
def test_iso_os_detection(image_type):
    installer_iso_path = image_type.img_path
    arch = image_type.img_arch
    if not arch:
        arch = platform.machine()
    result = subprocess.run([
        "osinfo-detect",
        installer_iso_path,
    ], capture_output=True, text=True, check=True)
    osinfo_output = result.stdout
    expected_output = f"Media is bootable.\n{osinfo_for(image_type, arch)}"
    assert osinfo_output == expected_output


@pytest.mark.skipif(platform.system() != "Linux", reason="osinfo detect test only runs on linux right now")
@pytest.mark.skipif(not testutil.has_executable("unsquashfs"), reason="need unsquashfs")
@pytest.mark.parametrize("image_type", gen_testcases("anaconda-iso"), indirect=["image_type"])
def test_iso_install_img_is_squashfs(tmp_path, image_type):
    installer_iso_path = image_type.img_path
    with ExitStack() as cm:
        mount_point = tmp_path / "cdrom"
        mount_point.mkdir()
        subprocess.check_call(["mount", installer_iso_path, os.fspath(mount_point)])
        cm.callback(subprocess.check_call, ["umount", os.fspath(mount_point)])
        # ensure install.img is the "flat" squashfs, before PR#777 the content
        # was an intermediate ext4 image "squashfs-root/LiveOS/rootfs.img"
        output = subprocess.check_output(["unsquashfs", "-ls", mount_point / "images/install.img"], text=True)
        assert "usr/bin/bootc" in output


@pytest.mark.parametrize("images", gen_testcases("multidisk"), indirect=["images"])
def test_multi_build_request(images):
    artifacts = set()
    expected = {"disk.qcow2", "disk.raw", "disk.vhd", "disk.vmdk", "image.tar.gz"}
    for result in images:
        filename = os.path.basename(result.img_path)
        assert result.img_path.exists()
        artifacts.add(filename)
    assert artifacts == expected


def assert_fs_customizations(image_type, test_vm):
    """
    Asserts that each mountpoint that appears in the build configuration also appears in mountpoint_sizes.

    TODO: assert that the size of each filesystem (or partition) also matches the expected size based on the
    customization.
    """
    # check the minsize specified in the build configuration for each mountpoint against the sizes in the image
    # TODO: replace 'df' call with 'parted --json' and find the partition size for each mountpoint
    exit_status, output = test_vm.run("df --output=target,size", user="root",
                                      keyfile=image_type.ssh_keyfile_private_path)
    assert exit_status == 0
    # parse the output of 'df' to a mountpoint -> size dict for convenience
    mountpoint_sizes = {}
    for line in output.splitlines()[1:]:
        fields = line.split()
        # Note that df output is in 1k blocks, not bytes
        mountpoint_sizes[fields[0]] = int(fields[1]) * 2 ** 10  # in bytes

    cfg = {
        "customizations": {},
    }
    testutil.maybe_create_filesystem_customizations(cfg, image_type)
    for fs in cfg["customizations"]["filesystem"]:
        mountpoint = fs["mountpoint"]
        if mountpoint == "/":
            # / is actually /sysroot
            mountpoint = "/sysroot"
        assert mountpoint in mountpoint_sizes


def assert_disk_customizations(image_type, test_vm):
    exit_status, output = test_vm.run("findmnt --json", user="root",
                                      keyfile=image_type.ssh_keyfile_private_path)
    assert exit_status == 0
    findmnt = json.loads(output)
    exit_status, swapon_output = test_vm.run("swapon --show", user="root",
                                             keyfile=image_type.ssh_keyfile_private_path)
    assert exit_status == 0
    if dc := image_type.disk_config:
        if dc == "lvm":
            mnts = [mnt for mnt in findmnt["filesystems"][0]["children"]
                    if mnt["target"] == "/sysroot"]
            assert len(mnts) == 1
            assert "/dev/mapper/vg00-rootlv" == mnts[0]["source"]
            # check swap too
            assert "7G" in swapon_output
        elif dc == "btrfs":
            mnts = [mnt for mnt in findmnt["filesystems"][0]["children"]
                    if mnt["target"] == "/sysroot"]
            assert len(mnts) == 1
            assert "btrfs" == mnts[0]["fstype"]
            # ensure sysroot comes from the "root" subvolume
            assert mnts[0]["source"].endswith("[/root]")
        elif dc == "swap":
            assert "123M" in swapon_output
