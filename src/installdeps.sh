#!/bin/bash
set -xeuo pipefail
cd $(dirname "$0")
./print-dependencies.sh | xargs dnf -y install
