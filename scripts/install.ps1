# WUT Installation Script for Windows
# Supports: Windows PowerShell 5.1, PowerShell 7+, Windows Terminal
# Administrator rights NOT required
#
# Usage:
#   irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex
#
#   # With options:
#   iex "& { $(irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1) } -Version v1.0.0 -Init"

[CmdletBinding()]
param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:USERPROFILE\.local\bin",
    [string]$ConfigDir = "$env:USERPROFILE\.config\wut",
    [switch]$NoShellIntegration,
    [switch]$Init,
    [switch]$Force,
    [switch]$Verbose
)

# Error handling
$ErrorActionPreference = "Stop"

# Script variables
$script:RepoUrl = "https://github.com/thirawat27/wut"
$script:ApiUrl = "https://api.github.com/repos/thirawat27/wut"
$script:TempDir = Join-Path $env:TEMP ("wut-install-" + [Guid]::NewGuid().ToString().Substring(0, 8))

# Color support detection
$supportsColor = $Host.UI.SupportsVirtualTerminal -or $env:TERM -like "*xterm*"
$isWindowsTerminal = $null -ne $env:WT_SESSION
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

# Color functions
function Write-Color($Text, $Color = "White", $NoNewline = $false) {
    if ($supportsColor) {
        Write-Host $Text -ForegroundColor $Color -NoNewline:$NoNewline
    } else {
        Write-Host $Text -NoNewline:$NoNewline
    }
}

function Write-Success($Message) { Write-Color "[OK] " "Green"; Write-Host "$Message" }
function Write-ErrorMsg($Message) { Write-Color "[ERR] " "Red"; Write-Host "$Message" }
function Write-Info($Message) { Write-Color "[INFO] " "Cyan"; Write-Host "$Message" }
function Write-Warning($Message) { Write-Color "[WARN] " "Yellow"; Write-Host "$Message" }
function Write-Step($Message) { Write-Color "[>] " "Blue"; Write-Host "$Message" }
function Write-VerboseLog($Message) {
    if ($Verbose) {
        Write-Color "[verbose] " "Gray"; Write-Host "$Message"
    }
}

function Write-Header {
    Write-Host ""
    Write-Color "╔════════════════════════════════════════════════════════════╗`n" "Cyan"
    Write-Color "║                                                            ║`n" "Cyan"
    Write-Color "║   WUT - Command Helper                                     ║`n" "Cyan"
    Write-Color "║   Windows Installation Script                              ║`n" "Cyan"
    Write-Color "║                                                            ║`n" "Cyan"
    Write-Color "╚════════════════════════════════════════════════════════════╝`n" "Cyan"
    Write-Host ""
}

