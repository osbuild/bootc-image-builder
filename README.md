# bootc-image-builder

A container for deploying bootable container images.

## Installation

Have [podman](https://podman.io/) installed on your system. Either through your systems package manager if you're on Linux or through [Podman Desktop](https://podman.io/) if you are on Mac OS or Windows. If you want to run the resulting virtual machine(s) or installer media you can use [qemu](https://www.qemu.org/).

On macOS, the podman machine must be running in rootful mode:
```
$ podman machine stop   # if already running
Waiting for VM to exit...
Machine "podman-machine-default" stopped successfully
$ podman machine set --rootful
$ podman machine start
```

## Supported image types

The tool can build the following image types:
- qcow2 (`.qcow2`) for use with QEMU
- ami (`.raw`) for AWS EC2

The output format can be selected with the `--type` option (default `"qcow2"`).

## Examples

The following example builds a [Fedora ELN]() bootable container into a QCOW2 image for the architecture you're running the command on.

```
mkdir output
sudo podman run \
    --rm \
    -it \
    --privileged \
    --pull=newer \
    --security-opt label=type:unconfined_t \
    -v $(pwd)/output:/output \
    quay.io/centos-bootc/bootc-image-builder:latest \
    --type qcow2 \
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

This assumes qemu was installed through [homebrew](https://brew.sh/).

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

