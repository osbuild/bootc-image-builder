#!/bin/bash
set -euo pipefail
# TODO: This script only exists for legacy reasons, the plan is to start requiring
# a e.g. `build-image` entrypoint.
/usr/bin/bootc-image-builder build "$@"
