#!/bin/bash
set -xeuo pipefail

src_container=$1
shift
target_imgref=$1
shift
disk=$(realpath /dev/disk/by-id/virtio-target)

skopeo copy $src_container containers-storage:localhost/image
podman run --net=none --rm --privileged --pid=host --security-opt label=type:unconfined_t localhost/image \
    bootc install --target-imgref $target_imgref --target-no-signature-verification --skip-fetch-check "${disk}"
echo "Done!"
