Integration tests for bootc-image-builder
----------------------------------------------

This directory contains integration tests for bootc-image-builder.

They can be run in two ways
1. On the local machine:  
 By just running `sudo pytest -s -v` in the _top level folder_ of the project (where `Containerfile` is)  
 If you have set up `pip` only for your user, you might just want to run the test with elevated privileges  
 `sudo -E $(which pytest) -s -v`
2. Via `tmt` [0] which will spin up a clean VM and run the tests inside: 

	tmt run -vvv

[0] https://github.com/teemtee/tmt

To install `tmt` on fedora at least those packages are needed:

```shell
sudo dnf install tmt tmt+provision-virtual
```

Ansible container
-----------------
To execute real tests against the VM ansible is used. Ansible is run from within a container.

⚠️ When changing `execution-environment.yml` you need to rebuild and commit `ansible-container/` as described below

```shell
ansible-builder create --context ansible-container
# optionally as this is also done by the test framework
podman build -t bootc-image-builder-ansible-runner ansible-container/
```

To avoid having to install `ansible-builder` (via `pip install ansible-builder`), the resulting folder `ansible-container`
is added to git.

A manual test of the setup (when the Test VM is running) can be:
```shell
podman run --rm -ti --net=host -v ./ansible-playbooks:/runner/ansible-playbooks:Z localhost/bootc-image-builder-ansible-runner ansible-playbook -i localhost, ansible-playbooks/get_distro_version.yml
```