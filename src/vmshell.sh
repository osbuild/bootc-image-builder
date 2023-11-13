#!/usr/bin/env bash
set -eou pipefail

dn=$(dirname "$0")
# shellcheck source=src/cmdlib.sh
. "${dn}"/cmdlib.sh

export workdir="$(pwd)"
RUNVM_SHELL=1 runvm "$@" -- bash
