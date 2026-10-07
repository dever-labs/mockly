#!/usr/bin/env pwsh
<#
.SYNOPSIS
  Install Mockly from GitHub Releases on Windows.

.DESCRIPTION
  Downloads the mockly-windows-amd64.exe release asset and installs it as
  mockly.exe, then adds the install directory to the current user's PATH
  if it isn't there already.

.PARAMETER Version
  Release tag to install (default: latest). Can also be set via the
  MOCKLY_VERSION environment variable.

.PARAMETER InstallDir
  Directory to place the binary (default: $env:LOCALAPPDATA\mockly). Can
  also be set via the INSTALL_DIR environment variable.

.EXAMPLE
  irm https://raw.githubusercontent.com/dever-labs/mockly/main/install.ps1 | iex

.EXAMPLE
  $env:MOCKLY_VERSION = "v0.14.0"
  irm https://raw.githubusercontent.com/dever-labs/mockly/main/install.ps1 | iex

.EXAMPLE
  .\install.ps1 -Version v0.14.0 -InstallDir C:\tools\mockly
#>
[CmdletBinding()]
param(
  [string]$Version = $(if ($env:MOCKLY_VERSION) { $env:MOCKLY_VERSION } else { "latest" }),
  [string]$InstallDir = $(if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { "$env:LOCALAPPDATA\mockly" })
)

$ErrorActionPreference = "Stop"
$Repo = "dever-labs/mockly"

# ── Detect arch ────────────────────────────────────────────────────────────
$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  "AMD64" { "amd64" }
  default {
    throw "Unsupported architecture: $($env:PROCESSOR_ARCHITECTURE) (only amd64 Windows binaries are published)"
  }
}

# ── Resolve version ────────────────────────────────────────────────────────
if ($Version -eq "latest") {
  Write-Host "Resolving latest Mockly version..."
  $release = Invoke-RestMethod -UseBasicParsing `
    -Headers @{ "Accept" = "application/vnd.github+json" } `
    -Uri "https://api.github.com/repos/$Repo/releases/latest"
  $Version = $release.tag_name
}

if ([string]::IsNullOrWhiteSpace($Version)) {
  throw "Failed to resolve version. Set -Version or `$env:MOCKLY_VERSION explicitly."
}

# ── Download ───────────────────────────────────────────────────────────────
$BinaryName = "mockly-windows-$Arch.exe"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$BinaryName"
$TmpFile = New-TemporaryFile

Write-Host "Installing mockly $Version (windows/$Arch)..."
Invoke-WebRequest -UseBasicParsing -Uri $DownloadUrl -OutFile $TmpFile

# ── Install ────────────────────────────────────────────────────────────────
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$TargetPath = Join-Path $InstallDir "mockly.exe"
Move-Item -Force -Path $TmpFile -Destination $TargetPath

# ── Add to PATH (current user) if not already present ─────────────────────
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (-not ($UserPath -split ";" | Where-Object { $_ -eq $InstallDir })) {
  Write-Host "Adding $InstallDir to your user PATH (restart your shell to pick it up)..."
  [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
}

Write-Host "mockly $Version installed to $TargetPath"
