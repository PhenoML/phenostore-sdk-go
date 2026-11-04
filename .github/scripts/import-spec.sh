#!/usr/bin/env bash
set -euo pipefail

if [[ ! "${SPEC_SHA:-}" =~ ^[a-fA-F0-9]{7,64}$ ]]; then
  echo "spec_sha must be a hexadecimal commit or content identifier" >&2
  exit 1
fi

spec_file=$(mktemp)
trap 'rm -f "$spec_file"' EXIT
if ! printf '%s' "${SPEC_B64:-}" | base64 --decode > "$spec_file"; then
  echo "spec_b64 must be a valid base64 OpenAPI document" >&2
  exit 1
fi
if [[ ! -s "$spec_file" ]]; then
  echo "spec_b64 must contain the upstream OpenAPI document" >&2
  exit 1
fi
cp "$spec_file" api/openapi.yaml
