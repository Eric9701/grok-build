# Publish Atlas CLI binary into atlas-server releases/ layout.
#
# Usage (from repo root or services/atlas-server):
#   .\scripts\publish-release.ps1 -Binary ..\..\target\release\xai-grok-pager.exe -Version 0.2.110
#   .\scripts\publish-release.ps1 -Binary D:\atlas\atlas.exe -Version 0.2.110 -Channel alpha
#   .\scripts\publish-release.ps1 -Binary .\atlas -Version 0.2.110 -Os linux -Arch x86_64
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

    [ValidateSet('windows', 'linux', 'macos')]
    [string]$Os = '',

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

if (-not $Os) {
    $ext = [System.IO.Path]::GetExtension($Binary)
    if ($ext -eq '.exe') {
        $Os = 'windows'
    } elseif ($IsLinux) {
        $Os = 'linux'
    } elseif ($IsMacOS) {
        $Os = 'macos'
    } else {
        $Os = 'windows'
    }
}

if (-not $Arch) {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { $Arch = 'x86_64' }
        'ARM64' { $Arch = 'aarch64' }
        default {
            $unameM = ''
            try { $unameM = (uname -m 2>$null) } catch { }
            switch -Regex ($unameM) {
                '^(x86_64|amd64)$' { $Arch = 'x86_64' }
                '^(arm64|aarch64)$' { $Arch = 'aarch64' }
                default {
                    Write-Error "Unsupported PROCESSOR_ARCHITECTURE=$env:PROCESSOR_ARCHITECTURE; pass -Arch x86_64 or aarch64"
                    exit 1
                }
            }
        }
    }
}

$platform = "$Os-$Arch"
$artifact = if ($Os -eq 'windows') { "grok-$Version-$platform.exe" } else { "grok-$Version-$platform" }
$dest = Join-Path $ReleasesDir $artifact
$channelFile = Join-Path $ReleasesDir $Channel

Copy-Item -LiteralPath $Binary -Destination $dest -Force
Set-Content -LiteralPath $channelFile -Value $Version.Trim() -NoNewline -Encoding ascii

$pagerScripts = Join-Path $PSScriptRoot '..\..\..\crates\codegen\xai-grok-pager\scripts'
foreach ($name in @('install.ps1', 'install-enterprise.ps1', 'install.sh', 'install-enterprise.sh')) {
    $src = Join-Path $pagerScripts $name
    if (Test-Path -LiteralPath $src) {
        Copy-Item -LiteralPath $src -Destination (Join-Path $ReleasesDir $name) -Force
    }
}

Write-Host "Published $artifact" -ForegroundColor Green
Write-Host "Channel $Channel -> $Version" -ForegroundColor Green
Write-Host "Dir: $ReleasesDir"
