
# osbuildbootc

## Usage

This tool can be invoked as a pre-built container image, and it can also be installed
as a standalone tool inside another environment.  The implementation uses qemu+KVM.

Example invocation for the container image:

```bash
podman run --rm -ti --security-opt label=disable --device /dev/kvm -v $(pwd):/srv -w /srv ghcr.io/cgwalters/osbuildbootc:latest build-qcow2 -I quay.io/cgwalters/ostest example.qcow2
```

Explanation of podman arguments:

- `--security-opt label=disable`: This is necessary to bind mount in host paths at all
- `--device /dev/kvm`: Pass the KVM device into the container image
- `-v $(pwd):/srv -w /srv`: Pass the current directory as `/srv` into the container

Note that by default KVM is required.  You can set the `OSBUILD_NO_KVM` environment variable
to use full qemu emulation if necessary.

### Take a container image from remote registry, output a qcow2

```bash
osbuildbootc build-qcow2 quay.io/centos-boot/fedora-boot-cloud:eln fedora-boot-cloud.qcow2
```

### Take a container image stored in local OCI directory

In some scenarios it may be desirable to have local disk caches of container images,
instead of fetching from a registry every time.

Note here we need to specify the *target* image after installtion to ensure that
the machine will fetch updates from the registry.

```bash
osbuildbootc build-qcow2 --transport oci oci:cgwalters-ostest -I -t quay.io/cgwalters/ostest foo.qcow2
```

## Development

This project is mostly in Go.  However, it also has some shell script because
some nontrivial code was inherited from [coreos-assembler](https://github.com/coreos/coreos-assembler/).

It's recommended to use e.g. [a toolbox](https://github.com/containers/toolbox/) for development:

```bash
make && sudo make install
```

Then you can run `osbuildbootc`.
