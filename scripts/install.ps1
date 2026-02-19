# WUT Installation Script for Windows
# Supports: Windows PowerShell 5.1, PowerShell 7+, Windows Terminal
# Administrator rights NOT required

[CmdletBinding()]
param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:USERPROFILE\.local\bin",
    [string]$ConfigDir = "$env:USERPROFILE\.config\wut",
    [switch]$NoShellIntegration,
    [switch]$Force
)

# Error handling
$ErrorActionPreference = "Stop"

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

function Write-Success($Message) { Write-Color "[OK] " "Green"; Write-Color "$Message`n" }
function Write-Error($Message) { Write-Color "[ERR] " "Red"; Write-Color "$Message`n" }
function Write-Info($Message) { Write-Color "[INFO] " "Cyan"; Write-Color "$Message`n" }
function Write-Warning($Message) { Write-Color "[WARN] " "Yellow"; Write-Color "$Message`n" }
function Write-Step($Message) { Write-Color "[>] " "Blue"; Write-Color "$Message`n" }

function Write-Header {
    Write-Host ""
    Write-Color "╔════════════════════════════════════════════════════════════╗`n" "Cyan"
    Write-Color "║                                                            ║`n" "Cyan"
    Write-Color "║   WUT - AI-Powered Command Helper                          ║`n" "Cyan"
    Write-Color "║   Windows Installation Script                              ║`n" "Cyan"
    Write-Color "║                                                            ║`n" "Cyan"
    Write-Color "╚════════════════════════════════════════════════════════════╝`n" "Cyan"
    Write-Host ""
}

# Detect Windows version and architecture
function Get-Platform {
    $arch = switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { "amd64" }
        "ARM64" { "arm64" }
        "x86"   { "386" }
        default { "amd64" }
    }
    
    $os = "windows"
    
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
        Platform = "$os-$arch"
        WinVersion = $winVersion
        IsWindowsTerminal = $isWindowsTerminal
        SupportsColor = $supportsColor
        IsAdmin = $isAdmin
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
    )
    
    foreach ($dir in $dirs) {
        if (!(Test-Path $dir)) {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
        }
    }
    
    Write-Success "Directories created"
}

# Download binary with progress
function Download-Binary($Platform) {
    Write-Step "Downloading WUT binary for $($Platform.Platform)..."
    
    $binaryName = "wut-$($Platform.Platform).exe"
    
    if ($Version -eq "latest") {
        $downloadUrl = "https://github.com/thirawat27/wut/releases/latest/download/$binaryName"
    } else {
        $downloadUrl = "https://github.com/thirawat27/wut/releases/download/$Version/$binaryName"
    }
    
    $outputPath = Join-Path $InstallDir "wut.exe"
    
    try {
        # Use BITS for better download on Windows
        if (Get-Command Start-BitsTransfer -ErrorAction SilentlyContinue) {
            Start-BitsTransfer -Source $downloadUrl -Destination $outputPath -DisplayName "Downloading WUT"
        } else {
            # Fallback to Invoke-WebRequest
            $ProgressPreference = 'Continue'
            Invoke-WebRequest -Uri $downloadUrl -OutFile $outputPath -UseBasicParsing
        }
        
        Write-Success "Binary downloaded: $outputPath"
    } catch {
        Write-Error "Failed to download: $_"
        throw
    }
}

# Build from source
function Build-FromSource {
    Write-Warning "Binary download failed, attempting to build from source..."
    
    if (!(Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error "Go is required to build from source"
        Write-Info "Download from: https://golang.org/dl/"
        exit 1
    }
    
    Write-Step "Building from source..."
    
    $tempDir = Join-Path $env:TEMP "wut-build-$(Get-Random)"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    
    try {
        Set-Location $tempDir
        
        # Clone repository
        if (Get-Command git -ErrorAction SilentlyContinue) {
            git clone --depth 1 https://github.com/thirawat27/wut.git 2>&1 | Out-Null
        } else {
            # Download zip
            $zipUrl = "https://github.com/thirawat27/wut/archive/refs/heads/main.zip"
            Invoke-WebRequest -Uri $zipUrl -OutFile "wut.zip"
            Expand-Archive -Path "wut.zip" -DestinationPath "."
            Rename-Item -Path "wut-main" -NewName "wut"
        }
        
        Set-Location wut
        
        # Build
        $env:CGO_ENABLED = "0"
        go build -ldflags="-s -w" -o (Join-Path $InstallDir "wut.exe") .
        
        Write-Success "Built from source"
    } finally {
        Set-Location $env:USERPROFILE
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
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
    # Try to find the appropriate profile
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
    
    # Fallback
    return "$env:USERPROFILE\Documents\PowerShell\Microsoft.PowerShell_profile.ps1"
}

# Install shell integration
function Install-ShellIntegration($Platform) {
    if ($NoShellIntegration) {
        return
    }
    
    Write-Step "Installing shell integration..."
    
    $wutPath = Join-Path $InstallDir "wut.exe"
    
    if (!(Test-Path $wutPath)) {
        Write-Warning "WUT binary not found, skipping shell integration"
        return
    }
    
    try {
        # Detect shell type
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
            $ver = & $wutPath --version 2>&1
            Write-Success "Installation verified: $ver"
        } catch {
            Write-Warning "Installation verification had issues"
        }
    } else {
        Write-Error "Installation verification failed - binary not found"
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

# Main installation
function Install-WUT {
    Write-Header
    
    $platform = Get-Platform
    Print-SystemInfo $platform
    
    # Try package managers first
    $installed = $false
    if (!$Force) {
        $installed = Install-ViaWinGet
        if (!$installed) {
            $installed = Install-ViaScoop
        }
    }
    
    if (!$installed) {
        Initialize-Directories
        
        # Try downloading, fallback to building
        try {
            Download-Binary $platform
        } catch {
            Build-FromSource
        }
        
        Add-ToPath
        Install-ShellIntegration $platform
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
Install-WUT
