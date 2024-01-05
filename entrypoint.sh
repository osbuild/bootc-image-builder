#!/bin/bash

set -euo pipefail

./prepare.sh
/usr/bin/bootc-image-builder build --store /store --rpmmd /rpmmd --output /output  "$@"
