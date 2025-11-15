#!/usr/bin/env pwsh

# Installation script for MCP CLI (Windows PowerShell)
# Usage: iwr -useb https://raw.githubusercontent.com/EstebanForge/mcp-cli-ent/main/scripts/install.ps1 | iex

param(
    [string]$InstallDir = $null,
    [switch]$Force = $false
)

# Colors for output
$Colors = @{
    Red = "Red"
    Green = "Green"
    Yellow = "Yellow"
    Cyan = "Cyan"
}

function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    Write-Host $Message -ForegroundColor $Colors[$Color]
}

function Get-PlatformInfo {
    $Architecture = $env:PROCESSOR_ARCHITECTURE
    $Is64Bit = [Environment]::Is64BitOperatingSystem

    if ($Is64Bit -and $Architecture -eq "AMD64") {
        return "amd64"
    } elseif ($Is64Bit -and $Architecture -eq "ARM64") {
        return "arm64"
    } elseif ($Architecture -eq "AMD64") {
        return "amd64"
    } else {
        return "386"
    }
}

function Get-LatestVersion {
    try {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/EstebanForge/mcp-cli-ent/releases/latest" -Headers @{
            "Accept" = "application/vnd.github.v3+json"
        } -ErrorAction Stop
        return $response.tag_name
    }
    catch {
        Write-ColorOutput "Failed to fetch latest version: $_" "Red"
        return "latest"
    }
}

function Test-CommandExists {
    param([string]$Command)
    try {
        $null = Get-Command $Command -ErrorAction Stop
        return $true
    }
    catch {
        return $false
    }
}

Write-ColorOutput "MCP CLI Installer for Windows" "Cyan"
Write-ColorOutput "============================" "Cyan"

# Determine install directory
if (-not $InstallDir) {
    $InstallDir = "$env:USERPROFILE\AppData\Roaming\mcp-cli-ent"
}

# Create install directory if it doesn't exist
if (-not (Test-Path $InstallDir)) {
    try {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        Write-ColorOutput "Created install directory: $InstallDir" "Green"
    }
    catch {
        Write-ColorOutput "Failed to create install directory: $_" "Red"
        exit 1
    }
}

# Check if already installed
$BinaryPath = Join-Path $InstallDir "mcp-cli-ent.exe"
if ((Test-Path $BinaryPath) -and (-not $Force)) {
    Write-ColorOutput "MCP CLI is already installed at: $BinaryPath" "Yellow"
    $Choice = Read-Host "Do you want to reinstall? (y/N)"
    if ($Choice -ne "y" -and $Choice -ne "Y") {
        Write-ColorOutput "Installation cancelled." "Yellow"
        exit 0
    }
}

# Detect platform
$Architecture = Get-PlatformInfo
$Platform = "windows"
$FileName = "mcp-cli-ent-windows-$Architecture.exe"

Write-ColorOutput "Detected platform: $Platform-$Architecture" "Green"

# Get latest version
$Version = Get-LatestVersion
Write-ColorOutput "Latest version: $Version" "Green"

# Construct download URL
$DownloadUrl = "https://github.com/EstebanForge/mcp-cli-ent/releases/download/$Version/$FileName"

# Download the binary
Write-ColorOutput "Downloading MCP CLI..." "Cyan"
try {
    $TempPath = Join-Path $env:TEMP $FileName
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TempPath -UseBasicParsing -ErrorAction Stop

    # Verify download
    if (-not (Test-Path $TempPath)) {
        throw "Download failed - file not found"
    }

    Write-ColorOutput "Download completed successfully" "Green"
}
catch {
    Write-ColorOutput "Failed to download MCP CLI: $_" "Red"
    exit 1
}

# Install the binary
Write-ColorOutput "Installing MCP CLI to: $InstallDir" "Cyan"
try {
    Move-Item -Path $TempPath -Destination $BinaryPath -Force
    Write-ColorOutput "Installation completed successfully!" "Green"
}
catch {
    Write-ColorOutput "Failed to install MCP CLI: $_" "Red"
    # Clean up temp file
    if (Test-Path $TempPath) {
        Remove-Item $TempPath -Force
    }
    exit 1
}

# Test installation
try {
    Write-ColorOutput "Testing installation..." "Cyan"
    $Output = & $BinaryPath --version 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-ColorOutput "Installation successful!" "Green"
        Write-ColorOutput "MCP CLI version: $($Output -join '')" "Green"
    } else {
        throw "Binary test failed"
    }
}
catch {
    Write-ColorOutput "Installation test failed: $_" "Red"
    exit 1
}

# Check if installation directory is in PATH
$EnvPaths = $env:PATH -split ';'
$InstallDirInPath = $EnvPaths -contains $InstallDir

if (-not $InstallDirInPath) {
    Write-ColorOutput "WARNING: Installation directory is not in your PATH" "Yellow"
    Write-ColorOutput "Add '$InstallDir' to your PATH environment variable" "Yellow"
    Write-ColorOutput "Or run: [Environment]::SetEnvironmentVariable('PATH', [Environment]::GetEnvironmentVariable('PATH') + ';$InstallDir', 'User')" "Cyan"
} else {
    Write-ColorOutput "Installation directory is already in PATH" "Green"
}

Write-ColorOutput "Installation complete!" "Green"
Write-ColorOutput "You can now run: mcp-cli-ent --help" "Cyan"
Write-ColorOutput "" "White"
Write-ColorOutput "First time setup:" "Cyan"
Write-ColorOutput "mcp-cli-ent list-servers" "White"