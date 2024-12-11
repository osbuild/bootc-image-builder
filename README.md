# bootc-image-builder

A container to create disk images from [bootc](https://github.com/containers/bootc) container inputs,
especially oriented towards [Fedora/CentOS bootc](https://docs.fedoraproject.org/en-US/bootc/) or
derivatives.

## 🔨 Installation

Have [podman](https://podman.io/) installed on your system. Either through your systems package manager if you're on
Linux or through [Podman Desktop](https://podman.io/) if you are on macOS or Windows. If you want to run the resulting
virtual machine(s) or installer media you can use [qemu](https://www.qemu.org/).

A very nice GUI extension for Podman Desktop is also
[available](https://github.com/containers/podman-desktop-extension-bootc).
The command line examples below can be all handled by
Podman Desktop.

On macOS, the podman machine must be running in rootful mode:

```bash
$ podman machine stop   # if already running
Waiting for VM to exit...
Machine "podman-machine-default" stopped successfully
$ podman machine set --rootful
$ podman machine start
```

## ✍ Prerequisites

The package `osbuild-selinux` or equivalent osbuild SELinux policies must be installed in the system running
`bootc-image-builder`.

## 🚀 Examples

The following example builds a `centos-bootc:stream9` bootable container into a QCOW2 image for the architecture you're running
the command on.  However, be sure to see the [upstream documentation](https://docs.fedoraproject.org/en-US/bootc/)
for more general information!  Note that outside of initial experimentation, it's recommended to build a *derived* container image
(or reuse a derived image built via someone else) and then use this project to make a disk image from your custom image.

The generic base images do not include a default user. This example injects a [user configuration file](#-build-config)
by adding a volume-mount for the local file to the bootc-image-builder container.

The following command will create a QCOW2 disk image. First, create `./config.toml` as described above to configure user access.

```bash
# Ensure the image is fetched
sudo podman pull quay.io/centos-bootc/centos-bootc:stream9
sudo podman run \
    --rm \
    -it \
    --privileged \
    --log-driver=passthrough-tty \
    --pull=newer \
    --security-opt label=type:unconfined_t \
    -v ./config.toml:/config.toml:ro \
    -v ./output:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder:latest \
    --type qcow2 \
    --local \
    quay.io/centos-bootc/centos-bootc:stream9
```

Note that some images (like fedora) do not have a default root
filesystem type. In this case adds the switch `--rootfs <type>`,
e.g. `--rootfs btrfs`.

### Running the resulting QCOW2 file on Linux (x86_64)

A virtual machine can be launched using `qemu-system-x86_64` or with `virt-install` as shown below;
however there is more information about virtualization and other
choices in the [Fedora/CentOS bootc documentation](https://docs.fedoraproject.org/en-US/bootc/).

#### qemu-system-x86_64

```bash
qemu-system-x86_64 \
    -M accel=kvm \
    -cpu host \
    -smp 2 \
    -m 4096 \
    -bios /usr/share/OVMF/OVMF_CODE.fd \
    -serial stdio \
    -snapshot output/qcow2/disk.qcow2
```

#### virt-install

```bash
sudo virt-install \
    --name fedora-bootc \
    --cpu host \
    --vcpus 4 \
    --memory 4096 \
    --import --disk ./output/qcow2/disk.qcow2,format=qcow2 \
    --os-variant fedora-eln
```

### Running the resulting QCOW2 file on macOS (aarch64)

This assumes qemu was installed through [homebrew](https://brew.sh/).

```bash
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

```bash
Usage:
  sudo podman run \
    --rm \
    -it \
    --privileged \
    --log-driver=passthrough-tty \
    --pull=newer \
    --security-opt label=type:unconfined_t \
    -v ./output:/output \
    quay.io/centos-bootc/bootc-image-builder:latest \
    <imgref>

Flags:
      --chown string           chown the ouput directory to match the specified UID:GID
      --tls-verify             require HTTPS and verify certificates when contacting registries (default true)
      --type string            image type to build [qcow2, ami] (default "qcow2")
      --target-arch string     architecture to build image for (default is the native architecture)
```

### Detailed description of optional flags

| Argument          | Description                                                                                               | Default Value |
|-------------------|-----------------------------------------------------------------------------------------------------------|:-------------:|
| **--chown**       | chown the output directory to match the specified UID:GID                                                 |       ❌      |
| **--rootfs**      | Root filesystem type. Overrides the default from the source container. Supported values: ext4, xfs, btrfs |       ❌      |
| **--tls-verify**  | Require HTTPS and verify certificates when contacting registries                                          |    `true`     |
| **--type**        | [Image type](#-image-types) to build                                                                      |    `qcow2`    |
| **--target-arch** | [Target arch](#-target-architecture) to build                                                             |       ❌      |

The `--type` parameter can be given multiple times and multiple outputs will
be produced.

*💡 Tip: Flags in **bold** are the most important ones.*

## 💾 Image types

The following image types are currently available via the `--type` argument:

| Image type            | Target environment                                                                    |
|-----------------------|---------------------------------------------------------------------------------------|
| `ami`                 | [Amazon Machine Image](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html) |
| `qcow2` **(default)** | [QEMU](https://www.qemu.org/)                                                         |
| `vmdk`                | [VMDK](https://en.wikipedia.org/wiki/VMDK) usable in vSphere, among others            |
| `anaconda-iso`        | An unattended Anaconda installer that installs to the first disk found.               |
| `raw`                 | Unformatted [raw disk](https://en.wikipedia.org/wiki/Rawdisk).                        |
| `vhd`                 | [vhd](https://en.wikipedia.org/wiki/VHD_(file_format)) usable in Virtual PC, among others |
| `gce`                 | [GCE](https://cloud.google.com/compute/docs/images#custom_images) |

## 💾 Target architecture

Specify the target architecture of the system on which the disk image will be installed on. By default,
`bootc-image-builder` will build for the native host architecture. The target architecture
must match an available architecture of the `bootc-image-builder` image you are using to build the disk image.
Currently, `amd64` and `arm64` are included in `quay.io/centos-bootc/bootc-image-builder` manifest list.
The architecture of the bootc OCI image and the bootc-image-builder image must match. For example, when building
a non-native architecture bootc OCI image, say, building for x86_64 from an arm-based Mac, it is possible to run
`podman build` with the `--platform linux/amd64` flag. In this case, to then build a disk image from the same arm-based Mac,
you should provide `--target-arch amd64` when running the `bootc-image-builder` command.

## ☁️ Cloud uploaders

### Amazon Machine Images (AMIs)

#### Prerequisites

In order to successfully import an AMI into your AWS account, you need to have the [vmimport service role](https://docs.aws.amazon.com/vm-import/latest/userguide/required-permissions.html) configured on your account.

#### Flags

AMIs can be automatically uploaded to AWS by specifying the following flags:

| Argument       | Description                                                      |
|----------------|------------------------------------------------------------------|
| --aws-ami-name | Name for the AMI in AWS                                          |
| --aws-bucket   | Target S3 bucket name for intermediate storage when creating AMI |
| --aws-region   | Target region for AWS uploads                                    |

*Notes:*

- *These flags must all be specified together. If none are specified, the AMI is exported to the output directory.*
- *The bucket must already exist in the selected region, bootc-image-builder will not create it if it is missing.*
- *The output volume is not needed in this case. The image is uploaded to AWS and not exported.*

#### AWS credentials file

If you already have a credentials file (usually in `$HOME/.aws/credentials`) you need to forward the
directory to the container

For example:

```bash
 $ sudo podman run \
  --rm \
  -it \
  --privileged \
  --log-driver=passthrough-tty \
  --pull=newer \
  --security-opt label=type:unconfined_t \
  -v $HOME/.aws:/root/.aws:ro \
  --env AWS_PROFILE=default \
  quay.io/centos-bootc/bootc-image-builder:latest \
  --type ami \
  --aws-ami-name centos-bootc-ami \
  --aws-bucket fedora-bootc-bucket \
  --aws-region us-east-1 \
  quay.io/centos-bootc/centos-bootc:stream9
```

Notes:

- *you can also inject **ALL** your AWS configuration parameters with `--env AWS_*`*

see the [AWS CLI documentation](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html) for more information about other environment variables

#### AWS credentials via environment

AWS credentials can be specified through two environment variables:
| Variable name         | Description                                                                                                         |
|-----------------------|---------------------------------------------------------------------------------------------------------------------|
| AWS_ACCESS_KEY_ID     | AWS access key associated with an IAM account.                                                                      |
| AWS_SECRET_ACCESS_KEY | Specifies the secret key associated with the access key. This is essentially the "password" for the access key.     |

Those **should not** be specified with `--env` as plain value, but you can silently hand them over with `--env AWS_*` or
save these variables in a file and pass them using the `--env-file` flag for `podman run`.

For example:

```bash
$ cat aws.secrets
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

$ sudo podman run \
  --rm \
  -it \
  --privileged \
  --log-driver=passthrough-tty \
  --pull=newer \
  --security-opt label=type:unconfined_t \
  --env-file=aws.secrets \
  quay.io/centos-bootc/bootc-image-builder:latest \
  --type ami \
  --aws-ami-name centos-bootc-ami \
  --aws-bucket centos-bootc-bucket \
  --aws-region us-east-1 \
  quay.io/centos-bootc/centos-bootc:stream9
```

## 💽 Volumes

The following volumes can be mounted inside the container:

| Volume    | Purpose                                                | Required |
|-----------|--------------------------------------------------------|:--------:|
| `/output` | Used for storing the resulting artifacts               |    ✅     |
| `/store`  | Used for the [osbuild store](https://www.osbuild.org/) |    No    |
| `/rpmmd`  | Used for the DNF cache                                 |    No    |

## 📝 Build config

A build config is a Toml (or JSON) file with customizations for the resulting image. The config file is mapped into the container directory to `/config.toml`. The customizations are specified under a `customizations` object.

As an example, let's show how you can add a user to the image:

Firstly create a file `./config.toml` and put the following content into it:

```toml
[[customizations.user]]
name = "alice"
password = "bob"
key = "ssh-rsa AAA ... user@email.com"
groups = ["wheel"]
```

Then, run `bootc-image-builder` with the following arguments:

```bash
sudo podman run \
    --rm \
    -it \
    --privileged \
    --log-driver=passthrough-tty \
    --pull=newer \
    --security-opt label=type:unconfined_t \
    -v ./config.toml:/config.toml:ro \
    -v ./output:/output \
    quay.io/centos-bootc/bootc-image-builder:latest \
    --type qcow2 \
    quay.io/centos-bootc/centos-bootc:stream9
```

The configuration can also be passed in via stdin when `--config -`
is used. Only JSON configuration is supported in this mode.

### Users (`user`, array)

Possible fields:

| Field      | Use                                        | Required |
|------------|--------------------------------------------|:--------:|
| `name`     | Name of the user                           |    ✅    |
| `password` | Unencrypted password                       |    No    |
| `key`      | Public SSH key contents                    |    No    |
| `groups`   | An array of secondary to put the user into |    No    |

Example:

```json
{
  "customizations": {
    "user": [
      {
        "name": "alice",
        "password": "bob",
        "key": "ssh-rsa AAA ... user@email.com",
        "groups": [
          "wheel",
          "admins"
        ]
      }
    ]
  }
}
```

### Kernel Arguments (`kernel`, mapping)

```json
{
  "customizations": {
    "kernel": {
      "append": "mitigations=auto,nosmt"
    }
  }
}

```

### Filesystems (`filesystem`, array)

The filesystem section of the customizations can be used to set the minimum size of the base partitions (`/` and `/boot`) as well as to create extra partitions with mountpoints under `/var`.

```toml
[[customizations.filesystem]]
mountpoint = "/"
minsize = "10 GiB"

[[customizations.filesystem]]
mountpoint = "/var/data"
minsize = "20 GiB"
```

```json
{
  "customizations": {
    "filesystem": [
      {
        "mountpoint": "/",
        "minsize": "10 GiB"
      },
      {
        "mountpoint": "/var/data",
        "minsize": "20 GiB"
      }
    ]
  }
}
```

#### Interaction with `rootfs`

#### Filesystem types

The `--rootfs` option also sets the filesystem types for all additional mountpoints, where appropriate. See the see [Detailed description of optional flags](#detailed-description-of-optional-flags).


#### Allowed mountpoints and sizes

The following restrictions and rules apply, unless the rootfs is `btrfs`:
- `/` can be specified to set the desired (minimum) size of the root filesystem. The final size of the filesystem, mounted at `/sysroot` on a booted system, is the value specified in this configuration or 2x the size of the base container, whichever is largest.
- `/boot`can be specified to set the desired size of the boot partition.
- Subdirectories of `/var` are supported, but symlinks in `/var` are not. For example, `/var/home` and `/var/run` are symlinks and cannot be filesystems on their own.
- `/var` itself cannot be a mountpoint.

The `rootfs` option (or source container config, see [Detailed description of optional flags](#detailed-description-of-optional-flags) section) defines the filesystem type for the root filesystem. Currently, creation of btrfs subvolumes at build time is not supported. Therefore, if the `rootfs` is `btrfs`, no custom mountpoints are supported under `/var`.  Only `/` and `/boot` can be configured.


### Anaconda ISO (installer) options (`installer`, mapping)

Users can include kickstart file content that will be added to an ISO build to configure the installation process.
Since multi-line strings are difficult to write and read in json, it's easier to use the toml format when adding kickstart contents:

```toml
[customizations.installer.kickstart]
contents = """
text --non-interactive
zerombr
clearpart --all --initlabel --disklabel=gpt
autopart --noswap --type=lvm
network --bootproto=dhcp --device=link --activate --onboot=on
"""
```

The equivalent in json would be:
```json
{
  "customizations": {
    "installer": {
      "kickstart": {
        "contents": "text --non-interactive\nzerombr\nclearpart --all --initlabel --disklabel=gpt\nautopart --noswap --type=lvm\nnetwork --bootproto=dhcp --device=link --activate --onboot=on"
      }
    }
  }
}
```

Note that bootc-image-builder will automatically add the command that installs the container image (`ostreecontainer ...`), so this line or any line that conflicts with it should not be included. See the relevant [Kickstart documentation](https://pykickstart.readthedocs.io/en/latest/kickstart-docs.html#ostreecontainer) for more information.
No other kickstart commands are added by bootc-image-builder in this case, so it is the responsibility of the user to provide all other commands (for example, for partitioning, network, language, etc).

#### Anaconda ISO (installer) Modules

The Anaconda installer can be configured by enabling or disabling its dbus modules.

```toml
[customizations.installer.modules]
enable = [
  "org.fedoraproject.Anaconda.Modules.Localization"
]
disable = [
  "org.fedoraproject.Anaconda.Modules.Users"
]
```

```json
{
  "customizations": {
    "installer": {
      "modules": {
        "enable": [
          "org.fedoraproject.Anaconda.Modules.Localization"
        ],
        "disable": [
          "org.fedoraproject.Anaconda.Modules.Users"
        ]
      }
    }
  }
}
```

The following module names are known and supported:
- `org.fedoraproject.Anaconda.Modules.Localization`
- `org.fedoraproject.Anaconda.Modules.Network`
- `org.fedoraproject.Anaconda.Modules.Payloads`
- `org.fedoraproject.Anaconda.Modules.Runtime`
- `org.fedoraproject.Anaconda.Modules.Security`
- `org.fedoraproject.Anaconda.Modules.Services`
- `org.fedoraproject.Anaconda.Modules.Storage`
- `org.fedoraproject.Anaconda.Modules.Subscription`
- `org.fedoraproject.Anaconda.Modules.Timezone`
- `org.fedoraproject.Anaconda.Modules.Users`

*Note: The values are not validated. Any name listed under `enable` will be added to the Anaconda configuration. This way, new or unknown modules can be enabled. However, it also means that mistyped or incorrect values may cause Anaconda to fail to start.*

By default, the following modules are enabled for all Anaconda ISOs:
- `org.fedoraproject.Anaconda.Modules.Network`
- `org.fedoraproject.Anaconda.Modules.Payloads`
- `org.fedoraproject.Anaconda.Modules.Security`
- `org.fedoraproject.Anaconda.Modules.Services`
- `org.fedoraproject.Anaconda.Modules.Storage`
- `org.fedoraproject.Anaconda.Modules.Users`


##### Enable vs Disable priority

The `disable` list is processed after the `enable` list and therefore takes priority. In other words, adding the same module in both `enable` and `disable` will result in the module being **disabled**.
Furthermore, adding a module that is enabled by default to `disable` will result in the module being **disabled**.


## Building

To build the container locally you can run

```shell
sudo podman build --tag bootc-image-builder .
```

NOTE: running already the `podman build` as root avoids problems later as we need to run the building
of the image as root anyway

### Accessing the system

With a virtual machine launched with the above [virt-install](#virt-install) example, access the system with

```shell
ssh -i /path/to/private/ssh-key alice@ip-address
```

Note that if you do not provide a password for the provided user, `sudo` will not work unless passwordless sudo
is configured. The base image `quay.io/centos-bootc/centos-bootc:stream9` does not configure passwordless sudo.
This can be configured in a derived bootc container by including the following in a Containerfile.

```dockerfile
FROM quay.io/centos-bootc/centos-bootc:stream9
ADD wheel-passwordless-sudo /etc/sudoers.d/wheel-passwordless-sudo
```

The contents of the file `wheel-passwordless-sudo` should be

```text
%wheel ALL=(ALL) NOPASSWD: ALL
```

## Reporting bugs

Please report bugs to the [Bug Tracker](https://github.com/osbuild/bootc-image-builder/issues) and include instructions to reproduce and the output of:
```
$ sudo podman run --rm -it quay.io/centos-bootc/bootc-image-builder:latest version
```

### Contributing

Please refer to the [developer guide](https://www.osbuild.org/docs/developer-guide/index) to learn about our
workflow, code style and more.

## 🗄️ Repository

- **web**:   <https://github.com/osbuild/bootc-image-builder>
- **https**: `https://github.com/osbuild/bootc-image-builder.git`
- **ssh**:   `git@github.com:osbuild/bootc-image-builder.git`

## 📊 Project

- **Website**: <https://www.osbuild.org>
- **Bug Tracker**: <https://github.com/osbuild/bootc-image-builder/issues>
- **Matrix**: #image-builder on [fedoraproject.org](https://matrix.to/#/#image-builder:fedoraproject.org)
* **Discussions**: https://github.com/orgs/osbuild/discussions

## 🧾 License

- **Apache-2.0**
- See LICENSE file for details.
