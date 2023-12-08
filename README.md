# bootc-image-builder

A simpler container for deploying bootable container images.

## Example

```
mkdir output
sudo podman run --rm -it --privileged --pull=newer --security-opt label=type:unconfined_t -v $(pwd)/output:/output quay.io/centos-bootc/bootc-image-builder:latest quay.io/centos-bootc/fedora-bootc:eln
```

amd64:
```
qemu-system-x86_64 -M accel=kvm -cpu host -smp 2 -m 4096 -bios /usr/share/OVMF/OVMF_CODE.fd -snapshot output/qcow2/disk.qcow2
```

aarch64:
```
qemu-system-aarch64 -M accel=hvf -cpu host -smp 2 -m 4096 -bios /opt/homebrew/Cellar/qemu/8.1.3_2/share/qemu/edk2-aarch64-code.fd -snapshot output/qcow2/disk.qcow2 -serial stdio -machine virt
```

## Volumes
- `/output` - used for output files
- `/store` - used for the osbuild store
- `/rpmmd` - used for the dnf-json rpm metadata cache

## Adding a user
`bootc-image-builder` accepts a `-config` option. `-config` needs to be a path to a JSON formatted file.

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
sudo podman run --rm -it --privileged --pull=newer --security-opt label=type:unconfined_t -v $(pwd)/output:/output quay.io/centos-bootc/bootc-image-builder:latest quay.io/centos-bootc/fedora-bootc:eln --config /output/config.json
```
