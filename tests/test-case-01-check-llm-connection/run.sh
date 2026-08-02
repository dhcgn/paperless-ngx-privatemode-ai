#!/bin/bash
# Smoke test: builds the tool and verifies it can start the managed privatemode-proxy and
# reach the LLM service using the API key from .env. Does not require a real Paperless NGX instance.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BINARY="$SCRIPT_DIR/paperless-ngx-privatemode-ai-test"
GENERATED_CONFIG="$SCRIPT_DIR/config.generated.yaml"

cleanup() {
  rm -f "$GENERATED_CONFIG" "$BINARY"
  rm -rf "$SCRIPT_DIR/privatemode-proxy-data"
}
trap cleanup EXIT

if [ ! -f "$SCRIPT_DIR/.env" ]; then
  echo "FAIL: $SCRIPT_DIR/.env not found. Copy sample.env to .env and fill in PRIVATE_MODE_API_KEY." >&2
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
printf '%s' "${template//__PRIVATE_MODE_API_KEY__/$PRIVATE_MODE_API_KEY}" > "$GENERATED_CONFIG"

echo "Building tool..."
(cd "$REPO_ROOT" && bash scripts/build.sh "$BINARY")

echo "Running --check-llm-connection..."
if (cd "$SCRIPT_DIR" && "$BINARY" --config "$GENERATED_CONFIG" --check-llm-connection); then
  echo "PASS: managed proxy started and LLM service is accessible"
  exit 0
else
  echo "FAIL: tool exited with an error, see output above" >&2
  exit 1
fi
