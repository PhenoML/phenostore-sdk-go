#!/usr/bin/env bash
set -euo pipefail

if [[ ! "${SPEC_SHA:-}" =~ ^[a-fA-F0-9]{7,64}$ ]]; then
  echo "spec_sha must be a hexadecimal commit or content identifier" >&2
  exit 1
fi

spec_dir=$(mktemp -d)
trap 'rm -rf "$spec_dir"' EXIT
spec_file="$spec_dir/spec"
if ! printf '%s' "${SPEC_B64:-}" | base64 --decode > "$spec_dir/decoded"; then
  echo "spec_b64 must be a valid base64 OpenAPI document" >&2
  exit 1
fi
case "${SPEC_ENCODING:-base64}" in
  base64)
    cp "$spec_dir/decoded" "$spec_file"
    ;;
  gzip+base64)
    if ! gzip --decompress --stdout "$spec_dir/decoded" > "$spec_file"; then
      echo "spec_b64 must contain a valid gzip OpenAPI document" >&2
      exit 1
    fi
    ;;
  *)
    echo "spec_encoding must be base64 or gzip+base64" >&2
    exit 1
    ;;
esac
if [[ ! -s "$spec_file" ]]; then
  echo "spec_b64 must contain the upstream OpenAPI document" >&2
  exit 1
fi
cp "$spec_file" api/openapi.yaml
