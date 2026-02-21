#Requires -Version 5.1
<#
.SYNOPSIS
    WUT Installer Script for Windows
    
.DESCRIPTION
    One-line install: irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex
    
.PARAMETER Version
    Install specific version (default: latest)
    
.PARAMETER InstallDir
    Installation directory (default: auto-detect)
    
.PARAMETER NoInit
    Skip running 'wut init --quick'
    
.PARAMETER NoShell
    Skip PowerShell profile integration
    
.PARAMETER Force
    Force overwrite existing installation
    
.PARAMETER Uninstall
    Uninstall WUT
    
.EXAMPLE
    # Default install
    irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex
    
.EXAMPLE
    # Install specific version
    & ([scriptblock]::Create((irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1))) -Version "v1.0.0"
    
.EXAMPLE
    # Uninstall
    & ([scriptblock]::Create((irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1))) -Uninstall
#>

[CmdletBinding()]
param(
    [string]$Version = "latest",
    [string]$InstallDir = "",
    [switch]$NoInit,
    [switch]$NoShell,
    [switch]$Force,
    [switch]$Uninstall
)

# Configuration
$script:Repo = "thirawat27/wut"
$script:Binary = "wut"
$script:ErrorActionPreference = "Stop"

# Colors for PowerShell
$Colors = @{
    Red = "`e[31m"
    Green = "`e[32m"
    Yellow = "`e[33m"
    Blue = "`e[34m"
    Cyan = "`e[36m"
    NC = "`e[0m"
    Bold = "`e[1m"
}

function Write-Header {
    Write-Host ""
    Write-Host "$($Colors.Cyan)$($Colors.Bold) _    _ _____ _____$($Colors.NC)"
    Write-Host "$($Colors.Cyan)$($Colors.Bold)| |  | |_   _|  __ \$($Colors.NC)"
    Write-Host "$($Colors.Cyan)$($Colors.Bold)| |  | | | | | |  | |$($Colors.NC)"
    Write-Host "$($Colors.Cyan)$($Colors.Bold)| |  | | | | | |  | |$($Colors.NC)"
    Write-Host "$($Colors.Cyan)$($Colors.Bold)| |__| |_| |_| |__| |$($Colors.NC)"
    Write-Host "$($Colors.Cyan)$($Colors.Bold) \____/|_____|_____/$($Colors.NC)"
    Write-Host ""
    Write-Host "$($Colors.Blue)AI-Powered Command Helper for Windows$($Colors.NC)"
    Write-Host ""
}

function Write-Info { param([string]$Message) Write-Host "$($Colors.Blue)[INFO]$($Colors.NC) $Message" }
function Write-Success { param([string]$Message) Write-Host "$($Colors.Green)[OK]$($Colors.NC) $Message" }
function Write-Warn { param([string]$Message) Write-Host "$($Colors.Yellow)[WARN]$($Colors.NC) $Message" }
function Write-Error { param([string]$Message) Write-Host "$($Colors.Red)[ERROR]$($Colors.NC) $Message" -ForegroundColor Red }

