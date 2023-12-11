# bootc-image-builder

A container for deploying bootable container images.

## Installation

Have [podman](https://podman.io/) installed on your system. Either through your systems package manager if you're on Linux or through [Podman Desktop](https://podman.io/) if you are on Mac OS or Windows.

## Examples

The following example builds a [Fedora ELN]() bootable container into a QCOW2 image for the architecture you're running the command on.

```
mkdir output
sudo podman run \
    --rm \
    -it
    --privileged \
    --pull=newer \
    --security-opt label=type:unconfined_t \
    -v $(pwd)/output:/output \
    quay.io/centos-bootc/bootc-image-builder:latest \
    quay.io/centos-bootc/fedora-bootc:eln
```

### Running the resulting QCOW2 file on Linux (x86_64)
```
qemu-system-x86_64 \
    -M accel=kvm \
    -cpu host \
    -smp 2 \
    -m 4096 \
    -bios /usr/share/OVMF/OVMF_CODE.fd \
    -serial stdio \
    -snapshot output/qcow2/disk.qcow2
```

### Running the resulting QCOW2 file on macOS (aarch64)
```
qemu-system-aarch64 \
    -M accel=hvf \
    -cpu host \
    -smp 2 \
    -m 4096 \
    -bios /opt/homebrew/Cellar/qemu/8.1.3_2/share/qemu/edk2-aarch64-code.fd \
    -serial stdio \
    -machine virt \
    -snapshot output/qcow2/disk.qcow2
```

## Volumes
- `/output` - used for output files
- `/store` - used for the osbuild store
- `/rpmmd` - used for the dnf-json rpm metadata cache

## Adding a user
`bootc-image-builder` accepts a `--config` option. `--config` needs to be a path to a JSON formatted file.

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

## Project

 * **Website**: <https://www.osbuild.org>
 * **Bug Tracker**: <https://github.com/osbuild/bootc-image-builder/issues>
 * **Matrix**: #image-builder on [fedoraproject.org](https://matrix.to/#/#image-builder:fedoraproject.org)
 * **Mailing List**: image-builder@redhat.com
 * **Changelog**: <https://github.com/osbuild/bootc-image-builder/releases>

### Contributing

Please refer to the [developer guide](https://www.osbuild.org/guides/developer-guide/index.html) to learn about our workflow, code style and more.

## Repository

 - **web**:   <https://github.com/osbuild/bootc-image-builder>
 - **https**: `https://github.com/osbuild/bootc-image-builder.git`
 - **ssh**:   `git@github.com:osbuild/bootc-image-builder.git`

## License

 - **Apache-2.0**
 - See LICENSE file for details.

