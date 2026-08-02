# Builds the Windows binary with version info and the flags required by the embedded privatemode-proxy.
param(
    [string]$OutputFile = "paperless-ngx-privatemode-ai.exe"
)

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

# Resolve OutputFile relative to the caller's current directory before we switch to the repo root.
if (-not [System.IO.Path]::IsPathRooted($OutputFile)) {
    $OutputFile = Join-Path (Get-Location) $OutputFile
}

Push-Location $RepoRoot
try {
    # Get version info (format: v1.0.0-5-g1234567 or v1.0.0)
    $gitDesc = (git describe --tags 2>$null)
    if ([string]::IsNullOrWhiteSpace($gitDesc)) {
        $gitDesc = "v0.0.0"
    }

    # Parse version components
    $baseTag = $gitDesc.Split('-')[0]
    $commitsSince = ""
    if ($gitDesc -match '(\d+)-g[0-9a-f]+$') {
        $commitsSince = $Matches[1]
    }

    # Build final version string
    if ($commitsSince) {
        $version = "$baseTag+$commitsSince"
    } else {
        $version = $baseTag
    }

    $commit = git rev-parse --short HEAD
    $buildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-dd_HH:mm:ss")

    # privatemode-proxy reports this to the Privatemode API, which rejects unrecognized/old versions.
    # Must match the github.com/edgelesssys/continuum version pinned in go.mod.
    $privatemodeProxyVersion = "v1.50.0"

    $ldflags = "-s -w -X main.Version=$version -X main.Commit=$commit -X main.BuildTime=$buildTime " +
        "-X github.com/edgelesssys/continuum/internal/oss/constants.version=$privatemodeProxyVersion"

    # Build the main package instead of all packages
    # contrast_unstable_api is required to enable the privatemode-proxy attestation SDK we embed
    go build -tags contrast_unstable_api -trimpath -v -o $OutputFile -ldflags $ldflags ./main.go
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed with exit code $LASTEXITCODE"
    }
} finally {
    Pop-Location
}
