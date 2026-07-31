$ErrorActionPreference = "Stop"
. "$PSScriptRoot\local-acceptance-common.ps1"

Assert-LocalCommand npm.cmd

Push-Location $script:LocalAcceptanceRepoRoot
try {
    if (Test-LocalListeningPort 5173) {
        if (Test-LocalHTTP "http://127.0.0.1:5173") {
            Write-Host "Frontend is already running: http://127.0.0.1:5173"
            return
        }
        throw "Port 5173 is already in use by another process."
    }

    if (-not (Test-Path -LiteralPath (Join-Path $script:LocalAcceptanceRepoRoot "node_modules"))) {
        Write-Host "[frontend] Installing dependencies for the first run..."
        & npm.cmd ci
        if ($LASTEXITCODE -ne 0) {
            throw "Frontend dependency installation failed."
        }
    }

    Write-Host "[frontend] Starting Vite at http://127.0.0.1:5173"
    Write-Host "[frontend] API proxy target: http://127.0.0.1:8080"
    Write-Host "[frontend] Press Ctrl+C to stop."
    & npm.cmd run dev
    if ($LASTEXITCODE -ne 0) {
        throw "Frontend stopped with exit code $LASTEXITCODE."
    }
}
finally {
    Pop-Location
}
