# WUT - Command Helper | Windows Installer
# Usage:  irm https://raw.githubusercontent.com/thirawat27/wut/main/scripts/install.ps1 | iex

function Install-WUT {
    param(
        [string]$Version = "latest",
        [string]$InstallDir = "$env:USERPROFILE\.local\bin",
        [switch]$NoInit,
        [switch]$NoShell,
        [switch]$Force
    )

    $ErrorActionPreference = "Stop"
    $ProgressPreference = "SilentlyContinue"

    $repo = "thirawat27/wut"
    $api  = "https://api.github.com/repos/$repo"
    $base = "https://github.com/$repo"
    $tmp  = Join-Path $env:TEMP "wut-install-$([guid]::NewGuid().ToString('N').Substring(0,8))"

    # --- helpers ---
    $c = $Host.UI.SupportsVirtualTerminal
    function ok   ($m) { if($c){Write-Host " [OK] " -F Green -N}else{Write-Host " [OK] " -N}; Write-Host $m }
    function err  ($m) { if($c){Write-Host " [ERR] " -F Red -N}else{Write-Host " [ERR] " -N}; Write-Host $m }
    function info ($m) { if($c){Write-Host " [>] " -F Cyan -N}else{Write-Host " [>] " -N}; Write-Host $m }
    function warn ($m) { if($c){Write-Host " [!] " -F Yellow -N}else{Write-Host " [!] " -N}; Write-Host $m }

    function banner {
        Write-Host ""
        if($c){ Write-Host "  WUT - Command Helper  |  Windows Installer" -F Cyan }
        else  { Write-Host "  WUT - Command Helper  |  Windows Installer" }
        Write-Host ""
    }

    # --- detect platform ---
    function Get-Arch {
        switch ($env:PROCESSOR_ARCHITECTURE) {
            "AMD64" { "x86_64" }
            "ARM64" { "arm64" }
            "x86"   { "i386" }
            default { "x86_64" }
        }
    }

    # --- resolve version ---
    function Resolve-Ver {
        if ($Version -ne "latest") { return $Version }
        try {
            $r = Invoke-RestMethod "$api/releases/latest" -TimeoutSec 15
            return $r.tag_name
        } catch {
            err "Cannot fetch latest version from GitHub"
            return $null
        }
    }

    # --- download ---
    function Get-File ($url, $out) {
        try {
            Invoke-WebRequest -Uri $url -OutFile $out -UseBasicParsing -TimeoutSec 120 | Out-Null
            return $true
        } catch { return $false }
    }

    # --- add to user PATH ---
    function Add-UserPath ($dir) {
        $p = [Environment]::GetEnvironmentVariable("PATH","User")
        if ($p -and $p.Split(';') -contains $dir) { return }
        $np = if($p){"$p;$dir"}else{$dir}
        [Environment]::SetEnvironmentVariable("PATH",$np,"User")
        if ($env:PATH -notlike "*$dir*") { $env:PATH += ";$dir" }
        ok "Added to PATH (permanent)"
    }

    # --- main ---
    banner

    $arch = Get-Arch
    $platform = "Windows_$arch"
    info "Platform: $platform"

    # resolve version
    $ver = Resolve-Ver
    if (-not $ver) { return }
    info "Version:  $ver"

    # prepare dirs
    @($InstallDir, "$env:USERPROFILE\.config\wut", "$env:USERPROFILE\.wut\data", "$env:USERPROFILE\.wut\logs", $tmp) | ForEach-Object {
        if (!(Test-Path $_)) { New-Item -ItemType Directory -Path $_ -Force | Out-Null }
    }

    $dest = Join-Path $InstallDir "wut.exe"

    # check existing
    if ((Test-Path $dest) -and !$Force) {
        $ev = try { & $dest --version 2>$null | Select-Object -First 1 } catch { "unknown" }
        warn "Already installed: $ev  (use -Force to overwrite)"
    }

    # --- try package managers first ---
    $installed = $false

    # WinGet
    if (!$installed -and (Get-Command winget -EA SilentlyContinue)) {
        info "Trying WinGet..."
        try {
            winget install "$repo" --accept-source-agreements --accept-package-agreements --silent 2>&1 | Out-Null
            # refresh PATH
            $env:PATH = [Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [Environment]::GetEnvironmentVariable("PATH","User")
            if (Get-Command wut -EA SilentlyContinue) {
                ok "Installed via WinGet"
                $installed = $true
            }
        } catch {}
    }

    # Scoop
    if (!$installed -and (Get-Command scoop -EA SilentlyContinue)) {
        info "Trying Scoop..."
        try {
            scoop install wut 2>&1 | Out-Null
            if (Get-Command wut -EA SilentlyContinue) {
                ok "Installed via Scoop"
                $installed = $true
            }
        } catch {}
    }

    # Direct download
    if (!$installed) {
        $vn = $ver -replace '^v',''
        $archive = "wut_${vn}_${platform}.zip"
        $url = "$base/releases/download/$ver/$archive"
        $zipPath = Join-Path $tmp $archive

        info "Downloading $archive ..."
        if (!(Get-File $url $zipPath)) {
            err "Download failed: $url"

            # fallback: build from source
            if (Get-Command go -EA SilentlyContinue) {
                info "Building from source..."
                try {
                    $env:CGO_ENABLED = "0"
                    $srcDir = Join-Path $tmp "src"
                    if (Get-Command git -EA SilentlyContinue) {
                        git clone --depth 1 "$base.git" $srcDir 2>&1 | Out-Null
                    } else {
                        Get-File "$base/archive/refs/heads/main.zip" (Join-Path $tmp "src.zip")
                        Expand-Archive (Join-Path $tmp "src.zip") $tmp -Force
                        Rename-Item (Join-Path $tmp "wut-main") $srcDir
                    }
                    Push-Location $srcDir
                    go build -ldflags="-s -w" -o $dest . 2>&1 | Out-Null
                    Pop-Location
                    ok "Built from source"
                    $installed = $true
                } catch {
                    err "Build failed: $_"
                }
            } else {
                err "No Go compiler found. Cannot build from source."
            }

            if (!$installed) {
                Remove-Item $tmp -Recurse -Force -EA SilentlyContinue
                return
            }
        } else {
            # extract
            info "Extracting..."
            Expand-Archive -Path $zipPath -DestinationPath $tmp -Force

            $bin = Join-Path $tmp "wut.exe"
            if (!(Test-Path $bin)) {
                $found = Get-ChildItem $tmp -Filter "wut.exe" -Recurse | Select-Object -First 1
                if ($found) { $bin = $found.FullName } else { err "wut.exe not found in archive"; return }
            }

            if (Test-Path $dest) { Move-Item $dest "$dest.bak" -Force -EA SilentlyContinue }
            Copy-Item $bin $dest -Force
            ok "Installed: $dest"
            $installed = $true
        }

        # ensure PATH
        Add-UserPath $InstallDir
    }

    # cleanup temp
    Remove-Item $tmp -Recurse -Force -EA SilentlyContinue

    # --- find wut ---
    $env:PATH = [Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [Environment]::GetEnvironmentVariable("PATH","User")
    $wut = (Get-Command wut -EA SilentlyContinue).Source
    if (!$wut -and (Test-Path $dest)) { $wut = $dest }

    if (!$wut) {
        err "Installation failed — wut not found"
        return
    }

    # verify
    $v = try { & $wut --version 2>$null | Select-Object -First 1 } catch { "installed" }
    ok "Verified: $v"

    # --- shell integration ---
    if (!$NoShell) {
        info "Setting up shell integration..."
        try { & $wut install --shell powershell 2>&1 | Out-Null; ok "PowerShell integration done" } catch { warn "Shell integration skipped" }
        @("bash","zsh","nu") | ForEach-Object {
            if (Get-Command $_ -EA SilentlyContinue) {
                try { & $wut install --shell $_ 2>&1 | Out-Null } catch {}
            }
        }
    }

    # --- auto init ---
    if (!$NoInit) {
        info "Initializing..."
        try { & $wut init --quick 2>&1 | Out-Null; ok "Initialization complete" } catch { warn "Init skipped (run 'wut init' manually)" }
    }

    # --- done ---
    Write-Host ""
    if($c){ Write-Host "  WUT is ready!" -F Green } else { Write-Host "  WUT is ready!" }
    Write-Host ""
    Write-Host "  Quick start:"
    Write-Host "    wut suggest 'git push'     # Get suggestions"
    Write-Host "    wut explain 'git rebase'   # Explain a command"
    Write-Host "    wut fix 'gti status'       # Fix typos"
    Write-Host "    wut --help                 # All commands"
    Write-Host ""

    if (!(Get-Command wut -EA SilentlyContinue)) {
        warn "Restart your terminal to activate PATH changes."
    }
}

Install-WUT @args
