# WUT - Windows Installation Script
# Works with: irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex

function Invoke-WutInstall {
    param(
        [string]$Version    = "latest",
        [string]$InstallDir = "$env:USERPROFILE\.local\bin",
        [switch]$NoInit,
        [switch]$NoShell,
        [switch]$Force,
        [switch]$Help
    )

    # ── Constants ───────────────────────────────────────────────────────────────
    $REPO  = "https://github.com/thirawat27/wut"
    $API   = "https://api.github.com/repos/thirawat27/wut"
    $TMP   = Join-Path $env:TEMP ("wut-" + [guid]::NewGuid().ToString("N").Substring(0,8))

    # ── Helpers ─────────────────────────────────────────────────────────────────
    function Cyan   ($s) { Write-Host $s -ForegroundColor Cyan }
    function Green  ($s) { Write-Host "  [+] $s" -ForegroundColor Green }
    function Yellow ($s) { Write-Host "  [!] $s" -ForegroundColor Yellow }
    function Red    ($s) { Write-Host "  [x] $s" -ForegroundColor Red }
    function Step   ($s) { Write-Host "`n  --> $s" -ForegroundColor Blue }
    function Info   ($s) { Write-Host "      $s" }

    function Show-Banner {
        Cyan ""
        Cyan "  ╔══════════════════════════════════════════╗"
        Cyan "  ║   WUT - Command Helper                   ║"
        Cyan "  ║   Windows Installer                      ║"
        Cyan "  ╚══════════════════════════════════════════╝"
        Cyan ""
    }

    function Show-Help {
        Write-Host @"
WUT Installer

USAGE
  irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex

  With options (must use scriptblock form):
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1))) [OPTIONS]

OPTIONS
  -Version <tag>      Specific version to install  (default: latest)
  -InstallDir <path>  Where to install wut.exe     (default: ~\.local\bin)
  -NoInit             Skip automatic 'wut init'
  -NoShell            Skip shell integration setup
  -Force              Overwrite existing install
  -Help               Show this help
