# Publish one AtlasBuddy desktop build into releases/desktop/{channel}/.
#
# Clients read GET /atlas/desktop/{channel}/update.json and self-update from it.
# No signing key is involved: integrity is the SHA-256 + size written here.
#
# Usage (from services/atlas-server):
#   .\scripts\publish-desktop.ps1 -Version 0.2.43 -Platform windows-x86_64 `
#       -Payload E:\GitHub\grok-app\src-tauri\target\release\bundle\nsis\AtlasBuddy_0.2.43_x64-setup.exe
#
#   # macOS payload must be packed on macOS (scripts/publish-desktop.sh keeps
#   # exec bits / symlinks); this script accepts the finished .app.tar.gz:
#   .\scripts\publish-desktop.ps1 -Version 0.2.43 -Platform darwin-aarch64 `
#       -Payload .\AtlasBuddy_0.2.43_aarch64.app.tar.gz -Installer .\AtlasBuddy_0.2.43_aarch64.dmg
#
# Same version can be published platform by platform; a new version starts a
# fresh manifest. manifest.json is written last (atomic rename), so clients
# never see a platform whose file is still being copied.

param(
    [Parameter(Mandatory = $true)]
    [string]$Version,

    [Parameter(Mandatory = $true)]
    [ValidateSet('windows-x86_64', 'windows-x86_64-portable', 'darwin-aarch64', 'darwin-x86_64')]
    [string]$Platform,

    [Parameter(Mandatory = $true)]
    [string]$Payload,

    # Double-click installer for manual fallback (defaults to the payload on Windows).
    [string]$Installer = '',

    [ValidateSet('stable', 'alpha', 'enterprise')]
    [string]$Channel = 'stable',

    [string]$Notes = '',

    [string]$MinSupported = '',

    [string]$ReleasesDir = ''
)

$ErrorActionPreference = 'Stop'

if ($Version -notmatch '^\d+\.\d+\.\d+([-+][0-9A-Za-z.-]+)?$') {
    Write-Error "Version must be semver (e.g. 0.2.43), got: $Version"
    exit 1
}
if (-not (Test-Path -LiteralPath $Payload -PathType Leaf)) {
    Write-Error "Payload not found: $Payload"
    exit 1
}

$payloadName = [System.IO.Path]::GetFileName($Payload)
if ($Platform -eq 'windows-x86_64') {
    if ($payloadName -notmatch '-setup\.exe$') {
        Write-Error "Windows payload must be the NSIS *-setup.exe, got: $payloadName"
        exit 1
    }
    $kind = 'nsis'
    if (-not $Installer) { $Installer = $Payload }
} elseif ($Platform -eq 'windows-x86_64-portable') {
    if ($payloadName -notmatch '-portable\.zip$') {
        Write-Error "Windows portable payload must be the *-portable.zip, got: $payloadName"
        exit 1
    }
    $kind = 'portable_zip'
    if (-not $Installer) { $Installer = $Payload }
} else {
    if ($payloadName -notmatch '\.app\.tar\.gz$') {
        Write-Error "macOS payload must be a .app.tar.gz (pack it with scripts/publish-desktop.sh on macOS), got: $payloadName"
        exit 1
    }
    $kind = 'app_tar_gz'
}
if ($payloadName -notmatch "_$([regex]::Escape($Version))_") {
    Write-Error "Payload file name must contain _${Version}_ (got $payloadName) so cached URLs never change bytes"
    exit 1
}

function Assert-SafeName([string]$name) {
    if ($name -notmatch '^[A-Za-z0-9._-]+$' -or $name.Contains('..')) {
        Write-Error "File name must match [A-Za-z0-9._-]+ : $name"
        exit 1
    }
}
Assert-SafeName $payloadName

if (-not $ReleasesDir) {
    $ReleasesDir = Join-Path $PSScriptRoot '..\releases'
}
$channelDir = [System.IO.Path]::GetFullPath((Join-Path $ReleasesDir "desktop\$Channel"))
New-Item -ItemType Directory -Path $channelDir -Force | Out-Null

function Copy-Artifact([string]$src) {
    $name = [System.IO.Path]::GetFileName($src)
    Assert-SafeName $name
    $dest = Join-Path $channelDir $name
    if ([System.IO.Path]::GetFullPath($src) -ne $dest) {
        $tmp = "$dest.uploading"
        Copy-Item -LiteralPath $src -Destination $tmp -Force
        Move-Item -LiteralPath $tmp -Destination $dest -Force
    }
    return $name
}

$payloadFile = Copy-Artifact $Payload
$payloadPath = Join-Path $channelDir $payloadFile
$sha = (Get-FileHash -LiteralPath $payloadPath -Algorithm SHA256).Hash.ToLowerInvariant()
$size = (Get-Item -LiteralPath $payloadPath).Length

$installerFile = ''
if ($Installer) {
    if (-not (Test-Path -LiteralPath $Installer -PathType Leaf)) {
        Write-Error "Installer not found: $Installer"
        exit 1
    }
    $installerFile = Copy-Artifact $Installer
}

# Merge into an existing manifest of the same version; otherwise start fresh.
$manifestPath = Join-Path $channelDir 'manifest.json'
$platforms = [ordered]@{}
$installers = [ordered]@{}
$existingNotes = ''
$existingMin = ''
if (Test-Path -LiteralPath $manifestPath) {
    $old = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    if ($old.version -eq $Version) {
        if ($old.platforms) {
            foreach ($p in $old.platforms.PSObject.Properties) { $platforms[$p.Name] = $p.Value }
        }
        if ($old.installers) {
            foreach ($p in $old.installers.PSObject.Properties) { $installers[$p.Name] = $p.Value }
        }
        $existingNotes = [string]$old.notes
        $existingMin = [string]$old.min_supported
    }
}

$platforms[$Platform] = [ordered]@{ kind = $kind; file = $payloadFile; sha256 = $sha; size = $size }
if ($installerFile) { $installers[$Platform] = $installerFile }

$manifest = [ordered]@{
    version       = $Version
    notes         = $(if ($Notes) { $Notes } else { $existingNotes })
    pub_date      = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
    min_supported = $(if ($MinSupported) { $MinSupported } else { $existingMin })
    platforms     = $platforms
    installers    = $installers
}

$tmpManifest = "$manifestPath.tmp"
$json = $manifest | ConvertTo-Json -Depth 6
[System.IO.File]::WriteAllText($tmpManifest, $json, (New-Object System.Text.UTF8Encoding($false)))
Move-Item -LiteralPath $tmpManifest -Destination $manifestPath -Force

Write-Host "Published $Platform $Version ($kind) -> $payloadFile" -ForegroundColor Green
Write-Host "sha256 $sha  size $size"
Write-Host "Check: GET {public_base_url}/desktop/$Channel/update.json"
