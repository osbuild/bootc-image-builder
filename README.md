# osbuild-deploy-container

A simpler container for deploying bootable container images.

## Example

```
mkdir output
sudo podman run --rm -it --privileged -v $(pwd)/output:/output ghcr.io/osbuild/osbuild-deploy-container quay.io/centos-boot/fedora-tier-1:eln

qemu-system-x86_64 -M accel=kvm -cpu host -smp 2 -m 4096 -bios /usr/share/OVMF/OVMF_CODE.fd -snapshot output/qcow2/disk.qcow2
```

## Volumes
- `/output` - used for output files
- `/store` - used for the osbuild store
- `/rpmmd` - used for the dnf-json rpm metadata cache
