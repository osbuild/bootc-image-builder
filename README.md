# bootc-image-builder

A container to create disk-images from [bootc](https://github.com/containers/bootc) container inputs.

This tools allows to build and deploy disk-images from bootc container
inputs.

## 🔨 Installation

Have [podman](https://podman.io/) installed on your system. Either through your systems package manager if you're on
Linux or through [Podman Desktop](https://podman.io/) if you are on macOS or Windows. If you want to run the resulting
virtual machine(s) or installer media you can use [qemu](https://www.qemu.org/).

On macOS, the podman machine must be running in rootful mode:

```bash
$ podman machine stop   # if already running
Waiting for VM to exit...
Machine "podman-machine-default" stopped successfully
$ podman machine set --rootful
$ podman machine start
```

## 🚀 Examples

The following example builds a [Fedora ELN](https://docs.fedoraproject.org/en-US/eln/) bootable container into a QCOW2 image for the architecture you're running
the command on.

The `fedora-bootc:eln` base image does not include a default user. This example injects a [user configuration file](#-build-config)
by adding a volume-mount for the local file as well as the `--config` flag to the bootc-image-builder container.

The following command will create a QCOW2 disk image. First, create `./config.json` as described above to configure user access.

```bash
sudo podman run \
    --rm \
    -it \
    --privileged \
    --pull=newer \
    --security-opt label=type:unconfined_t \
    -v $(pwd)/config.json:/config.json \
    -v $(pwd)/output:/output \
    quay.io/centos-bootc/bootc-image-builder:latest \
    --type qcow2 \
    --config /config.json \
    quay.io/centos-bootc/fedora-bootc:eln
```

### Running the resulting QCOW2 file on Linux (x86_64)

A virtual machine can be launched using `qemu-system-x86_64` or with `virt-install` as shown below.

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
    --pull=newer \
    --security-opt label=type:unconfined_t \
    -v $(pwd)/output:/output \
    quay.io/centos-bootc/bootc-image-builder:latest \
    <imgref>

Flags:
      --chown string    chown the ouput directory to match the specified UID:GID
      --config string   build config file
      --tls-verify      require HTTPS and verify certificates when contacting registries (default true)
      --type string     image type to build [qcow2, ami] (default "qcow2")
```

### Detailed description of optional flags

| Argument         | Description                                                      | Default Value |
|------------------|------------------------------------------------------------------|:-------------:|
| **--chown**      | chown the ouput directory to match the specified UID:GID         |       ❌      |
| **--config**     | Path to a [build config](#-build-config)                         |       ❌      |
| **--tls-verify** | Require HTTPS and verify certificates when contacting registries |    `true`     |
| **--type**       | [Image type](#-image-types) to build                             |    `qcow2`    |

*💡 Tip: Flags in **bold** are the most important ones.*

## 💾 Image types

The following image types are currently available via the `--type` argument:

| Image type            | Target environment                                                                    |
|-----------------------|---------------------------------------------------------------------------------------|
| `ami`                 | [Amazon Machine Image](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html) |
| `qcow2` **(default)** | [QEMU](https://www.qemu.org/)                                                         |
| `anaconda-iso`        | An unattended Anaconda installer that installs to the first disk found.               |

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
  --pull=newer \
  --security-opt label=type:unconfined_t \
  -v $HOME/.aws:/root/.aws:ro \
  --env AWS_PROFILE=default \
  quay.io/centos-bootc/bootc-image-builder:latest \
  --type ami \
  --aws-ami-name fedora-bootc-ami \
  --aws-bucket fedora-bootc-bucket \
  --aws-region us-east-1 \
  quay.io/centos-bootc/fedora-bootc:eln
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
  --pull=newer \
  --security-opt label=type:unconfined_t \
  --env-file=aws.secrets \
  quay.io/centos-bootc/bootc-image-builder:latest \
  --type ami \
  --aws-ami-name fedora-bootc-ami \
  --aws-bucket fedora-bootc-bucket \
  --aws-region us-east-1 \
  quay.io/centos-bootc/fedora-bootc:eln
```

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

Firstly create a file `./config.json` and put the following content into it:

```json
{
  "blueprint": {
    "customizations": {
      "user": [
        {
          "name": "alice",
          "password": "bob",
          "key": "ssh-rsa AAA ... user@email.com",
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

```bash
sudo podman run \
    --rm \
    -it \
    --privileged \
    --pull=newer \
    --security-opt label=type:unconfined_t \
    -v $(pwd)/config.json:/config.json \
    -v $(pwd)/output:/output \
    quay.io/centos-bootc/bootc-image-builder:latest \
    --type qcow2 \
    --config /config.json \
    quay.io/centos-bootc/fedora-bootc:eln
```

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
```

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
is configured. The base image `quay.io/centos-bootc/fedora-bootc:eln` does not configure passwordless sudo.
This can be configured in a derived bootc container by including the following in a Containerfile.

```dockerfile
FROM quay.io/centos-bootc/fedora-bootc:eln
ADD wheel-passwordless-sudo /etc/sudoers.d/wheel-passwordless-sudo
```

The contents of the file `$(pwd)/wheel-passwordless-sudo` should be

```text
%wheel ALL=(ALL) NOPASSWD: ALL
```

## 📊 Project

- **Website**: <https://www.osbuild.org>
- **Bug Tracker**: <https://github.com/osbuild/bootc-image-builder/issues>
- **Matrix**: #image-builder on [fedoraproject.org](https://matrix.to/#/#image-builder:fedoraproject.org)
- **Mailing List**: <image-builder@redhat.com>
- **Changelog**: <https://github.com/osbuild/bootc-image-builder/releases>

### Contributing

Please refer to the [developer guide](https://www.osbuild.org/docs/developer-guide/index) to learn about our
workflow, code style and more.

## 🗄️ Repository

- **web**:   <https://github.com/osbuild/bootc-image-builder>
- **https**: `https://github.com/osbuild/bootc-image-builder.git`
- **ssh**:   `git@github.com:osbuild/bootc-image-builder.git`

## 🧾 License

- **Apache-2.0**
- See LICENSE file for details.
