# osbuild-deploy-container

A simpler container for deploying bootable container images.

## Example

x86_64:
```
mkdir output
sudo podman run --rm -it --privileged --security-opt label=type:unconfined_t -v $(pwd)/output:/output ghcr.io/osbuild/osbuild-deploy-container quay.io/centos-boot/fedora-tier-1:eln

qemu-system-x86_64 -M accel=kvm -cpu host -smp 2 -m 4096 -bios /usr/share/OVMF/OVMF_CODE.fd -snapshot output/qcow2/disk.qcow2
```

aarch64:
```
mkdir output
podman run --rm -it --privileged --security-opt label=type:unconfined_t -v $(pwd)/output:/output ghcr.io/osbuild/osbuild-deploy-container -imageref quay.io/centos-bootc/fedora-bootc
qemu-system-aarch64 -M accel=hvf -cpu host -smp 2 -m 4096 -bios /opt/homebrew/Cellar/qemu/8.1.3_2/share/qemu/edk2-aarch64-code.fd -snapshot output/qcow2/disk.qcow2 -serial stdio -machine virt
```

## Volumes
- `/output` - used for output files
- `/store` - used for the osbuild store
- `/rpmmd` - used for the dnf-json rpm metadata cache

## Adding a user
`osbuild-deploy-container` accepts a `-config` option. `-config` needs to be a path to a JSON formatted file.

Example of such a config:

```json
{
  "blueprint": {
    "customizations": {
      "user": [
        {
          "name": "foo",
          "password": "bar",
          "groups": ["wheel"]
        }
      ]
    }
  }
}
```

Save this config as `output/config.json` and run:

```
sudo podman run --rm -it --privileged --security-opt label=type:unconfined_t -v $(pwd)/output:/output ghcr.io/osbuild/osbuild-deploy-container quay.io/centos-boot/fedora-tier-1:eln --config /output/config.json
```
