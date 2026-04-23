param(
    [Parameter(Mandatory = $true)]
    [string]$BinaryPath,

    [Parameter(Mandatory = $true)]
    [string]$OutputDir
)

$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$bundleRoot = Join-Path $repoRoot $OutputDir

New-Item -ItemType Directory -Force -Path $bundleRoot | Out-Null

function Copy-IfExists {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Source,

        [Parameter(Mandatory = $true)]
        [string]$Destination
    )

    if (Test-Path -LiteralPath $Source) {
        $destinationDir = Split-Path -Parent $Destination
        if ($destinationDir) {
            New-Item -ItemType Directory -Force -Path $destinationDir | Out-Null
        }

        Copy-Item -LiteralPath $Source -Destination $Destination -Recurse -Force
    }
}

Copy-IfExists -Source (Join-Path $repoRoot $BinaryPath) -Destination (Join-Path $bundleRoot (Split-Path -Leaf $BinaryPath))
Copy-IfExists -Source (Join-Path $repoRoot "LICENSE") -Destination (Join-Path $bundleRoot "LICENSE")
Copy-IfExists -Source (Join-Path $repoRoot "README.md") -Destination (Join-Path $bundleRoot "README.md")
Copy-IfExists -Source (Join-Path $repoRoot "CONTRIBUTING.md") -Destination (Join-Path $bundleRoot "CONTRIBUTING.md")
Copy-IfExists -Source (Join-Path $repoRoot "scripts\install.sh") -Destination (Join-Path $bundleRoot "scripts\install.sh")
Copy-IfExists -Source (Join-Path $repoRoot "scripts\install.ps1") -Destination (Join-Path $bundleRoot "scripts\install.ps1")
