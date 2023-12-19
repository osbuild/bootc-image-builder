# bootc-image-builder

A container for deploying bootable container images.

## 🔨 Installation

Have [podman](https://podman.io/) installed on your system. Either through your systems package manager if you're on
Linux or through [Podman Desktop](https://podman.io/) if you are on macOS or Windows. If you want to run the resulting
virtual machine(s) or installer media you can use [qemu](https://www.qemu.org/).

On macOS, the podman machine must be running in rootful mode:

```
$ podman machine stop   # if already running
Waiting for VM to exit...
Machine "podman-machine-default" stopped successfully
$ podman machine set --rootful
$ podman machine start
```

## 🚀 Examples

The following example builds a [Fedora ELN](https://docs.fedoraproject.org/en-US/eln/) bootable container into a QCOW2 image for the architecture you're running
the command on.

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

## 📝 Arguments

```
Usage:
  sudo podman run \
    --rm \
    -it \
    --privileged \
    --pull=newer \
    --security-opt label=type:unconfined_t \
    -v $(pwd)/output:/output \
    quay.io/centos-bootc/bootc-image-builder:latest \
    <imgref>

Flags:
      --config string   build config file
      --tls-verify      require HTTPS and verify certificates when contacting registries (default true)
      --type string     image type to build [qcow2, ami] (default "qcow2")
```

### Detailed description of positional flags 

| Argument     | Description                                                      | Default Value |
|--------------|------------------------------------------------------------------|:-------------:|
| **--config** | Path to a [build config](#-build-config)                         |       ❌       |
| --tls-verify | Require HTTPS and verify certificates when contacting registries |    `true`     |
| **--type**   | [Image type](#-image-types) to build                             |    `qcow2`    |

*💡 Tip: Flags in **bold** are the most important ones.* 

## 💾 Image types

The following image types are currently available via the `--type` argument:

| Image type            | Target environment                                                                    |
|-----------------------|---------------------------------------------------------------------------------------|
| `ami`                 | [Amazon Machine Image](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html) |
| `qcow2` **(default)** | [QEMU](https://www.qemu.org/)                                                         |

## 💽 Volumes

The following volumes can be mounted inside the container:

| Volume    | Purpose                                                | Required |
|-----------|--------------------------------------------------------|:--------:|
| `/output` | Used for storing the resulting artifacts               |    ✅     |
| `/store`  | Used for the [osbuild store](https://www.osbuild.org/) |    No    |
| `/rpmmd`  | Used for the DNF cache                                 |    No    |

## 📝 Build config

A build config is a JSON file with customizations for the resulting image. A path to the file is passed via  the `--config` argument. The customizations are specified under a `blueprint.customizations` object.

As an example, let's show how you can add a user to the image:

Firstly create a file `output/config.json` and put the following content into it:

```json
{
  "blueprint": {
    "customizations": {
      "user": [
        {
          "name": "foo",
          "password": "bar",
          "groups": [
            "wheel"
          ]
        }
      ]
    }
  }
}
```

Then, run `bootc-image-builder` with the following arguments:

```
sudo podman run \
    --rm \
    -it \
    --privileged \
    --pull=newer \
    --security-opt label=type:unconfined_t \
    -v $(pwd)/output:/output \
    quay.io/centos-bootc/bootc-image-builder:latest \
    --type qcow2 \
    --config /output/config.json \
    quay.io/centos-bootc/fedora-bootc:eln
```

### Users (`user`, array)

Possible fields:

| Field      | Use                                        | Required |
|------------|--------------------------------------------|:--------:|
| `name`     | Name of the user                           |    ✅     |
| `password` | Unencrypted password                       |    No    |
| `groups`   | An array of secondary to put the user into |    No    |

Example:

```json
{
  "user": [
    {
      "name": "alice",
      "password": "bob",
      "groups": [
        "wheel",
        "admins"
      ]
    }
  ]
}
```

## 📊 Project

* **Website**: <https://www.osbuild.org>
* **Bug Tracker**: <https://github.com/osbuild/bootc-image-builder/issues>
* **Matrix**: #image-builder on [fedoraproject.org](https://matrix.to/#/#image-builder:fedoraproject.org)
* **Mailing List**: image-builder@redhat.com
* **Changelog**: <https://github.com/osbuild/bootc-image-builder/releases>

### Contributing

Please refer to the [developer guide](https://www.osbuild.org/guides/developer-guide/index.html) to learn about our
workflow, code style and more.

## 🗄️ Repository

- **web**:   <https://github.com/osbuild/bootc-image-builder>
- **https**: `https://github.com/osbuild/bootc-image-builder.git`
- **ssh**:   `git@github.com:osbuild/bootc-image-builder.git`

## 🧾 License

- **Apache-2.0**
- See LICENSE file for details.

