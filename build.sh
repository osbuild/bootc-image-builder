#!/bin/bash

set -euo pipefail
# Keep this in sync with e.g. https://github.com/containers/podman/blob/2981262215f563461d449b9841741339f4d9a894/Makefile#L51
# It turns off the esoteric containers-storage backends that add dependencies
# on things like btrfs that we don't need.
CONTAINERS_STORAGE_THIN_TAGS="containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper"

cd bib
set -x
go build -tags "${CONTAINERS_STORAGE_THIN_TAGS}" -o ../bin/bootc-image-builder ./cmd/bootc-image-builder

# expand the list as we support more architectures
for arch in amd64 arm64; do
    if [ "$arch" = "$(go env GOARCH)" ]; then
	continue
    fi

    # what is slightly sad is that this generates a 1MB file. Fedora does
    # not have a cross gcc that can cross build userspace otherwise something
    # like: `void _start() { syscall(SYS_exit() }` would work with
    # `gcc -static -static-libgcc -nostartfiles -nostdlib -l` and give us a 10k
    # cross platform binary. Or maybe no-std rust (thanks Colin)?
    GOARCH="$arch" go build -ldflags="-s -w" -o ../bin/bib-canary-"$arch" ./cmd/cross-arch/
done
