#!/bin/bash

# Parse arguments
OUTPUT_FILE=${1:-"paperless-ngx-privatemode-ai"}  # Default to "paperless-ngx-privatemode-ai" if no argument provided

# Get version info (format: v1.0.0-5-g1234567 or v1.0.0)
GIT_DESC=$(git describe --tags 2>/dev/null || echo "v0.0.0")

# Parse version components
BASE_TAG=$(echo $GIT_DESC | cut -d- -f1)
COMMITS_SINCE=$(echo $GIT_DESC | grep -o '[0-9]*-g[0-9a-f]*$' | cut -d- -f1)

# Build final version string
if [ -n "$COMMITS_SINCE" ]; then
  VERSION="${BASE_TAG}+${COMMITS_SINCE}"
else
  VERSION="${BASE_TAG}"
fi

COMMIT=$(git rev-parse --short HEAD)
BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

# privatemode-proxy reports this to the Privatemode API, which rejects unrecognized/old versions.
# Must match the github.com/edgelesssys/continuum version pinned in go.mod.
PRIVATEMODE_PROXY_VERSION="v1.50.0"

# Build the main package instead of all packages
# contrast_unstable_api is required to enable the privatemode-proxy attestation SDK we embed
go build -tags contrast_unstable_api -trimpath -v -o "${OUTPUT_FILE}" -ldflags "-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILDTIME} -X github.com/edgelesssys/continuum/internal/oss/constants.version=${PRIVATEMODE_PROXY_VERSION}" ./main.go