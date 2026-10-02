#!/usr/bin/env bash
set -euo pipefail

# Expects:
#   SNAPSHOT       — Konflux snapshot JSON with .components[].containerImage
#   RESULT_FILE    — path to write the Konflux TEST_OUTPUT result JSON
#   IMAGE_URL_FILE — path to write the extracted image URL (for downstream tasks)
#   PODINFO_LABELS (optional) — path to downward-API labels file

fail() {
    printf '{"result":"FAILURE","successes":0,"failures":1,"warnings":0,"note":"%s"}' "$1" > "$RESULT_FILE"
    exit 1
}

# Extract component name from pod labels (if available)
COMPONENT=""
if [[ -f "${PODINFO_LABELS:-/etc/podinfo/labels}" ]]; then
    COMPONENT=$(grep -oP '(?<=component=")[^"]+' "${PODINFO_LABELS:-/etc/podinfo/labels}" || echo "")
fi

# Extract container image from SNAPSHOT
if [[ -n "$COMPONENT" ]]; then
    IMAGE=$(echo "${SNAPSHOT}" | jq -r ".components[]|select(.name==\"$COMPONENT\")|.containerImage")
else
    IMAGE=$(echo "${SNAPSHOT}" | jq -r '.components[0].containerImage')
fi

if [[ -z "$IMAGE" || "$IMAGE" == "null" ]]; then
    fail "Could not extract image URL from SNAPSHOT"
fi

echo "Testing image: $IMAGE"

echo "=== Inspecting image metadata ==="
if ! skopeo inspect --no-tags "docker://$IMAGE" > /tmp/inspect.json; then
    fail "skopeo inspect failed — image is not pullable"
fi

echo "=== Image labels ==="
jq '.Labels' /tmp/inspect.json

echo "=== Inspecting image config ==="
if ! skopeo inspect --no-tags --config "docker://$IMAGE" > /tmp/config.json; then
    fail "skopeo inspect --config failed"
fi

echo "=== Entrypoint ==="
jq '.config.Entrypoint' /tmp/config.json

if ! jq -r '(.config.Entrypoint // [])[]' /tmp/config.json | grep -q bootc-image-builder; then
    fail "Entrypoint does not contain bootc-image-builder"
fi

echo "PASS: image metadata verified"
printf '%s' "$IMAGE" > "$IMAGE_URL_FILE"
printf '{"result":"SUCCESS","successes":1,"failures":0,"warnings":0,"note":"image metadata verified"}' > "$RESULT_FILE"
