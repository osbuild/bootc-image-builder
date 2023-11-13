#!/bin/bash

set -euo pipefail


cd images
go build ./cmd/osbuild-deploy-container
