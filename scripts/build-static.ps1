<#
.SYNOPSIS
    Builds the landing app and copies output to conscience_go/static for kernel to serve.
.DESCRIPTION
    Runs npm run build in apps/landing, then copies dist/* to conscience_go/static/.
    Keeps conscience_go/static/.gitkeep. Run from repo root.
#>

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
if (-not (Test-Path "$repoRoot\apps\landing")) {
    Write-Error "apps/landing not found. Run from repo root or ensure apps/landing exists."
    exit 1
}

Write-Host "[build-static] Building apps/landing..." -ForegroundColor Cyan
Push-Location "$repoRoot\apps\landing"
try {
    npm run build
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    Pop-Location
}

$staticDir = "$repoRoot\conscience_go\static"
$distDir  = "$repoRoot\apps\landing\dist"
if (-not (Test-Path $distDir)) {
    Write-Error "apps/landing/dist not found after build."
    exit 1
}

# Preserve .gitkeep, replace rest of static
$keep = Join-Path $staticDir ".gitkeep"
$keepExists = Test-Path $keep
Get-ChildItem $staticDir -Force | Where-Object { $_.Name -ne ".gitkeep" } | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
Copy-Item -Path "$distDir\*" -Destination $staticDir -Recurse -Force
if (-not $keepExists) { New-Item -Path $keep -ItemType File -Force | Out-Null }

Write-Host "[build-static] Done. conscience_go/static updated." -ForegroundColor Green
