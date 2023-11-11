#!/usr/bin/env bash
set -euo pipefail
# Shared shell script library

DIR=$(dirname "$(realpath "${BASH_SOURCE[0]}")")
RFC3339="%Y-%m-%dT%H:%M:%SZ"

info() {
    echo "info: $*" 1>&2
}

fatal() {
    if test -t 1; then
        echo "$(tput setaf 1)fatal:$(tput sgr0) $*" 1>&2
    else
        echo "fatal: $*" 1>&2
    fi
    exit 1
}

# Execute a command, also writing the cmdline to stdout
runv() {
    echo "Running: " "$@"
    "$@"
}

# Get target base architecture
basearch=$(python3 -c '
import gi
gi.require_version("RpmOstree", "1.0")
from gi.repository import RpmOstree
print(RpmOstree.get_basearch())')
export basearch

# Get target architecture
arch=$(uname -m)
export arch

case $arch in
    "x86_64")  DEFAULT_TERMINAL="ttyS0,115200n8" ;;
    "ppc64le") DEFAULT_TERMINAL="hvc0"           ;;
    "aarch64") DEFAULT_TERMINAL="ttyAMA0"        ;;
    "s390x")   DEFAULT_TERMINAL="ttysclp0"       ;;
    # minimal support; the rest of cosa isn't yet riscv64-aware
    "riscv64") DEFAULT_TERMINAL="ttyS0"          ;;
    *)         fatal "Architecture ${arch} not supported"
esac
export DEFAULT_TERMINAL

COSA_PRIVILEGED=
has_privileges() {
    if [ -z "${COSA_PRIVILEGED:-}" ]; then
        if [ -n "${FORCE_UNPRIVILEGED:-}" ]; then
            info "Detected FORCE_UNPRIVILEGED; using virt"
            COSA_PRIVILEGED=0
        elif ! capsh --print | grep -q 'Bounding.*cap_sys_admin'; then
            info "Missing CAP_SYS_ADMIN; using virt"
            COSA_PRIVILEGED=0
        elif [ "$(id -u)" != "0" ] && ! sudo true; then
            info "Missing sudo privs; using virt"
            COSA_PRIVILEGED=0
        else
            COSA_PRIVILEGED=1
        fi
        export COSA_PRIVILEGED
    fi
    [ ${COSA_PRIVILEGED} == 1 ]
}

preflight() {
    depcheck

    # See https://pagure.io/centos-infra/issue/48
    if test "$(umask)" = 0000; then
        fatal "Your umask is unset, please use umask 0022 or so"
    fi
}

preflight_kvm() {
    # permissions on /dev/kvm vary by (host) distro.  If it's
    # not writable, recreate it.

    if test -z "${COSA_NO_KVM:-}"; then
        if ! test -c /dev/kvm; then
            fatal "Missing /dev/kvm; you can set COSA_NO_KVM=1 to bypass this at the cost of performance."
        fi
        if ! [ -w /dev/kvm ]; then
            if ! has_privileges; then
                fatal "running unprivileged, and /dev/kvm not writable"
            else
                sudo rm -f /dev/kvm
                sudo mknod /dev/kvm c 10 232
                sudo setfacl -m u:"$USER":rw /dev/kvm
            fi
        fi
    fi
}

# runvm generates and runs a minimal VM which we use to "bootstrap" our build
# process.  It mounts the workdir via virtiofs.  If you need to add new packages into
# the vm, update `vmdeps.txt`.
# If you need to debug it, one trick is to change the `-serial file` below
# into `-serial stdio`, drop the <&- and virtio-serial stuff and then e.g. add
# `bash` into the init process.
runvm() {
    local qemu_args=()
    while true; do
        case "$1" in
            --)
                shift
                break
                ;;
            *)
                qemu_args+=("$1")
                shift
                ;;
        esac
    done

    # tmp_builddir is set in prepare_build, but some stages may not
    # know that it exists.
    # shellcheck disable=SC2086
    if [ -z "${tmp_builddir:-}" ]; then
        tmp_builddir="$(mktemp -p ${workdir}/tmp -d supermin.XXXX)"
        export tmp_builddir
        local cleanup_tmpdir=1
    fi

    # shellcheck disable=SC2155
    local vmpreparedir="${tmp_builddir}/supermin.prepare"
    local vmbuilddir="${tmp_builddir}/supermin.build"
    local runvm_console="${tmp_builddir}/runvm-console.txt"
    local rc_file="${tmp_builddir}/rc"

    mkdir -p "${vmpreparedir}" "${vmbuilddir}"

    local rpms
    # then add all the base deps
    # for syntax see: https://github.com/koalaman/shellcheck/wiki/SC2031
    rpms=$(grep -v '^#' < "${DIR}"/vmdeps.txt)

    # shellcheck disable=SC2086
    supermin --prepare --use-installed -o "${vmpreparedir}" $rpms

    # include COSA in the image
    find /usr/lib/bootc2disk/ -type f > "${vmpreparedir}/hostfiles"

    # and include all GPG keys
    find /etc/pki/rpm-gpg/ -type f >> "${vmpreparedir}/hostfiles"

    # the reason we do a heredoc here is so that the var substition takes
    # place immediately instead of having to proxy them through to the VM
    cat > "${vmpreparedir}/init" <<EOF
