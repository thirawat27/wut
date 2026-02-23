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
    [switch]$Uninstall,
    [switch]$Help
)

# Configuration
$script:Repo = "thirawat27/wut"
$script:Binary = "wut"
$script:ErrorActionPreference = "Stop"

# Colors for PowerShell
$script:Colors = @{
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
    Write-Host "$($script:Colors.Cyan)$($script:Colors.Bold) _    _ _____ _____$($script:Colors.NC)"
    Write-Host "$($script:Colors.Cyan)$($script:Colors.Bold)| |  | |_   _|  __ \$($script:Colors.NC)"
    Write-Host "$($script:Colors.Cyan)$($script:Colors.Bold)| |  | | | | | |  | |$($script:Colors.NC)"
    Write-Host "$($script:Colors.Cyan)$($script:Colors.Bold)| |  | | | | | |  | |$($script:Colors.NC)"
    Write-Host "$($script:Colors.Cyan)$($script:Colors.Bold)| |__| |_| |_| |__| |$($script:Colors.NC)"
    Write-Host "$($script:Colors.Cyan)$($script:Colors.Bold) \____/|_____|_____/$($script:Colors.NC)"
    Write-Host ""
    Write-Host "$($script:Colors.Blue)AI-Powered Command Helper for Windows$($script:Colors.NC)"
    Write-Host ""
}

function Write-Info { 
    param([string]$Message) 
    Write-Host "$($script:Colors.Blue)[INFO]$($script:Colors.NC) $Message" 
}

function Write-Success { 
    param([string]$Message) 
    Write-Host "$($script:Colors.Green)[OK]$($script:Colors.NC) $Message" 
}

function Write-Warn { 
    param([string]$Message) 
    Write-Host "$($script:Colors.Yellow)[WARN]$($script:Colors.NC) $Message" 
}

function Write-Error { 
    param([string]$Message) 
    Write-Host "$($script:Colors.Red)[ERROR]$($script:Colors.NC) $Message" -ForegroundColor Red 
}

function Test-Admin {
    $currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    return $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-Architecture {
    $processor = [System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE")
    
    switch ($processor) {
        "AMD64" { return "x86_64" }
        "ARM64" { return "arm64" }
        "x86" { return "i386" }
        default { 
            if ([System.Environment]::Is64BitOperatingSystem) { return "x86_64" }
            else { return "i386" }
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
    
    # Normalize version (remove 'v' prefix if present for URL construction)
    $versionTag = $Version
    if ($Version -like "v*") {
        $versionTag = $Version.Substring(1)
    }
    
    # Archive name format: wut_<VERSION>_Windows_<ARCH>.zip
    $archiveName = "$($script:Binary)_${versionTag}_Windows_${arch}.zip"
    
    # Download URL
    if ($Version -eq "latest") {
        $downloadUrl = "https://github.com/$($script:Repo)/releases/latest/download/$archiveName"
    }
    else {
        $downloadUrl = "https://github.com/$($script:Repo)/releases/download/$Version/$archiveName"
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
    
    # Create temp directory
    $tempDir = Join-Path $env:TEMP "wut-install-$([Guid]::NewGuid().ToString())"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    
    try {
        # Download archive
        $archivePath = Join-Path $tempDir $archiveName
        Write-Info "Downloading from: $downloadUrl"
        
        try {
            $webClient = New-Object System.Net.WebClient
            $webClient.Headers.Add("User-Agent", "WUT-Installer")
            $webClient.DownloadFile($downloadUrl, $archivePath)
            Write-Success "Download complete"
        }
        catch {
            throw "Failed to download archive from: $downloadUrl"
        }
        
        # Extract archive
        Write-Info "Extracting archive..."
        try {
            # Use Expand-Archive (PowerShell 5.1+)
            Expand-Archive -Path $archivePath -DestinationPath $tempDir -Force
            Write-Success "Extraction complete"
        }
        catch {
            throw "Failed to extract archive: $_"
        }
        
        # Find the binary in extracted files
        $extractedBinary = Get-ChildItem -Path $tempDir -Recurse -Filter "$($script:Binary).exe" | Select-Object -First 1
        
        if (!$extractedBinary) {
            throw "Binary not found in archive"
        }
        
        # Install
        Move-Item $extractedBinary.FullName $targetPath -Force
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
    finally {
        # Cleanup
        if (Test-Path $tempDir) {
            Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
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
    
    $wutProfile = @'

# WUT key bindings
if (Get-Command wut -ErrorAction SilentlyContinue) {
    # Ctrl+Space to open WUT
    Set-PSReadLineKeyHandler -Chord Ctrl+Space -ScriptBlock {
        [Microsoft.PowerShell.PSConsoleReadLine]::Insert('wut ')
        [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()
    }
    
    # Ctrl+G to open WUT with current line
    Set-PSReadLineKeyHandler -Chord Ctrl+G -ScriptBlock {
        $line = $null
        $cursor = $null
        [Microsoft.PowerShell.PSConsoleReadLine]::GetBufferState([ref]$line, [ref]$cursor)
        [Microsoft.PowerShell.PSConsoleReadLine]::RevertLine()
        [Microsoft.PowerShell.PSConsoleReadLine]::Insert("wut $line")
        [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine()
    }
}
'@
    
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
    
    try {
        # Install
        $installedPath = Install-Wut -Version $Version -InstallDir $InstallDir
        
        # Setup shell integration
        Setup-PowerShellProfile
        
        # Initialize
        Initialize-Wut
        
        # Success message
        Write-Host ""
        Write-Host "$($script:Colors.Green)$($script:Colors.Bold)Installation complete!$($script:Colors.NC)"
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
