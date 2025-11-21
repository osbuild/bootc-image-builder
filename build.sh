#!/bin/bash

set -euo pipefail
# Keep this in sync with e.g. https://github.com/containers/podman/blob/2981262215f563461d449b9841741339f4d9a894/Makefile#L51
# It turns off the esoteric containers-storage backends that add dependencies
# on things like btrfs that we don't need.
CONTAINERS_STORAGE_THIN_TAGS="containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper"

BINDIR="$(pwd)/bin"
cd bib
set -x
# XXX: replace with
# GOBIN=../bin go install -tags "${CONTAINERS_STORAGE_THIN_TAGS}" github.com/osbuild/image-builder-cli/cmd/image-builder@<rev>
# once its merge
# silly workaround
TMPDIR=$(mktemp -d)
  cd "$TMPDIR"
  go mod init dummy/installer
  go mod edit -replace github.com/osbuild/image-builder-cli=github.com/mvo5/image-builder-cli@merge-bib-multicall
  go mod tidy
  go get github.com/osbuild/image-builder-cli/cmd/image-builder
  GOBIN="$BINDIR" go install -tags "${CONTAINERS_STORAGE_THIN_TAGS}"  github.com/osbuild/image-builder-cli/cmd/image-builder
  mv "$BINDIR"/image-builder "$BINDIR"/bootc-image-builder
  cd -
# end silly workaround

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
