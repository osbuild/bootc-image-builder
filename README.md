# osbuildbootc

## Usage

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
