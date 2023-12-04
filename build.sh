#!/bin/bash

set -euo pipefail

cd odc
go build -o ../bin/bootc-image-builder ./cmd/bootc-image-builder
