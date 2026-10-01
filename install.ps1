<#
.SYNOPSIS
Installs the HTB CLI for the current Windows user.

.DESCRIPTION
Downloads the Windows release archive, verifies its SHA-256 checksum, and
installs htb.exe without requiring administrator rights.
#>
[CmdletBinding()]
param(
    [string]$Repository = $env:HTB_REPOSITORY,
    [string]$Version = $env:HTB_VERSION,
    [string]$InstallDir = $env:HTB_INSTALL_DIR
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($Repository)) {
    $Repository = 'kazerlelutin/htb'
}
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = 'latest'
}
if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    $InstallDir = Join-Path $env:LOCALAPPDATA 'HTB\bin'
}

$archive = 'htb_windows_amd64.zip'
if ($Version -eq 'latest') {
    $baseUrl = "https://github.com/$Repository/releases/latest/download"
} else {
    $baseUrl = "https://github.com/$Repository/releases/download/$Version"
}

$temporaryDir = Join-Path ([System.IO.Path]::GetTempPath()) ("htb-" + [guid]::NewGuid())
$archivePath = Join-Path $temporaryDir $archive
$checksumsPath = Join-Path $temporaryDir 'checksums.txt'
$expandedDir = Join-Path $temporaryDir 'expanded'

try {
    New-Item -ItemType Directory -Path $temporaryDir | Out-Null
    Invoke-WebRequest -Uri "$baseUrl/$archive" -OutFile $archivePath
    Invoke-WebRequest -Uri "$baseUrl/checksums.txt" -OutFile $checksumsPath

    $archiveName = [regex]::Escape($archive)
    $checksumLine = Get-Content -LiteralPath $checksumsPath | Where-Object {
        $_ -match "^([a-fA-F0-9]{64})\s+\*?$archiveName$"
    } | Select-Object -First 1
    if ($null -eq $checksumLine) {
        throw "No checksum was published for $archive."
    }
    $expectedHash = ([regex]::Match($checksumLine, '^[a-fA-F0-9]{64}')).Value
    $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $archivePath).Hash
    if (-not $expectedHash.Equals($actualHash, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw 'Checksum verification failed; HTB was not installed.'
    }

    Expand-Archive -LiteralPath $archivePath -DestinationPath $expandedDir
    $binary = Join-Path $expandedDir 'htb.exe'
    if (-not (Test-Path -LiteralPath $binary -PathType Leaf)) {
        throw 'The release archive does not contain htb.exe.'
    }
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -LiteralPath $binary -Destination (Join-Path $InstallDir 'htb.exe') -Force
} finally {
    if (Test-Path -LiteralPath $temporaryDir) {
        Remove-Item -LiteralPath $temporaryDir -Recurse -Force
    }
}

$normalizedInstallDir = $InstallDir.TrimEnd('\')
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$pathEntries = @($userPath -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
if (-not ($pathEntries | Where-Object { $_.TrimEnd('\') -ieq $normalizedInstallDir })) {
    $newUserPath = @($pathEntries + $InstallDir) -join ';'
    [Environment]::SetEnvironmentVariable('Path', $newUserPath, 'User')
}
if (-not (($env:Path -split ';') | Where-Object { $_.TrimEnd('\') -ieq $normalizedInstallDir })) {
    $env:Path = "$InstallDir;$env:Path"
}

Write-Host "HTB installed: $(Join-Path $InstallDir 'htb.exe')"
Write-Host 'The current PowerShell session is ready; future terminals will also find htb on PATH.'
& (Join-Path $InstallDir 'htb.exe') version
