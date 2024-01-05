#!/bin/bash
set -euo pipefail
/usr/bin/bootc-image-builder build --store /store --rpmmd /rpmmd --output /output "$@"
