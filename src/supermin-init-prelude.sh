#!/usr/bin/env bash
# This script is our half-baked ad-hoc reimplementation of a minimal
# Linux boot environment that we generate via `supermin`.  At some
# point we will likely switch to using systemd.

set -x

mount -t proc /proc /proc
mount -t sysfs /sys /sys
mount -t cgroup2 cgroup2 -o rw,nosuid,nodev,noexec,relatime,seclabel,nsdelegate,memory_recursiveprot /sys/fs/cgroup
mount -t devtmpfs devtmpfs /dev

# need /dev/shm for podman
mkdir -p /dev/shm
mount -t tmpfs tmpfs /dev/shm

# load selinux policy
LANG=C /sbin/load_policy  -i


# need fuse module for rofiles-fuse/bwrap during post scripts run
/sbin/modprobe fuse

# we want /dev/disk symlinks for coreos-installer
/usr/lib/systemd/systemd-udevd --daemon
# We've seen this hang before, so add a timeout. This is best-effort anyway, so
# let's not fail on it.
timeout 30s /usr/sbin/udevadm trigger --settle || :

# set up networking
if [ -z "${RUNVM_NONET:-}" ]; then
    /usr/sbin/dhclient eth0
fi

# set the umask so that anyone in the group can rwx
umask 002

# set up workdir
# https://github.com/coreos/coreos-assembler/issues/2171
mkdir -p "${workdir:?}"
mount -t virtiofs -o rw workdir "${workdir}"

if [ -f "${workdir}/tmp/supermin/supermin.env" ]; then
    source "${workdir}/tmp/supermin/supermin.env";
fi

# Code can check this to see whether it's being run via supermin; in
# some cases it may be convenient to have files wrap themselves, and
# this disambiguates between container and supermin.
touch /etc/cosa-supermin

# https://github.com/koalaman/shellcheck/wiki/SC2164
cd "${workdir}" || exit
