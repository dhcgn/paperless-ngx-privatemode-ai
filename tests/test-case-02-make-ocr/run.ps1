# Smoke test: builds the tool and verifies it can perform OCR on a local image (SCN_0001.webp)
# via the managed privatemode-proxy, using the API key from .env.
$ErrorActionPreference = "Stop"

$ScriptDir = $PSScriptRoot
$RepoRoot = (Resolve-Path (Join-Path $ScriptDir "..\..")).Path
$Binary = Join-Path $ScriptDir "paperless-ngx-privatemode-ai-test.exe"
$GeneratedConfig = Join-Path $ScriptDir "config.generated.yaml"
$ImageFile = Join-Path $ScriptDir "SCN_0001.webp"
$ExpectedKeyword = "INVOICE"

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

    if (-not (Test-Path $ImageFile)) {
        Write-Error "FAIL: $ImageFile not found."
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

    $imageMagickPath = Get-ChildItem "C:\Program Files\ImageMagick-*\magick.exe" -ErrorAction SilentlyContinue |
        Select-Object -First 1 -ExpandProperty FullName
    if (-not $imageMagickPath) {
        $cmd = Get-Command magick.exe -ErrorAction SilentlyContinue
        if ($cmd) { $imageMagickPath = $cmd.Source }
    }
    if (-not $imageMagickPath) {
        Write-Error "FAIL: ImageMagick not found under 'C:\Program Files\ImageMagick-*' or on PATH."
        exit 1
    }
    # YAML single-quoted strings escape ' as ''; also normalize backslashes are fine unescaped in single quotes.
    $imageMagickPathYaml = $imageMagickPath.Replace("'", "''")

    $template = Get-Content (Join-Path $ScriptDir "config.template.yaml") -Raw
    $template = $template.Replace("__PRIVATE_MODE_API_KEY__", $apiKey)
    $template = $template.Replace("__IMAGEMAGICK_PATH__", $imageMagickPathYaml)
    $template | Set-Content -Path $GeneratedConfig -NoNewline

    Write-Host "Building tool..."
    & (Join-Path $RepoRoot "scripts\build.ps1") -OutputFile $Binary

    Write-Host "Running --ocr-file..."
    Push-Location $ScriptDir
    try {
        $output = & $Binary --config $GeneratedConfig --ocr-file $ImageFile 2>&1 | Out-String
        $exitCode = $LASTEXITCODE
    } finally {
        Pop-Location
    }

    Write-Host $output

    if ($exitCode -ne 0) {
        Write-Error "FAIL: tool exited with code $exitCode, see output above"
        exit $exitCode
    }

    if ($output -notmatch [regex]::Escape($ExpectedKeyword)) {
        Write-Error "FAIL: extracted text did not contain expected keyword '$ExpectedKeyword'"
        exit 1
    }

    Write-Host "PASS: OCR extracted text containing '$ExpectedKeyword'"
    exit 0
} finally {
    Remove-TestArtifacts
}