#!/bin/bash
set -xeuo pipefail
export PATH=/usr/sbin:$PATH
workdir=${workdir}

# use the builder user's id, otherwise some operations like
# chmod will set ownership to root, not builder
export USER=$(id -u)
export RUNVM_NONET=${RUNVM_NONET:-}
$(cat "${DIR}"/supermin-init-prelude.sh)
rc=0
# tee to the virtio port so its output is also part of the supermin output in
# case e.g. a key msg happens in dmesg when the command does a specific operation
if [ -z "${RUNVM_SHELL:-}" ]; then
  bash ${tmp_builddir}/cmd.sh |& tee /dev/virtio-ports/cosa-cmdout || rc=\$?
else
  bash; poweroff -f -f; sleep infinity
fi
echo \$rc > ${rc_file}
if [ -n "\${cachedev}" ]; then
    /sbin/fstrim -v ${workdir}/cache
fi
/sbin/reboot -f
EOF
    chmod a+x "${vmpreparedir}"/init
    (cd "${vmpreparedir}" && tar -czf init.tar.gz --remove-files init)
    # put the supermin output in a separate file since it's noisy
    if ! supermin --build "${vmpreparedir}" --size 10G -f ext2 -o "${vmbuilddir}" \
            &> "${tmp_builddir}/supermin.out"; then
        cat "${tmp_builddir}/supermin.out"
        fatal "Failed to run: supermin --build"
    fi
    superminrootfsuuid=$(blkid --output=value --match-tag=UUID "${vmbuilddir}/root")

    # this is the command run in the supermin container
    # we hardcode a umask of 0022 here to make sure that composes are run
    # with a consistent value, regardless of the environment
    echo "umask 0022" > "${tmp_builddir}"/cmd.sh
    for arg in "$@"; do
        # escape it appropriately so that spaces in args survive
        printf '%q ' "$arg" >> "${tmp_builddir}"/cmd.sh
    done

    touch "${runvm_console}"

    # There seems to be some false positives in shellcheck
    # https://github.com/koalaman/shellcheck/issues/2217
    memory_default=2048
    # shellcheck disable=2031
    case $arch in
    # Power 8 page faults with 2G of memory in rpm-ostree
    # Most probably due to radix and 64k overhead.
    "ppc64le") memory_default=4096 ;;
    esac

    kola_args=(kola qemuexec -m "${COSA_SUPERMIN_MEMORY:-${memory_default}}" --auto-cpus -U --workdir none \
               --console-to-file "${runvm_console}" --bind-rw "${workdir},workdir")

    base_qemu_args=(-drive 'if=none,id=root,format=raw,snapshot=on,file='"${vmbuilddir}"'/root,index=1' \
                    -device 'virtio-blk,drive=root' \
                    -kernel "${vmbuilddir}/kernel" -initrd "${vmbuilddir}/initrd" \
                    -no-reboot -nodefaults \
                    -device virtio-serial \
                    -append "root=UUID=${superminrootfsuuid} console=${DEFAULT_TERMINAL} selinux=1 enforcing=0 autorelabel=1" \
                   )

    if [ -z "${RUNVM_SHELL:-}" ]; then
        if ! "${kola_args[@]}" -- "${base_qemu_args[@]}" \
            -device virtserialport,chardev=virtioserial0,name=cosa-cmdout \
            -chardev stdio,id=virtioserial0 \
            "${qemu_args[@]}" <&-; then # the <&- here closes stdin otherwise qemu waits forever
                cat "${runvm_console}"
                fatal "Failed to run 'kola qemuexec'"
        fi
    else
        exec "${kola_args[@]}" -- "${base_qemu_args[@]}" -serial stdio "${qemu_args[@]}"
    fi

    rm -rf "${tmp_builddir}/supermin.out" "${vmpreparedir}" "${vmbuilddir}"

    if [ ! -f "${rc_file}" ]; then
        cat "${runvm_console}"
        if test -n "${ARTIFACT_DIR:-}"; then
            cp "${runvm_console}" "${ARTIFACT_DIR}"
        fi
        fatal "Couldn't find rc file; failure inside supermin init?"
    fi
    rc="$(cat "${rc_file}")"

    if [ -n "${cleanup_tmpdir:-}" ]; then
        rm -rf "${tmp_builddir}"
        unset tmp_builddir
    fi

    return "${rc}"
}

jq_git() {
    # jq_git extracts JSON elements generated using prepare_git_artifacts.
    # ARG1 is the element name, and ARG2 is the location of the
    # json document.
    jq -rM ".git.$1" "${2}"
}

sha256sum_str() {
    sha256sum | cut -f 1 -d ' '
}
