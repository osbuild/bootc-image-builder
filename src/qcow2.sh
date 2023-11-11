#!/usr/bin/env bash
set -euo pipefail

dn=$(dirname "$0")
# shellcheck source=src/cmdlib.sh
. "${dn}"/cmdlib.sh

# Parse options
src_imgref=$1
shift
target_imgref=$1
shift
disk=$1
shift
diskname=$(basename $disk)
set -x

export workdir="$(pwd)"
mkdir -p ${workdir}/tmp
tmpdir=${workdir}/tmp

disk_args=()
qemu_args=()

image_size="8000M"
echo "Disk size estimated to ${image_size}"

qemu-img create -f qcow2 "$tmpdir/${diskname}" "${image_size}"

qemu_args+=("-drive" "if=none,id=target,format=qcow2,file=$tmpdir/${diskname},cache=unsafe" \
              "-device" "virtio-blk,serial=target,drive=target")

runvm "${qemu_args[@]}" -- /usr/lib/bootc2disk/create_disk.sh ${src_imgref} "${target_imgref}" "${disk_args[@]}"

mv -Tf "$tmpdir/${diskname}" "${disk}"
echo "Successfully generated: ${disk}"
