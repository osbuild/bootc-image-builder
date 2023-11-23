#!/bin/bash

set -euo pipefail

# Create a new tmpfs. This solves two issues for us:
# - / is mounted as nosuid, this prevents SELinux to transition to `install_t` because domain transitions are
#   disallowed if they give more caps to the process and the target executable is on `nosuid` filesystem
# - / can be mounted as OverlayFS that doesn't support overlaying SELinux labels. Thus, we need to ensure that
#  the relabeling happens on a mountpoint that's definitely not an OverlayFS.
TMP=/run/suidtmp
mkdir -p "${TMP}"

# The container is mounted as MS_SHARED, this mount as well. Thus, we don't need to care about cleanup, when the
# container dies, it will take this mount with itself.
mount -t tmpfs tmpfs "${TMP}"

# Copy osbuild to the new mountpoint.
cp /usr/bin/osbuild "${TMP}/osbuild"

# Label it as `install_exec_t`. We need this in order to get `install_t` that has `CAP_MAC_ADMIN` for creating SELinux
# labels unknown to the host.
#
# Note that the transition to `install_t` must happen at this point. Osbuild stages run in `bwrap` that creates
# a nosuid, no_new_privs environment. In such an environment, we cannot transition from `unconfined_t` to `install_t`,
# because we would get more privileges.
chcon system_u:object_r:install_exec_t:s0 "${TMP}/osbuild"

# "Copy" back the relabeled osbuild to its right place. We obviously cannot copy it, so let's bind-mount it instead.
# Once again, we don't care about clean-up, this is MS_SHARED.
mount -o bind "${TMP}/osbuild" /usr/bin/osbuild
