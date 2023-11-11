# osbuildbootc

Example usage:

## Take a container image from remote registry, output a qcow2

```bash
osbuildbootc qcow2 quay.io/centos-boot/fedora-boot-cloud:eln fedora-boot-cloud.qcow2
```

## Take a container image stored in local OCI directory

In some scenarios it may be desirable to have local disk caches of container images,
instead of fetching from a registry every time.

Note here we need to specify the *target* image after installtion to ensure that
the machine will fetch updates from the registry.

```bash
osbuildbootc qcow2 --transport oci oci:cgwalters-ostest -I -t quay.io/cgwalters/ostest foo.qcow2
```