"@
    }

    function Get-Arch {
        switch ($env:PROCESSOR_ARCHITECTURE) {
            "AMD64" { "x86_64" }
            "ARM64" { "arm64"  }
            "x86"   { "i386"   }
            default { "x86_64" }
        }
    }

    function Get-LatestTag {
        try {
            (Invoke-RestMethod "$API/releases/latest" -TimeoutSec 15).tag_name
        } catch {
            throw "Cannot reach GitHub API. Check your internet connection."
        }
    }

    function Download-Binary ($tag) {
        $arch    = Get-Arch
        $ver     = $tag -replace '^v',''
        $archive = "wut_${ver}_Windows_${arch}.zip"
        $url     = "$REPO/releases/download/$tag/$archive"
        $dest    = Join-Path $TMP $archive

        Step "Downloading wut $tag ($arch)..."
        Info $url

        try {
            $prev = $ProgressPreference
            $ProgressPreference = 'SilentlyContinue'
            Invoke-WebRequest $url -OutFile $dest -UseBasicParsing -TimeoutSec 120
            $ProgressPreference = $prev
        } catch {
            throw "Download failed: $_"
        }

        Step "Extracting..."
        Expand-Archive $dest -DestinationPath $TMP -Force

        $bin = Get-ChildItem $TMP -Filter "wut.exe" -Recurse | Select-Object -First 1
        if (-not $bin) { throw "wut.exe not found in archive" }
        return $bin.FullName
    }

    function Build-FromSource {
        Step "Building from source (Go required)..."
        if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
            throw "Go not found. Install from https://golang.org/dl/ and retry."
        }
        $src = Join-Path $TMP "src"
        New-Item -ItemType Directory $src -Force | Out-Null
        if (Get-Command git -ErrorAction SilentlyContinue) {
            git clone --depth 1 "$REPO.git" $src 2>&1 | Out-Null
        } else {
            $zip = Join-Path $TMP "main.zip"
            Invoke-WebRequest "$REPO/archive/refs/heads/main.zip" -OutFile $zip
            Expand-Archive $zip -DestinationPath $TMP -Force
            Rename-Item (Join-Path $TMP "wut-main") $src
        }
        $out = Join-Path $TMP "wut.exe"
        Push-Location $src
        $env:CGO_ENABLED = "0"
        go build -ldflags="-s -w" -o $out .
        Pop-Location
        return $out
    }

    function Find-Wut {
        # Check PATH first
        $cmd = Get-Command wut -ErrorAction SilentlyContinue
        if ($cmd) { return $cmd.Source }
        # Then check install dir
        $direct = Join-Path $InstallDir "wut.exe"
        if (Test-Path $direct) { return $direct }
        return $null
    }

    function Add-ToPath {
        $current = [Environment]::GetEnvironmentVariable("PATH","User")
        if ($current -notlike "*$InstallDir*") {
            [Environment]::SetEnvironmentVariable("PATH", "$current;$InstallDir", "User")
            $env:PATH = "$env:PATH;$InstallDir"
            Green "Added $InstallDir to PATH"
        } else {
            Green "PATH already includes $InstallDir"
        }
    }

    function Setup-Shell ($wut) {
        if ($NoShell) { return }
        Step "Setting up shell integration..."
        try {
            & $wut install --shell powershell 2>&1 | Out-Null
            Green "PowerShell integration installed"
            # Also try bash/zsh if WSL or Git Bash present
            foreach ($sh in @("bash","zsh","nu")) {
                if (Get-Command $sh -ErrorAction SilentlyContinue) {
                    & $wut install --shell $sh 2>&1 | Out-Null
                    Green "$sh integration installed"
                }
            }
        } catch {
            Yellow "Shell integration: $_ — run 'wut install' later"
        }
    }

    function Run-Init ($wut) {
        if ($NoInit) { return }
        Step "Running 'wut init --quick'..."
        try {
            & $wut init --quick
        } catch {
            Yellow "Init skipped: $_ — run 'wut init' manually"
        }
    }

    function Cleanup {
        Remove-Item $TMP -Recurse -Force -ErrorAction SilentlyContinue
    }

    # ── Main ────────────────────────────────────────────────────────────────────
    if ($Help) { Show-Help; return }

    $ErrorActionPreference = "Stop"
    Show-Banner

    # Resolve version
    if ($Version -eq "latest") {
        Step "Fetching latest version..."
        $Version = Get-LatestTag
        Green "Latest: $Version"
    }

    # Create install dir
    New-Item -ItemType Directory -Force $InstallDir | Out-Null
    New-Item -ItemType Directory -Force $TMP        | Out-Null

    # Check existing install
    $existing = Find-Wut
    if ($existing -and -not $Force) {
        $ev = (& $existing --version 2>$null) | Select-Object -First 1
        Yellow "WUT already installed: $ev"
        $ans = Read-Host "      Reinstall? [y/N]"
        if ($ans -notmatch '^[Yy]') {
            Info "Cancelled."; Cleanup; return
        }
    }

    # Get binary
    $bin = $null
    try   { $bin = Download-Binary $Version }
    catch {
        Yellow "Binary download failed: $_"
        $bin = Build-FromSource
    }

    # Install
    Step "Installing to $InstallDir..."
    $dest = Join-Path $InstallDir "wut.exe"
    if (Test-Path $dest) {
        Move-Item $dest "$dest.bak" -Force
    }
    Copy-Item $bin $dest -Force
    Green "Installed: $dest"

    # PATH
    Add-ToPath

    # Verify
    Step "Verifying..."
    $wut = Find-Wut
    if (-not $wut) {
        Red "Binary not found after install — open a new terminal and try 'wut --version'"
        Cleanup; return
    }
    $ver = (& $wut --version 2>&1) | Select-Object -First 1
    Green "Verified: $ver"

    # Shell + Init (the key part — happens automatically)
    Setup-Shell $wut
    Run-Init    $wut

    Cleanup

    # Done
    Cyan ""
    Cyan "  ╔══════════════════════════════════════════╗"
    Cyan "  ║   Installation complete!                 ║"
    Cyan "  ╚══════════════════════════════════════════╝"
    Cyan ""

    if (Get-Command wut -ErrorAction SilentlyContinue) {
        Green "WUT is ready. Try:"
        Info "  wut suggest git"
        Info "  wut explain 'git rebase'"
        Info "  wut --help"
    } else {
        Yellow "Restart your terminal, then run 'wut --help'"
    }
    Cyan ""
}

# Entry point — passes all arguments through, works with both:
#   irm ... | iex
#   .\install.ps1 -Force
Invoke-WutInstall @args