function Test-Admin {
    $currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    return $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-Architecture {
    $arch = [System.Environment]::Is64BitOperatingSystem
    $processor = [System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE")
    
    switch ($processor) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        "x86" { return "386" }
        default { 
            if ($arch) { return "amd64" }
            else { return "386" }
        }
    }
}

function Get-LatestVersion {
    try {
        $apiUrl = "https://api.github.com/repos/$($script:Repo)/releases/latest"
        $response = Invoke-RestMethod -Uri $apiUrl -TimeoutSec 10
        return $response.tag_name
    }
    catch {
        Write-Warn "Could not fetch latest version, using 'latest'"
        return "latest"
    }
}

function Get-InstallDirectory {
    param([string]$PreferredDir)
    
    if ($PreferredDir) {
        return $PreferredDir
    }
    
    # Priority: Program Files > LocalAppData > UserProfile
    $programFiles = ${env:ProgramFiles}
    $localAppData = $env:LOCALAPPDATA
    $userProfile = $env:USERPROFILE
    
    if (Test-Admin) {
        # Admin: use Program Files
        $dir = Join-Path $programFiles "WUT"
    }
    else {
        # User: use LocalAppData
        $dir = Join-Path $localAppData "WUT"
    }
    
    # Create directory if not exists
    if (!(Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    
    return $dir
}

function Add-ToPath {
    param([string]$Directory)
    
    $currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    
    if ($currentPath -notlike "*$Directory*") {
        $newPath = "$currentPath;$Directory"
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        Write-Success "Added to PATH: $Directory"
        
        # Also update current session
        $env:PATH = "$env:PATH;$Directory"
    }
}

function Remove-FromPath {
    param([string]$Directory)
    
    $currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    $newPath = ($currentPath -split ';' | Where-Object { $_ -ne $Directory }) -join ';'
    [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
}

function Install-Wut {
    param(
        [string]$Version,
        [string]$InstallDir
    )
    
    $arch = Get-Architecture
    Write-Info "Detected architecture: $arch"
    
    # Get version
    if ($Version -eq "latest") {
        Write-Info "Fetching latest version..."
        $Version = Get-LatestVersion
    }
    Write-Info "Version: $Version"
    
    # Determine install directory
    $installDir = Get-InstallDirectory -PreferredDir $InstallDir
    Write-Info "Install directory: $installDir"
    
    # Download URL
    $fileName = "$($script:Binary)-windows-$arch.exe"
    if ($Version -eq "latest") {
        $downloadUrl = "https://github.com/$($script:Repo)/releases/latest/download/$fileName"
    }
    else {
        $downloadUrl = "https://github.com/$($script:Repo)/releases/download/$Version/$fileName"
    }
    
    # Check existing
    $targetPath = Join-Path $installDir "$($script:Binary).exe"
    if (Test-Path $targetPath) {
        if (!$Force) {
            Write-Warn "WUT is already installed at: $targetPath"
            $response = Read-Host "Overwrite? [y/N]"
            if ($response -notmatch '^[Yy]$') {
                throw "Installation cancelled"
            }
        }
        Remove-Item $targetPath -Force
    }
    
    # Download with progress
    Write-Info "Downloading from: $downloadUrl"
    $tempFile = [System.IO.Path]::GetTempFileName() + ".exe"
    
    try {
        # Use WebClient for progress
        $webClient = New-Object System.Net.WebClient
        $webClient.Headers.Add("User-Agent", "WUT-Installer")
        
        $lastProgress = 0
        Register-ObjectEvent -InputObject $webClient -EventName DownloadProgressChanged -Action {
            $progress = $EventArgs.ProgressPercentage
            if ($progress -gt $lastProgress -and $progress % 10 -eq 0) {
                Write-Info "Download progress: $progress%"
                $script:lastProgress = $progress
            }
        } | Out-Null
        
        $webClient.DownloadFile($downloadUrl, $tempFile)
        Unregister-Event -SourceIdentifier $webClient.GetHashCode() -ErrorAction SilentlyContinue
        
        Write-Success "Download complete"
    }
    catch {
        # Try alternative URLs
        $altUrls = @(
            $downloadUrl -replace "windows-$arch", "windows-amd64",
            $downloadUrl -replace "windows-$arch", "windows-386",
            "https://github.com/$($script:Repo)/releases/download/$Version/wut.exe"
        )
        
        $downloaded = $false
        foreach ($url in $altUrls) {
            try {
                Write-Info "Trying: $url"
                Invoke-WebRequest -Uri $url -OutFile $tempFile -UseBasicParsing -TimeoutSec 30
                $downloaded = $true
                break
            }
            catch {
                continue
            }
        }
        
        if (!$downloaded) {
            throw "Failed to download binary from all sources"
        }
    }
    
    # Install
    Move-Item $tempFile $targetPath -Force
    Write-Success "Installed to: $targetPath"
    
    # Add to PATH
    Add-ToPath -Directory $installDir
    
    # Verify
    try {
        $installedVersion = & $targetPath --version 2>$null | Select-Object -First 1
        Write-Success "Version: $installedVersion"
    }
    catch {
        Write-Warn "Could not verify installation"
    }
    
    return $targetPath
}

function Setup-PowerShellProfile {
    if ($NoShell) {
        Write-Info "Skipping PowerShell profile setup"
        return
    }
    
    $profilePath = $PROFILE.CurrentUserAllHosts
    if (!(Test-Path $profilePath)) {
        $profileDir = Split-Path $profilePath -Parent
        if (!(Test-Path $profileDir)) {
            New-Item -ItemType Directory -Path $profileDir -Force | Out-Null
        }
        New-Item -ItemType File -Path $profilePath -Force | Out-Null
    }
    
    $wutProfile = @"

# WUT key bindings
if (Get-Command wut -ErrorAction SilentlyContinue) {
    # Ctrl+Space to open WUT
    Set-PSReadLineKeyHandler -Chord Ctrl+Space -ScriptBlock {
        [Microsoft.PowerShell.PSConsoleReadLine]::Insert('wut ')
        [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()
    }
    
    # Ctrl+G to open WUT with current line
    Set-PSReadLineKeyHandler -Chord Ctrl+G -ScriptBlock {
        `$line = `$null
        `$cursor = `$null
        [Microsoft.PowerShell.PSConsoleReadLine]::GetBufferState([ref]`$line, [ref]`$cursor)
        [Microsoft.PowerShell.PSConsoleReadLine]::RevertLine()
        [Microsoft.PowerShell.PSConsoleReadLine]::Insert("wut `$line")
        [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()
    }
}
"@
    
    $profileContent = Get-Content $profilePath -Raw -ErrorAction SilentlyContinue
    if ($profileContent -notlike "*WUT key bindings*") {
        Add-Content -Path $profilePath -Value $wutProfile
        Write-Success "Added PowerShell integration to profile"
    }
    else {
        Write-Info "PowerShell integration already exists"
    }
}

function Initialize-Wut {
    if ($NoInit) {
        Write-Info "Skipping initialization (-NoInit)"
        return
    }
    
    try {
        Write-Info "Running quick initialization..."
        & $script:Binary init --quick 2>$null
        Write-Success "Initialization complete"
    }
    catch {
        Write-Warn "Initialization failed, you can run 'wut init' later"
    }
}

function Uninstall-Wut {
    Write-Info "Uninstalling WUT..."
    
    $found = $false
    
    # Find and remove binary from PATH locations
    $pathDirs = $env:PATH -split ';'
    foreach ($dir in $pathDirs) {
        $binaryPath = Join-Path $dir "$($script:Binary).exe"
        if (Test-Path $binaryPath) {
            Remove-Item $binaryPath -Force
            Write-Success "Removed: $binaryPath"
            $found = $true
            
            # Remove from PATH
            Remove-FromPath -Directory $dir
        }
    }
    
    # Check common locations
    $commonPaths = @(
        (Join-Path $env:ProgramFiles "WUT\$($script:Binary).exe"),
        (Join-Path $env:LOCALAPPDATA "WUT\$($script:Binary).exe"),
        (Join-Path $env:USERPROFILE "WUT\$($script:Binary).exe")
    )
    
    foreach ($path in $commonPaths) {
        if (Test-Path $path) {
            Remove-Item $path -Force
            Write-Success "Removed: $path"
            $found = $true
        }
    }
    
    # Remove config
    $configDir = Join-Path $env:USERPROFILE ".config\wut"
    if (Test-Path $configDir) {
        $response = Read-Host "Remove configuration directory? [y/N]"
        if ($response -match '^[Yy]$') {
            Remove-Item $configDir -Recurse -Force
            Write-Success "Removed config: $configDir"
        }
    }
    
    if ($found) {
        Write-Success "WUT has been uninstalled"
    }
    else {
        Write-Warn "WUT not found"
    }
}

function Show-Usage {
    Write-Host @"
Usage: install.ps1 [OPTIONS]

Options:
    -Version VERSION      Install specific version (default: latest)
    -InstallDir DIR       Install to specific directory
    -NoInit               Skip running 'wut init --quick'
    -NoShell              Skip PowerShell profile integration
    -Force                Force overwrite existing installation
    -Uninstall            Uninstall WUT
    -Help                 Show this help message

Examples:
    # Default install (latest version)
    irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex

    # Install specific version
    & ([scriptblock]::Create((irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1))) -Version "v1.0.0"
"@
}

# Main
function Main {
    if ($Help) {
        Show-Usage
        return
    }
    
    Write-Header
    
    if ($Uninstall) {
        Uninstall-Wut
        return
    }
    
    # Check execution policy
    $execPolicy = Get-ExecutionPolicy
    if ($execPolicy -eq "Restricted") {
        Write-Warn "PowerShell execution policy is Restricted"
        Write-Info "Run: Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser"
    }
    
    # Suggest better installation methods first
    Write-Info "Checking for better installation methods..."
    
    # Check if winget is available
    $wingetAvailable = $null -ne (Get-Command winget -ErrorAction SilentlyContinue)
    
    # Check if running from local file (installer mode)
    $isLocalFile = $MyInvocation.MyCommand.Path -and (Test-Path $MyInvocation.MyCommand.Path)
    
    if (!$isLocalFile -and $wingetAvailable -and $Version -eq "latest") {
        Write-Host ""
        Write-Host "$($Colors.Cyan)💡 Recommendation:$($Colors.NC)"
        Write-Host "   This script downloads and installs from GitHub releases."
        Write-Host ""
        Write-Host "$($Colors.Green)Better options available:$($Colors.NC)"
        Write-Host ""
        Write-Host "   1. WinGet (Recommended - auto-updates):"
        Write-Host "      winget install thirawat27.wut"
        Write-Host ""
        Write-Host "   2. Download installer from GitHub:"
        Write-Host "      https://github.com/$script:Repo/releases"
        Write-Host ""
        Write-Host "$($Colors.Yellow)Continue with script installation? [Y/n]$($Colors.NC) " -NoNewline
        $response = Read-Host
        if ($response -match '^[Nn]$') {
            Write-Host ""
            Write-Info "Cancelled. Use one of the recommended methods above."
            return
        }
        Write-Host ""
    }
    
    try {
        # Install
        $installedPath = Install-Wut -Version $Version -InstallDir $InstallDir
        
        # Setup shell integration
        Setup-PowerShellProfile
        
        # Initialize
        Initialize-Wut
        
        # Success message
        Write-Host ""
        Write-Host "$($Colors.Green)$($Colors.Bold)✓ Installation complete!$($Colors.NC)"
        Write-Host ""
        Write-Host "Quick start:"
        Write-Host "  wut --help       Show help"
        Write-Host "  wut suggest      Get command suggestions"
        Write-Host "  wut fix 'gti'    Fix typos"
        Write-Host ""
        Write-Host "PowerShell shortcuts:"
        Write-Host "  Ctrl+Space       Open WUT"
        Write-Host "  Ctrl+G           Open WUT with current line"
        Write-Host ""
        Write-Host "Restart PowerShell to apply all changes."
    }
    catch {
        Write-Error $_.Exception.Message
        exit 1
    }
}

# Run
Main
