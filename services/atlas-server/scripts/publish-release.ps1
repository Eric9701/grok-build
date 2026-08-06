# Publish Atlas CLI binary into atlas-server releases/ layout.
#
# Usage (from repo root or services/atlas-server):
#   .\scripts\publish-release.ps1 -Binary ..\..\target\release\xai-grok-pager.exe -Version 0.2.110
#   .\scripts\publish-release.ps1 -Binary D:\atlas\atlas.exe -Version 0.2.110 -Channel alpha
#
# Then restart/reload is not required — files are served from disk immediately.
# Point the CLI at this server:
#   $env:GROK_CLI_BASE_URL = "http://127.0.0.1:22255/cli"
#   # or in ~/.atlas/config.toml:
#   # [endpoints]
#   # cli_update_base_url = "http://127.0.0.1:22255/cli"
#   # cli_chat_proxy_base_url = "http://127.0.0.1:22255/v1"

param(
    [Parameter(Mandatory = $true)]
    [string]$Binary,

    [Parameter(Mandatory = $true)]
    [string]$Version,

    [ValidateSet('stable', 'alpha', 'enterprise')]
    [string]$Channel = 'stable',

    [string]$ReleasesDir = '',

    [ValidateSet('x86_64', 'aarch64')]
    [string]$Arch = ''
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path -LiteralPath $Binary)) {
    Write-Error "Binary not found: $Binary"
    exit 1
}

if ($Version -notmatch '^\d+\.\d+\.\d+') {
    Write-Error "Version must look like semver (e.g. 0.2.110), got: $Version"
    exit 1
}

if (-not $ReleasesDir) {
    $ReleasesDir = Join-Path $PSScriptRoot '..\releases'
}
$ReleasesDir = [System.IO.Path]::GetFullPath($ReleasesDir)
New-Item -ItemType Directory -Path $ReleasesDir -Force | Out-Null

if (-not $Arch) {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { $Arch = 'x86_64' }
        'ARM64' { $Arch = 'aarch64' }
        default { Write-Error "Unsupported PROCESSOR_ARCHITECTURE=$env:PROCESSOR_ARCHITECTURE"; exit 1 }
    }
}

$platform = "windows-$Arch"
$artifact = "grok-$Version-$platform.exe"
$dest = Join-Path $ReleasesDir $artifact
$channelFile = Join-Path $ReleasesDir $Channel

Copy-Item -LiteralPath $Binary -Destination $dest -Force
Set-Content -LiteralPath $channelFile -Value $Version.Trim() -NoNewline -Encoding ascii

# Also publish install.ps1 from the pager scripts if present.
$installSrc = Join-Path $PSScriptRoot '..\..\..\crates\codegen\xai-grok-pager\scripts\install.ps1'
if (Test-Path -LiteralPath $installSrc) {
    Copy-Item -LiteralPath $installSrc -Destination (Join-Path $ReleasesDir 'install.ps1') -Force
}

Write-Host "Published $artifact" -ForegroundColor Green
Write-Host "Channel $Channel -> $Version" -ForegroundColor Green
Write-Host "Dir: $ReleasesDir"
Write-Host ""
Write-Host "Smoke check:"
Write-Host "  curl http://127.0.0.1:22255/cli/$Channel"
Write-Host "  curl -OJ http://127.0.0.1:22255/cli/$artifact"
