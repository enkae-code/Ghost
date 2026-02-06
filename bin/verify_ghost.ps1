# GHOST SYSTEM VERIFICATION SCRIPT (Pulse Check)
# ------------------------------------------------
# Starts Kernel, Body, and Brain, then probes the REST API.
# Author: Enkae (enkae.dev@pm.me)

Write-Host ">>> STARTING GHOST SYSTEM..." -ForegroundColor Cyan

# Ensure we are running from project root
$root = Join-Path $PSScriptRoot ".."
Set-Location $root
Write-Host ">>> Working Directory: $(Get-Location)" -ForegroundColor Gray

# 1. Start CONSCIENCE (Kernel)
$kernelProcess = Start-Process -FilePath "go" -ArgumentList "run main.go" -WorkingDirectory "conscience_go" -PassThru -NoNewWindow
Write-Host ">>> [KERNEL] Started (PID: $($kernelProcess.Id))" -ForegroundColor Green
Start-Sleep -Seconds 5

# 2. Start BODY (Sentinel)
$bodyProcess = Start-Process -FilePath "cargo" -ArgumentList "run" -WorkingDirectory "body_rust" -PassThru -NoNewWindow
Write-Host ">>> [BODY] Started (PID: $($bodyProcess.Id))" -ForegroundColor Green
Start-Sleep -Seconds 3

# 3. Start BRAIN (Python)
$pythonPath = ".venv\Scripts\python"
$brainProcess = Start-Process -FilePath $pythonPath -ArgumentList "brain_python/main.py" -PassThru -NoNewWindow
Write-Host ">>> [BRAIN] Started (PID: $($brainProcess.Id))" -ForegroundColor Green
Start-Sleep -Seconds 5

# 4. PROBE THE REST GATEWAY (The "Glass Box" Test)
Write-Host ">>> PROBING REST GATEWAY (localhost:8080)..." -ForegroundColor Yellow

try {
    # Check System State (Should show Active Window from Rust Body)
    $response = Invoke-RestMethod -Uri "http://localhost:8080/v1/system/state" -Method Get
    Write-Host ">>> [RESPONSE] System State:" -ForegroundColor Cyan
    Write-Host ($response | ConvertTo-Json -Depth 2)
}
catch {
    Write-Host ">>> [FAIL] Could not connect to Gateway!" -ForegroundColor Red
    Write-Host $_.Exception.Message
}

# 5. CLEANUP
Write-Host ">>> STOPPING SYSTEM..." -ForegroundColor Magenta
Stop-Process -Id $brainProcess.Id -Force -ErrorAction SilentlyContinue
Stop-Process -Id $bodyProcess.Id -Force -ErrorAction SilentlyContinue
Stop-Process -Id $kernelProcess.Id -Force -ErrorAction SilentlyContinue
Write-Host ">>> [DONE] Verification Complete."
