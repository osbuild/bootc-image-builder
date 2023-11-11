#!/bin/bash
set -xeuo pipefail

src_container=$1
shift
target_imgref=$1
shift
disk=$(realpath /dev/disk/by-id/virtio-target)

echo $PATH
echo $$
skopeo copy $src_container containers-storage:localhost/image
ls -al /usr/bin/skopeo
podman run --net=none --rm --privileged --pid=host localhost/image env
podman run --net=none --rm --privileged --pid=host localhost/image ls -al /proc/1/root/usr/bin/skopeo
podman run --env RUST_LOG=trace --net=none --rm --privileged --pid=host --security-opt label=type:unconfined_t localhost/image \
    bootc install --target-imgref $target_imgref --target-no-signature-verification --skip-fetch-check "${disk}"
