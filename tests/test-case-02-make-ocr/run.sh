#!/bin/bash
# Smoke test: builds the tool and verifies it can perform OCR on a local image (SCN_0001.webp)
# via the managed privatemode-proxy, using the API key from .env.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BINARY="$SCRIPT_DIR/paperless-ngx-privatemode-ai-test"
GENERATED_CONFIG="$SCRIPT_DIR/config.generated.yaml"
IMAGE_FILE="$SCRIPT_DIR/SCN_0001.webp"
EXPECTED_KEYWORD="INVOICE"

cleanup() {
  rm -f "$GENERATED_CONFIG" "$BINARY"
  rm -rf "$SCRIPT_DIR/privatemode-proxy-data"
}
trap cleanup EXIT

if [ ! -f "$SCRIPT_DIR/.env" ]; then
  echo "FAIL: $SCRIPT_DIR/.env not found. Copy sample.env to .env and fill in PRIVATE_MODE_API_KEY." >&2
  exit 1
fi

if [ ! -f "$IMAGE_FILE" ]; then
  echo "FAIL: $IMAGE_FILE not found." >&2
  exit 1
fi

set -a
# shellcheck source=/dev/null
source "$SCRIPT_DIR/.env"
set +a

if [ -z "${PRIVATE_MODE_API_KEY:-}" ] || [ "$PRIVATE_MODE_API_KEY" = "your_private_mode_api_key_here" ]; then
  echo "FAIL: PRIVATE_MODE_API_KEY is not set in $SCRIPT_DIR/.env" >&2
  exit 1
fi

template="$(cat "$SCRIPT_DIR/config.template.yaml")"
template="${template//__PRIVATE_MODE_API_KEY__/$PRIVATE_MODE_API_KEY}"
# tools.imagemagick-for-windows is ignored on Linux (auto-detected from PATH), leave it empty
printf '%s' "${template//__IMAGEMAGICK_PATH__/}" > "$GENERATED_CONFIG"

echo "Building tool..."
(cd "$REPO_ROOT" && bash scripts/build.sh "$BINARY")

echo "Running --ocr-file..."
set +e
OUTPUT="$(cd "$SCRIPT_DIR" && "$BINARY" --config "$GENERATED_CONFIG" --ocr-file "$IMAGE_FILE" 2>&1)"
EXIT_CODE=$?
set -e

echo "$OUTPUT"

if [ $EXIT_CODE -ne 0 ]; then
  echo "FAIL: tool exited with code $EXIT_CODE, see output above" >&2
  exit 1
fi

if ! echo "$OUTPUT" | grep -qi "$EXPECTED_KEYWORD"; then
  echo "FAIL: extracted text did not contain expected keyword '$EXPECTED_KEYWORD'" >&2
  exit 1
fi

echo "PASS: OCR extracted text containing '$EXPECTED_KEYWORD'"
exit 0
