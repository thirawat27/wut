# WUT Installation Script for Windows
# Supports: Windows PowerShell 5.1, PowerShell 7+, Windows Terminal
# Administrator rights NOT required
#
# Usage:
#   irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex
#
#   # With options (when running as a file):
#   .\install.ps1 -Version v1.0.0 -Force
#
#   # With options (via irm | iex):
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1))) -Version v1.0.0 -Force

# Wrap everything in a function so it works with both `irm | iex` and direct execution
function Install-WUTMain {
    [CmdletBinding()]
    param(
        [string]$Version = "latest",
        [string]$InstallDir = "$env:USERPROFILE\.local\bin",
        [string]$ConfigDir = "$env:USERPROFILE\.config\wut",
        [switch]$NoShellIntegration,
        [switch]$NoInit,
        [switch]$Force,
        [switch]$Help
    )

    # Error handling
    $ErrorActionPreference = "Stop"

    # Script variables
    $RepoUrl = "https://github.com/thirawat27/wut"
    $ApiUrl = "https://api.github.com/repos/thirawat27/wut"
    $TempDir = Join-Path $env:TEMP ("wut-install-" + [Guid]::NewGuid().ToString().Substring(0, 8))
    $WutExecutable = $null

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
    function Write-Warn($Message) { Write-Color "[WARN] " "Yellow"; Write-Host "$Message" }
    function Write-Step($Message) { Write-Color ">>> " "Blue"; Write-Host "$Message" }

    function Write-Header {
        Write-Host ""
        Write-Color "================================================================`n" "Cyan"
        Write-Color "                                                                `n" "Cyan"
        Write-Color "   WUT - Command Helper                                         `n" "Cyan"
        Write-Color "   Windows Installation Script                                  `n" "Cyan"
        Write-Color "                                                                `n" "Cyan"
        Write-Color "================================================================`n" "Cyan"
        Write-Host ""
    }

    # Print usage
    function Show-Usage {
        @"
Usage: install.ps1 [OPTIONS]

Options:
  -Version <string>       Install specific version (default: latest)
  -InstallDir <string>    Install to specific directory (default: ~\.local\bin)
  -NoShellIntegration     Skip shell integration
  -NoInit                 Skip automatic initialization
  -Force                  Force overwrite existing installation
  -Help                   Show this help message

Examples:
  # Install latest version (auto-init, ready to use immediately)
  irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex

  # Install specific version
  .\install.ps1 -Version v1.0.0

  # Force reinstall
  .\install.ps1 -Force
"@ | Write-Host
    }

    # Show help if requested
    if ($Help) {
        Show-Usage
        return
    }

    # Cleanup function
    function Invoke-Cleanup {
        if (Test-Path $TempDir) {
            Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    # Refresh environment variables from registry
    function Invoke-RefreshEnvironment {
        $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
        $machinePath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
        $newPath = ($userPath, $machinePath -split ';' | Where-Object { $_ } | Select-Object -Unique) -join ';'
        $env:PATH = $newPath
    }

    # Find wut executable in common locations
    function Find-WutExecutable {
        $inPath = Get-Command wut -ErrorAction SilentlyContinue
        if ($inPath) {
            return $inPath.Source
        }

        $possiblePaths = @(
            (Join-Path $InstallDir "wut.exe")
            (Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Packages\thirawat27.wut\wut.exe")
            (Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Links\wut.exe")
            "C:\Program Files\WUT\wut.exe"
            "C:\Program Files (x86)\WUT\wut.exe"
            (Join-Path $env:USERPROFILE "scoop\shims\wut.exe")
            (Join-Path $env:USERPROFILE "scoop\apps\wut\current\wut.exe")
        )

        foreach ($path in $possiblePaths) {
            if (Test-Path $path) {
                return $path
            }
        }

        $pathDirs = $env:PATH -split ';'
        foreach ($dir in $pathDirs) {
            if ($dir) {
                $wutPath = Join-Path $dir "wut.exe"
                if (Test-Path $wutPath) {
                    return $wutPath
                }
            }
        }

        return $null
    }

    # Detect Windows version and architecture
    function Get-Platform {
        $arch = switch ($env:PROCESSOR_ARCHITECTURE) {
            "AMD64" { "x86_64" }
            "ARM64" { "arm64" }
            "x86"   { "i386" }
            default { "x86_64" }
        }

        $os = "Windows"

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
            $response = Invoke-RestMethod -Uri "$ApiUrl/releases/latest" -TimeoutSec 30
            return $response.tag_name
        } catch {
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
            $TempDir
        )

        foreach ($dir in $dirs) {
            if (!(Test-Path $dir)) {
                New-Item -ItemType Directory -Path $dir -Force | Out-Null
            }
        }

        Write-Success "Directories created"
    }

    # Download file
    function Invoke-Download($Url, $OutputPath) {
        try {
            $ProgressPreference = 'Continue'
            Invoke-WebRequest -Uri $Url -OutFile $OutputPath -UseBasicParsing -TimeoutSec 300
            return $true
        } catch {
            return $false
        }
    }

    # Verify checksum
    function Test-Checksum($File, $ChecksumFile) {
        if (!(Test-Path $ChecksumFile)) {
            Write-Warn "Checksum file not found, skipping verification"
            return $true
        }

        Write-Step "Verifying checksum..."

        $fileName = Split-Path $File -Leaf
        $checksumContent = Get-Content $ChecksumFile
        $expectedChecksum = $checksumContent | Where-Object { $_ -match $fileName } | ForEach-Object { ($_ -split '\s+')[0] }

        if (-not $expectedChecksum) {
            Write-Warn "Checksum for $fileName not found in checksums file"
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
    function Invoke-DownloadAndInstall($Platform) {
        $ver = $Version
        if ($ver -eq "latest") {
            $ver = Get-LatestVersion
            Write-Info "Latest version: $ver"
        }

        $versionNoV = $ver -replace '^v', ''
        $archiveName = "wut_${versionNoV}_$($Platform.Platform).zip"
        $checksumName = "checksums.txt"

        $downloadUrl = "$RepoUrl/releases/download/$ver/$archiveName"
        $checksumUrl = "$RepoUrl/releases/download/$ver/$checksumName"

        Write-Step "Downloading WUT $ver for $($Platform.Platform)..."

        $archivePath = Join-Path $TempDir $archiveName
        $checksumPath = Join-Path $TempDir $checksumName

        if (!(Invoke-Download -Url $downloadUrl -OutputPath $archivePath)) {
            Write-ErrorMsg "Failed to download archive"
            return $false
        }

        # Try to download checksums (optional)
        Invoke-Download -Url $checksumUrl -OutputPath $checksumPath | Out-Null

        # Verify checksum
        if (Test-Path $checksumPath) {
            if (!(Test-Checksum -File $archivePath -ChecksumFile $checksumPath)) {
                return $false
            }
        }

        # Extract archive
        Write-Step "Extracting archive..."
        try {
            Expand-Archive -Path $archivePath -DestinationPath $TempDir -Force
        } catch {
            Write-ErrorMsg "Failed to extract archive: $_"
            return $false
        }

        # Find the binary
        $extractedBinary = Join-Path $TempDir "wut.exe"
        if (!(Test-Path $extractedBinary)) {
            $found = Get-ChildItem -Path $TempDir -Filter "wut.exe" -Recurse | Select-Object -First 1
            if ($found) {
                $extractedBinary = $found.FullName
            } else {
                Write-ErrorMsg "Binary not found in archive"
                return $false
            }
        }

        # Install binary
        $outputPath = Join-Path $InstallDir "wut.exe"

        # Check for existing installation
        if ((Test-Path $outputPath) -and !$Force) {
            $existingVersion = & $outputPath --version 2>$null | Select-Object -First 1
            Write-Warn "WUT is already installed: $existingVersion"
            Write-Info "Use -Force to overwrite"

            $continue = Read-Host "Continue with installation? [y/N]"
            if ($continue -notmatch '^[Yy]$') {
                Write-Info "Installation cancelled"
                return $false
            }
        }

        # Backup existing binary
        if (Test-Path $outputPath) {
            Move-Item -Path $outputPath -Destination "$outputPath.backup" -Force
        }

        # Copy binary
        Copy-Item -Path $extractedBinary -Destination $outputPath -Force

        Write-Success "Binary installed: $outputPath"

        # Store the executable path
        Set-Variable -Name WutExecutable -Value $outputPath -Scope 1
        return $true
    }

    # Build from source
    function Build-FromSource {
        Write-Warn "Binary download failed, attempting to build from source..."

        if (!(Get-Command go -ErrorAction SilentlyContinue)) {
            Write-ErrorMsg "Go is required to build from source"
            Write-Info "Download from: https://golang.org/dl/"
            return
        }

        $goVersion = go version
        Write-Info "Found Go: $goVersion"

        Write-Step "Building from source..."

        $buildDir = Join-Path $env:TEMP ("wut-build-" + [Guid]::NewGuid().ToString().Substring(0, 8))
        New-Item -ItemType Directory -Path $buildDir -Force | Out-Null

        try {
            Push-Location $buildDir

            if (Get-Command git -ErrorAction SilentlyContinue) {
                git clone --depth 1 "$RepoUrl.git" wut 2>&1 | Out-Null
            } else {
                $zipUrl = "$RepoUrl/archive/refs/heads/main.zip"
                Invoke-WebRequest -Uri $zipUrl -OutFile "wut.zip"
                Expand-Archive -Path "wut.zip" -DestinationPath "."
                Rename-Item -Path "wut-main" -NewName "wut"
            }

            Set-Location wut

            $env:CGO_ENABLED = "0"
            $outputPath = Join-Path $InstallDir "wut.exe"
            go build -ldflags="-s -w" -o $outputPath .

            Write-Success "Built from source: $outputPath"

            Set-Variable -Name WutExecutable -Value $outputPath -Scope 1
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

        # Update current session immediately
        $env:PATH = "$env:PATH;$InstallDir"

        Write-Success "Added to PATH"
    }

    # Install shell integration
    function Install-ShellIntegration($Platform) {
        if ($NoShellIntegration) {
            return
        }

        Write-Step "Installing shell integration..."

        $wutPath = if ($WutExecutable) { $WutExecutable } else { Find-WutExecutable }

        if (-not $wutPath -or !(Test-Path $wutPath)) {
            Write-Warn "WUT binary not found, skipping shell integration"
            return
        }

        try {
            if ($Platform.IsWindowsTerminal) {
                Write-Info "Detected: Windows Terminal"
            }

            & $wutPath install --shell powershell 2>&1 | Out-Null

            $shells = @("bash", "zsh", "nu")
            foreach ($shell in $shells) {
                if (Get-Command $shell -ErrorAction SilentlyContinue) {
                    & $wutPath install --shell $shell 2>&1 | Out-Null
                }
            }

            Write-Success "Shell integration installed"
        } catch {
            Write-Warn "Shell integration may need manual configuration: $_"
            Write-Info "Run: wut install --all"
        }
    }

    # Run initialization (auto by default)
    function Invoke-Init {
        if ($NoInit) {
            return
        }

        Write-Step "Running initialization..."

        $wutPath = if ($WutExecutable) { $WutExecutable } else { Find-WutExecutable }

        if (-not $wutPath -or !(Test-Path $wutPath)) {
            Write-Warn "WUT binary not found, skipping initialization"
            return
        }

        try {
            & $wutPath init --quick 2>&1 | Out-Null
            Write-Success "Initialization complete"
        } catch {
            Write-Warn "Initialization failed or not available"
            Write-Info "Run manually: wut init"
        }
    }

    # Install via WinGet if available
    function Install-ViaWinGet {
        if (Get-Command winget -ErrorAction SilentlyContinue) {
            Write-Step "Attempting to install via WinGet..."
            try {
                winget install thirawat27.wut --accept-source-agreements --accept-package-agreements
                Write-Success "Installed via WinGet"

                Invoke-RefreshEnvironment

                $installedPath = Find-WutExecutable
                if ($installedPath) {
                    Set-Variable -Name WutExecutable -Value $installedPath -Scope 1

                    $wutDir = Split-Path $installedPath -Parent
                    if ($env:PATH -notlike "*$wutDir*") {
                        $env:PATH = "$env:PATH;$wutDir"
                    }
                }

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

                Invoke-RefreshEnvironment

                $installedPath = Find-WutExecutable
                if ($installedPath) {
                    Set-Variable -Name WutExecutable -Value $installedPath -Scope 1
                }

                return $true
            } catch {
                Write-Info "Scoop installation failed, falling back to direct download"
                return $false
            }
        }
        return $false
    }

    # Verify installation
    function Test-Installation {
        Write-Step "Verifying installation..."

        $wutPath = Find-WutExecutable

        if ($wutPath) {
            Set-Variable -Name WutExecutable -Value $wutPath -Scope 1

            try {
                $ver = & $wutPath --version 2>&1 | Select-Object -First 1
                Write-Success "Installation verified: $ver"
                return $true
            } catch {
                Write-Warn "Installation verification had issues"
            }
        } else {
            Write-ErrorMsg "Installation verification failed - binary not found"
        }

        return $false
    }

    # Print system information
    function Show-SystemInfo($Platform) {
        Write-Info "Platform: $($Platform.Platform)"
        Write-Info "Windows: $($Platform.WinVersion)"
        Write-Info "Terminal: $(if ($Platform.IsWindowsTerminal) { "Windows Terminal" } else { "Console" })"
        Write-Info "Color Support: $($Platform.SupportsColor)"
        Write-Info "Admin: $(if ($Platform.IsAdmin) { "Yes" } else { "No" })"
        Write-Info "Install directory: $InstallDir"
    }

    # ===== MAIN LOGIC =====

    Write-Header

    $platform = Get-Platform
    Show-SystemInfo $platform

    # Check for existing installation
    $existingWut = Find-WutExecutable
    if ($existingWut -and !$Force) {
        try {
            $existingVersion = & $existingWut --version 2>$null | Select-Object -First 1
            Write-Info "Existing installation found: $existingVersion"
            Write-Info "Use -Force to overwrite"

            $continue = Read-Host "Continue with installation? [y/N]"
            if ($continue -notmatch '^[Yy]$') {
                Write-Info "Installation cancelled"
                return
            }
        } catch { }
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
        $downloadSuccess = $false
        try {
            $downloadSuccess = Invoke-DownloadAndInstall $platform
        } catch { }

        if (!$downloadSuccess) {
            Build-FromSource
        }

        Add-ToPath
    }

    # Refresh environment to ensure PATH is up to date
    Invoke-RefreshEnvironment

    # Verify installation
    $verified = Test-Installation

    # Auto-install shell integration and auto-init
    if ($verified) {
        Install-ShellIntegration $platform
        Invoke-Init
    }

    Write-Header
    Write-Success "WUT installation complete!"
    Write-Host ""

    # Test if wut is immediately usable
    $accessiblePath = Get-Command wut -ErrorAction SilentlyContinue
    if ($accessiblePath) {
        Write-Success "WUT is ready to use!"
        Write-Host ""
        Write-Info "Quick Start:"
        Write-Host "  wut suggest 'git push'    # Get command suggestions"
        Write-Host "  wut history               # View command history"
        Write-Host "  wut explain 'git rebase'  # Explain a command"
        Write-Host "  wut --help                # Show all commands"
    } else {
        Write-Warn "WUT is installed but requires a fresh terminal session to use."
        Write-Host ""
        Write-Info "Please run one of the following:"
        Write-Host "  1. Close and reopen your terminal"
        Write-Host "  2. Or run: refreshenv"
        Write-Host ""
        Write-Info "After that, you can use:"
        Write-Host "  wut --help"
    }

    Write-Host ""

    if ($platform.IsWindowsTerminal) {
        Write-Info "Tip: Windows Terminal detected - full features available!"
    }

    # Cleanup
    Invoke-Cleanup
}

# Run the installer - this works with both `irm | iex` and `.\install.ps1`
Install-WUTMain @args
