#!/bin/bash

set -euo pipefail

# Create a new tmpfs. This solves two issues for us:
# - / can be mounted as overlayfs with all files being `system_u:object_r:container_files_t`
# - / can be mounted as OverlayFS that doesn't support overlaying SELinux labels. Thus, we need to ensure that
#  the relabeling happens on a mountpoint that's definitely not an OverlayFS.
TMP=/run/suidtmp
mkdir -p "${TMP}"

# The container is mounted as MS_SHARED, this mount as well. Thus, we don't need to care about cleanup, when the
# container dies, it will take this mount with itself.
mount -t tmpfs tmpfs "${TMP}"

# Copy osbuild to the new mountpoint.
cp /usr/bin/osbuild "${TMP}/osbuild"
# Also copy setfiles
cp /usr/sbin/setfiles "${TMP}/setfiles"

# All labels inside the container are "wrong" but the only two we care
# about are "osbuild" and "setfiles" so label them "correctly" (as
# they are labeled on a real system).
chcon system_u:object_r:osbuild_exec_t:s0 "${TMP}/osbuild"
chcon system_u:object_r:setfiles_exec_t:s0 "${TMP}/setfiles"

# "Copy" back the relabeled osbuild to its right place. We obviously cannot copy it, so let's bind-mount it instead.
# Once again, we don't care about clean-up, this is MS_SHARED.
mount -o bind "${TMP}/osbuild" /usr/bin/osbuild
mount -o bind "${TMP}/setfiles" /usr/bin/setfiles
