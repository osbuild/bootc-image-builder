# bootc-image-builder

A simpler container for deploying bootable container images.

## Example

```
mkdir output
sudo podman run --rm -it --privileged --security-opt label=type:unconfined_t -v $(pwd)/output:/output ghcr.io/osbuild/bootc-image-builder quay.io/centos-boot/fedora-tier-1:eln

qemu-system-x86_64 -M accel=kvm -cpu host -smp 2 -m 4096 -bios /usr/share/OVMF/OVMF_CODE.fd -snapshot output/qcow2/disk.qcow2
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
sudo podman run --rm -it --privileged --security-opt label=type:unconfined_t -v $(pwd)/output:/output ghcr.io/osbuild/bootc-image-builder quay.io/centos-boot/fedora-tier-1:eln --config /output/config.json
```
