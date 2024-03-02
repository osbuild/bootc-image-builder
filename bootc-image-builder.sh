#!/bin/bash

set -e

if [ "$EUID" -ne 0 ]; then
  echo "Please run as root"
  exit 0
fi

podman run --rm --privileged --pull=newer --security-opt label=type:unconfined_t quay.io/centos-bootc/bootc-image-builder:latest "$@"

