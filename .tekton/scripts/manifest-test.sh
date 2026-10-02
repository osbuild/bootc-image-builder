#!/usr/bin/env bash
set -euo pipefail

# Expects:
#   RESULT_FILE — path to write the Konflux TEST_OUTPUT result JSON
#
# Runs inside the bootc-image-builder container image under test.
# Generates a qcow2 manifest from a public bootc image and validates
# the output is well-formed JSON.

BOOTC_SOURCE="quay.io/centos-bootc/centos-bootc:stream10"

fail() {
    printf '{"result":"FAILURE","successes":0,"failures":1,"warnings":0,"note":"%s"}' "$1" > "$RESULT_FILE"
    exit 1
}

echo "=== Generating qcow2 manifest from $BOOTC_SOURCE ==="

if ! bootc-image-builder manifest --type qcow2 "$BOOTC_SOURCE" > /tmp/manifest.json 2>/tmp/manifest-stderr.txt; then
    echo "stderr:"
    cat /tmp/manifest-stderr.txt
    fail "bootc-image-builder manifest exited with error"
fi

# Validate output is non-empty JSON
if [[ ! -s /tmp/manifest.json ]]; then
    fail "manifest output is empty"
fi

if ! python3 -m json.tool /tmp/manifest.json > /dev/null 2>&1; then
    echo "Raw output (first 500 chars):"
    head -c 500 /tmp/manifest.json
    fail "manifest output is not valid JSON"
fi

MANIFEST_SIZE=$(stat -c%s /tmp/manifest.json)
echo "Manifest generated: $MANIFEST_SIZE bytes"
echo "PASS: qcow2 manifest generation"

printf '{"result":"SUCCESS","successes":1,"failures":0,"warnings":0,"note":"qcow2 manifest generated successfully (%s bytes)"}' "$MANIFEST_SIZE" > "$RESULT_FILE"
