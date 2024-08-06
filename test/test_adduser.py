import subprocess
import textwrap

import pytest

import testutil

if not testutil.has_executable("podman"):
    pytest.skip("no podman, skipping integration tests that required podman", allow_module_level=True)

from containerbuild import build_container_fixture, make_container  # noqa: F401


def test_adduser_ssh_smoke(tmp_path, build_container):
    # no need to parameterize this test, adduser-ssh is the same for all containers
    container_ref = "quay.io/centos-bootc/centos-bootc:stream9"

    # create container that uses our adduser during the build
    cntf_path = tmp_path / "Containerfile"
    cntf_path.write_text(textwrap.dedent(f"""\n
    FROM {build_container} AS toolbox_stage

    FROM {container_ref}
    RUN --mount=type=bind,from=toolbox_stage,target=/toolbox \
      /toolbox/usr/bin/adduser-ssh --ssh-key foo-key foo -- -c "comment for foo"
    RUN --mount=type=bind,from=toolbox_stage,target=/toolbox \
      /toolbox/usr/bin/adduser-ssh --ssh-key root-key --skip-useradd root;
    """), encoding="utf8")

    with make_container(tmp_path) as container_tag:
        print(f"using {container_tag}")
        # assert user got added
        output = subprocess.check_output([
            "podman", "run", "--rm",
            '--entrypoint=["/usr/bin/cat", "/etc/passwd"]',
            container_tag,
        ], encoding="utf8")
        # XXX: is /var/home correct in this context?
        assert "\nfoo:x:1000:1000:comment for foo:/var/home/foo:/bin/bash\n" in output

        output = subprocess.check_output([
            "podman", "run", "--rm",
            '--entrypoint=["/usr/bin/cat", "/root/.ssh/authorized_keys", "/home/foo/.ssh/authorized_keys"]',
            container_tag,
        ], encoding="utf8")
        # assert our key got written
        assert "# created by adduser-ssh on " in output
        assert "# key for root from cmdline\nroot-key\n" in output
        assert "# key for foo from cmdline\nfoo-key" in output
        # XXX: check permissions

        # but it is a bootc one
        output = subprocess.check_output([
            "podman", "run", "--rm",
            '--entrypoint=["/usr/bin/test", "-d", "/ostree"]',
            container_tag,
        ], encoding="utf8")
        assert output == ""
