#!/bin/bash

set -euo pipefail

./prepare.sh
/usr/bin/osbuild-deploy-container -store /store -rpmmd /rpmmd -output /output  "$@"
