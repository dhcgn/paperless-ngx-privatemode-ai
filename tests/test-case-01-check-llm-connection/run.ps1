# Smoke test: builds the tool and verifies it can start the managed privatemode-proxy and
# reach the LLM service using the API key from .env. Does not require a real Paperless NGX instance.
$ErrorActionPreference = "Stop"

$ScriptDir = $PSScriptRoot
$RepoRoot = (Resolve-Path (Join-Path $ScriptDir "..\..")).Path
$Binary = Join-Path $ScriptDir "paperless-ngx-privatemode-ai-test.exe"
$GeneratedConfig = Join-Path $ScriptDir "config.generated.yaml"

function Remove-TestArtifacts {
    Remove-Item -Path $GeneratedConfig -Force -ErrorAction SilentlyContinue
    Remove-Item -Path $Binary -Force -ErrorAction SilentlyContinue
    Remove-Item -Path (Join-Path $ScriptDir "privatemode-proxy-data") -Recurse -Force -ErrorAction SilentlyContinue
}

try {
    $envFile = Join-Path $ScriptDir ".env"
    if (-not (Test-Path $envFile)) {
        Write-Error "FAIL: $envFile not found. Copy sample.env to .env and fill in PRIVATE_MODE_API_KEY."
        exit 1
    }

    $apiKey = $null
    foreach ($line in Get-Content $envFile) {
        if ($line -match '^\s*PRIVATE_MODE_API_KEY\s*=\s*(.*)\s*$') {
            $apiKey = $Matches[1].Trim()
        }
    }

    if ([string]::IsNullOrWhiteSpace($apiKey) -or $apiKey -eq "your_private_mode_api_key_here") {
        Write-Error "FAIL: PRIVATE_MODE_API_KEY is not set in $envFile"
        exit 1
    }

    $template = Get-Content (Join-Path $ScriptDir "config.template.yaml") -Raw
    $template.Replace("__PRIVATE_MODE_API_KEY__", $apiKey) | Set-Content -Path $GeneratedConfig -NoNewline

    Write-Host "Building tool..."
    & (Join-Path $RepoRoot "scripts\build.ps1") -OutputFile $Binary

    Write-Host "Running --check-llm-connection..."
    Push-Location $ScriptDir
    try {
        & $Binary --config $GeneratedConfig --check-llm-connection
        $exitCode = $LASTEXITCODE
    } finally {
        Pop-Location
    }

    if ($exitCode -eq 0) {
        Write-Host "PASS: managed proxy started and LLM service is accessible"
        exit 0
    } else {
        Write-Error "FAIL: tool exited with code $exitCode, see output above"
        exit $exitCode
    }
} finally {
    Remove-TestArtifacts
}
