#!/bin/bash

set -euo pipefail


git clone --branch bifrost-image --depth 1 https://github.com/achilleas-k/images.git
cd images
go build ./cmd/osbuild-deploy-container