# Cleanup function
function Cleanup {
    if (Test-Path $script:TempDir) {
        Remove-Item -Path $script:TempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# Register cleanup on exit
trap { Cleanup }

# Detect Windows version and architecture
function Get-Platform {
    $arch = switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { "x86_64" }
        "ARM64" { "arm64" }
        "x86"   { "i386" }
        default { "x86_64" }
    }
    
    $os = "Windows"
    
    # Detect Windows version
    $winVer = [System.Environment]::OSVersion.Version
    $winVersion = switch ($winVer.Major) {
        10 { if ($winVer.Build -ge 22000) { "Windows 11" } else { "Windows 10" } }
        6  { switch ($winVer.Minor) {
                3 { "Windows 8.1" }
                2 { "Windows 8" }
                1 { "Windows 7" }
                default { "Windows Vista" }
            }}
        default { "Unknown Windows" }
    }
    
    return @{
        OS = $os
        Arch = $arch
        Platform = "${os}_${arch}"
        WinVersion = $winVersion
        IsWindowsTerminal = $isWindowsTerminal
        SupportsColor = $supportsColor
        IsAdmin = $isAdmin
    }
}

# Get latest version from GitHub
function Get-LatestVersion {
    try {
        $response = Invoke-RestMethod -Uri "$script:ApiUrl/releases/latest" -TimeoutSec 30
        return $response.tag_name
    } catch {
        Write-VerboseLog "Failed to get latest version: $_"
        return "latest"
    }
}

# Create directories
function Initialize-Directories {
    Write-Step "Creating directories..."
    
    $dirs = @(
        $InstallDir
        "$env:USERPROFILE\.wut\data"
        "$env:USERPROFILE\.wut\models"
        "$env:USERPROFILE\.wut\logs"
        $ConfigDir
        $script:TempDir
    )
    
    foreach ($dir in $dirs) {
        if (!(Test-Path $dir)) {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
        }
    }
    
    Write-Success "Directories created"
}

# Download file with progress
function Download-File($Url, $OutputPath) {
    Write-VerboseLog "Downloading: $Url"
    Write-VerboseLog "Output: $OutputPath"
    
    try {
        # Use Invoke-WebRequest with progress
        $ProgressPreference = 'Continue'
        Invoke-WebRequest -Uri $Url -OutFile $OutputPath -UseBasicParsing -TimeoutSec 300
        return $true
    } catch {
        Write-VerboseLog "Download failed: $_"
        return $false
    }
}

# Verify checksum
function Verify-Checksum($File, $ChecksumFile) {
    if (!(Test-Path $ChecksumFile)) {
        Write-Warning "Checksum file not found, skipping verification"
        return $true
    }
    
    Write-Step "Verifying checksum..."
    
    $fileName = Split-Path $File -Leaf
    $checksumContent = Get-Content $ChecksumFile
    $expectedChecksum = $checksumContent | Where-Object { $_ -match $fileName } | ForEach-Object { ($_ -split '\s+')[0] }
    
    if (-not $expectedChecksum) {
        Write-Warning "Checksum for $fileName not found in checksums file"
        return $true
    }
    
    $actualChecksum = (Get-FileHash -Path $File -Algorithm SHA256).Hash.ToLower()
    
    if ($expectedChecksum -ne $actualChecksum) {
        Write-ErrorMsg "Checksum verification failed!"
        Write-ErrorMsg "Expected: $expectedChecksum"
        Write-ErrorMsg "Actual:   $actualChecksum"
        return $false
    }
    
    Write-Success "Checksum verified"
    return $true
}

# Download and install
function Download-AndInstall($Platform) {
    $version = $Version
    if ($version -eq "latest") {
        $version = Get-LatestVersion
        Write-Info "Latest version: $version"
    }
    
    # Remove 'v' prefix for archive name
    $versionNoV = $version -replace '^v', ''
    $archiveName = "wut_${versionNoV}_$($Platform.Platform).zip"
    $checksumName = "checksums.txt"
    
    $downloadUrl = "$script:RepoUrl/releases/download/$version/$archiveName"
    $checksumUrl = "$script:RepoUrl/releases/download/$version/$checksumName"
    
    Write-Step "Downloading WUT $version for $($Platform.Platform)..."
    Write-VerboseLog "Archive: $archiveName"
    Write-VerboseLog "URL: $downloadUrl"
    
    $archivePath = Join-Path $script:TempDir $archiveName
    $checksumPath = Join-Path $script:TempDir $checksumName
    
    # Download archive
    if (!(Download-File -Url $downloadUrl -OutputPath $archivePath)) {
        Write-ErrorMsg "Failed to download archive"
        return $false
    }
    
    Write-VerboseLog "Downloaded to: $archivePath"
    
    # Try to download checksums (optional)
    Download-File -Url $checksumUrl -OutputPath $checksumPath | Out-Null
    
    # Verify checksum
    if (Test-Path $checksumPath) {
        if (!(Verify-Checksum -File $archivePath -ChecksumFile $checksumPath)) {
            return $false
        }
    }
    
    # Extract archive
    Write-Step "Extracting archive..."
    try {
        Expand-Archive -Path $archivePath -DestinationPath $script:TempDir -Force
    } catch {
        Write-ErrorMsg "Failed to extract archive: $_"
        return $false
    }
    
    # Find the binary
    $extractedBinary = Join-Path $script:TempDir "wut.exe"
    if (!(Test-Path $extractedBinary)) {
        # Try to find in subdirectory
        $found = Get-ChildItem -Path $script:TempDir -Filter "wut.exe" -Recurse | Select-Object -First 1
        if ($found) {
            $extractedBinary = $found.FullName
        } else {
            Write-ErrorMsg "Binary not found in archive"
            return $false
        }
    }
    
    Write-VerboseLog "Found binary: $extractedBinary"
    
    # Install binary
    $outputPath = Join-Path $InstallDir "wut.exe"
    
    # Check for existing installation
    if ((Test-Path $outputPath) -and !$Force) {
        $existingVersion = & $outputPath --version 2>$null | Select-Object -First 1
        Write-Warning "WUT is already installed: $existingVersion"
        Write-Info "Use -Force to overwrite"
        
        $continue = Read-Host "Continue with installation? [y/N]"
        if ($continue -notmatch '^[Yy]$') {
            Write-Info "Installation cancelled"
            exit 0
        }
    }
    
    # Backup existing binary
    if (Test-Path $outputPath) {
        Move-Item -Path $outputPath -Destination "$outputPath.backup" -Force
        Write-VerboseLog "Backed up existing binary"
    }
    
    # Copy binary
    Copy-Item -Path $extractedBinary -Destination $outputPath -Force
    
    Write-Success "Binary installed: $outputPath"
    return $true
}

# Build from source
function Build-FromSource {
    Write-Warning "Binary download failed, attempting to build from source..."
    
    if (!(Get-Command go -ErrorAction SilentlyContinue)) {
        Write-ErrorMsg "Go is required to build from source"
        Write-Info "Download from: https://golang.org/dl/"
        exit 1
    }
    
    $goVersion = go version
    Write-Info "Found Go: $goVersion"
    
    Write-Step "Building from source..."
    
    $buildDir = Join-Path $env:TEMP ("wut-build-" + [Guid]::NewGuid().ToString().Substring(0, 8))
    New-Item -ItemType Directory -Path $buildDir -Force | Out-Null
    
    try {
        Push-Location $buildDir
        
        # Clone repository
        if (Get-Command git -ErrorAction SilentlyContinue) {
            git clone --depth 1 $script:RepoUrl.git wut 2>&1 | Out-Null
        } else {
            # Download zip
            $zipUrl = "$script:RepoUrl/archive/refs/heads/main.zip"
            Invoke-WebRequest -Uri $zipUrl -OutFile "wut.zip"
            Expand-Archive -Path "wut.zip" -DestinationPath "."
            Rename-Item -Path "wut-main" -NewName "wut"
        }
        
        Set-Location wut
        
        # Build
        $env:CGO_ENABLED = "0"
        $outputPath = Join-Path $InstallDir "wut.exe"
        go build -ldflags="-s -w" -o $outputPath .
        
        Write-Success "Built from source: $outputPath"
    } finally {
        Pop-Location
        Remove-Item -Path $buildDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# Add to PATH
function Add-ToPath {
    Write-Step "Checking PATH..."
    
    $currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    
    if ($currentPath -like "*$InstallDir*") {
        Write-Success "Already in PATH"
        return
    }
    
    Write-Info "Adding to PATH..."
    $newPath = "$currentPath;$InstallDir"
    [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
    
    # Update current session
    $env:PATH = "$env:PATH;$InstallDir"
    
    Write-Success "Added to PATH"
}

# Detect PowerShell profile
function Get-ProfilePath {
    $profiles = @(
        $PROFILE.CurrentUserAllHosts
        $PROFILE.CurrentUserCurrentHost
        $PROFILE.AllUsersAllHosts
        $PROFILE.AllUsersCurrentHost
    )
    
    foreach ($prof in $profiles) {
        if ($prof) {
            $dir = Split-Path $prof -Parent
            if (Test-Path $dir -ErrorAction SilentlyContinue) {
                return $prof
            }
        }
    }
    
    return "$env:USERPROFILE\Documents\PowerShell\Microsoft.PowerShell_profile.ps1"
}

# Install shell integration
function Install-ShellIntegration($Platform) {
    if ($NoShellIntegration) {
        Write-VerboseLog "Skipping shell integration (-NoShellIntegration)"
        return
    }
    
    Write-Step "Installing shell integration..."
    
    $wutPath = Join-Path $InstallDir "wut.exe"
    
    if (!(Test-Path $wutPath)) {
        Write-Warning "WUT binary not found, skipping shell integration"
        return
    }
    
    try {
        if ($Platform.IsWindowsTerminal) {
            Write-Info "Detected: Windows Terminal"
        }
        
        # Install PowerShell integration
        & $wutPath install --shell powershell 2>&1 | Out-Null
        
        # Try to install for other shells if available
        $shells = @("bash", "zsh", "nu")
        foreach ($shell in $shells) {
            if (Get-Command $shell -ErrorAction SilentlyContinue) {
                & $wutPath install --shell $shell 2>&1 | Out-Null
            }
        }
        
        Write-Success "Shell integration installed"
    } catch {
        Write-Warning "Shell integration may need manual configuration: $_"
        Write-Info "Run: wut install --all"
    }
}

# Run initialization
function Run-Init {
    if (!$Init) {
        return
    }
    
    Write-Step "Running initialization..."
    
    $wutPath = Join-Path $InstallDir "wut.exe"
    
    if (Test-Path $wutPath) {
        try {
            & $wutPath init --quick 2>&1 | Out-Null
            Write-Success "Initialization complete"
        } catch {
            Write-Warning "Initialization failed or not available"
            Write-Info "Run manually: wut init"
        }
    }
}

# Install via WinGet if available
function Install-ViaWinGet {
    if (Get-Command winget -ErrorAction SilentlyContinue) {
        Write-Step "Attempting to install via WinGet..."
        try {
            winget install thirawat27.wut --accept-source-agreements --accept-package-agreements
            Write-Success "Installed via WinGet"
            return $true
        } catch {
            Write-Info "WinGet installation failed, falling back to direct download"
            return $false
        }
    }
    return $false
}

# Install via Scoop if available
function Install-ViaScoop {
    if (Get-Command scoop -ErrorAction SilentlyContinue) {
        Write-Step "Attempting to install via Scoop..."
        try {
            scoop install wut
            Write-Success "Installed via Scoop"
            return $true
        } catch {
            Write-Info "Scoop installation failed, falling back to direct download"
            return $false
        }
    }
    return $false
}

# Verify installation
function Verify-Installation {
    Write-Step "Verifying installation..."
    
    $wutPath = Join-Path $InstallDir "wut.exe"
    
    if (Test-Path $wutPath) {
        try {
            $ver = & $wutPath --version 2>&1 | Select-Object -First 1
            Write-Success "Installation verified: $ver"
        } catch {
            Write-Warning "Installation verification had issues"
        }
    } else {
        Write-ErrorMsg "Installation verification failed - binary not found"
    }
}

# Print system information
function Print-SystemInfo($Platform) {
    Write-Info "Platform: $($Platform.Platform)"
    Write-Info "Windows: $($Platform.WinVersion)"
    Write-Info "Terminal: $(if ($Platform.IsWindowsTerminal) { "Windows Terminal" } else { "Console" })"
    Write-Info "Color Support: $($Platform.SupportsColor)"
    Write-Info "Admin: $(if ($Platform.IsAdmin) { "Yes" } else { "No" })"
    Write-Info "Install directory: $InstallDir"
}

# Print usage
function Print-Usage {
    @"
Usage: install.ps1 [OPTIONS]

Options:
  -Version <string>       Install specific version (default: latest)
  -InstallDir <string>    Install to specific directory (default: ~\.local\bin)
  -NoShellIntegration     Skip shell integration
  -Init                   Run 'wut init' after installation
  -Force                  Force overwrite existing installation
  -Verbose                Enable verbose output
  -Help                   Show this help message

Examples:
  # Install latest version
  .\install.ps1

  # Install specific version
  .\install.ps1 -Version v1.0.0

  # Install with initialization
  .\install.ps1 -Init

  # Force reinstall
  .\install.ps1 -Force
"@ | Write-Host
}

# Main installation
function Install-WUT {
    if ($args -contains "-Help" -or $args -contains "--help") {
        Print-Usage
        exit 0
    }
    
    Write-Header
    
    $platform = Get-Platform
    Print-SystemInfo $platform
    
    # Check for existing installation
    $wutPath = Join-Path $InstallDir "wut.exe"
    if ((Test-Path $wutPath) -and !$Force) {
        try {
            $existingVersion = & $wutPath --version 2>$null | Select-Object -First 1
            Write-Info "Existing installation found: $existingVersion"
            Write-Info "Use -Force to overwrite"
            
            $continue = Read-Host "Continue with installation? [y/N]"
            if ($continue -notmatch '^[Yy]$') {
                Write-Info "Installation cancelled"
                exit 0
            }
        } catch {
            Write-VerboseLog "Could not check existing version"
        }
    }
    
    Initialize-Directories
    
    # Try package managers first
    $installed = $false
    if (!$Force) {
        $installed = Install-ViaWinGet
        if (!$installed) {
            $installed = Install-ViaScoop
        }
    }
    
    if (!$installed) {
        # Try downloading, fallback to building
        $downloadSuccess = $false
        try {
            $downloadSuccess = Download-AndInstall $platform
        } catch {
            Write-VerboseLog "Download failed: $_"
        }
        
        if (!$downloadSuccess) {
            Build-FromSource
        }
        
        Add-ToPath
        Install-ShellIntegration $platform
        Run-Init
        Verify-Installation
    }
    
    Write-Header
    Write-Success "WUT installation complete!"
    Write-Host ""
    Write-Info "Quick Start:"
    Write-Host "  wut suggest 'git push'    # Get command suggestions"
    Write-Host "  wut history               # View command history"
    Write-Host "  wut explain 'git rebase'  # Explain a command"
    Write-Host "  wut --help                # Show all commands"
    Write-Host ""
    Write-Info "Please restart your terminal or run:"
    Write-Host "  . ``$(Get-ProfilePath)``"
    Write-Host ""
    
    if ($platform.IsWindowsTerminal) {
        Write-Info "Tip: Windows Terminal detected - full features available!"
    }
}

# Run installation
Install-WUT @args
