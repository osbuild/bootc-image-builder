#!/bin/bash

set -euo pipefail

cd odc
go build -o ../bin/osbuild-deploy-container ./cmd/osbuild-deploy-container
