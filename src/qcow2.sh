#!/usr/bin/env bash
set -euo pipefail

dn=$(dirname "$0")
# shellcheck source=src/cmdlib.sh
. "${dn}"/cmdlib.sh

# Parse options
disk=$1
shift
config=$1
shift

export workdir="$(pwd)"
mkdir -p ${workdir}/tmp
tmpdir=${workdir}/tmp

qemu_args=("-drive" "if=none,id=target,format=qcow2,file=$disk,cache=unsafe" \
           "-device" "virtio-blk,serial=target,drive=target")
runvm "${qemu_args[@]}" -- /usr/bin/osbuildbootc build-disk-impl ${config}
