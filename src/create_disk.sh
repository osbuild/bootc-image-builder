#!/bin/bash
set -xeuo pipefail

src_container=$1
shift

skopeo copy $src_container containers-storage:localhost/image
podman run --net=none --rm --privileged --pid=host --security-opt  localhost/image \
    bootc install "$@"
echo "Done!"
