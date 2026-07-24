$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot

function Test-HTTP([string]$url) {
    try {
        $response = Invoke-WebRequest `
            -UseBasicParsing `
            -Uri $url `
            -TimeoutSec 2
        return $response.StatusCode -ge 200 -and $response.StatusCode -lt 300
    }
    catch {
        return $false
    }
}

function Test-ListeningPort([int]$port) {
    return $null -ne (Get-NetTCPConnection `
        -LocalPort $port `
        -State Listen `
        -ErrorAction SilentlyContinue |
        Select-Object -First 1)
}

Push-Location $repoRoot
try {
    if (-not (Test-HTTP "http://127.0.0.1:8080/readyz")) {
        & powershell -NoProfile -ExecutionPolicy Bypass `
            -File "$PSScriptRoot\import-clawex-model-providers.ps1"
        if ($LASTEXITCODE -ne 0) {
            throw "Provider model import failed"
        }
        & powershell -NoProfile -ExecutionPolicy Bypass `
            -File "$PSScriptRoot\start-local-adapter-provider.ps1"
        if ($LASTEXITCODE -ne 0) {
            throw "Cookies API startup failed"
        }
    }
    else {
        Write-Output "Cookies API is already running"
    }

    if (-not (Test-Path "$repoRoot\web\node_modules")) {
        & npm ci --prefix web
        if ($LASTEXITCODE -ne 0) {
            throw "Frontend dependency installation failed"
        }
    }

    if (-not (Test-ListeningPort 5173)) {
        Start-Process `
            -FilePath "cmd.exe" `
            -ArgumentList "/k", "npm run dev --prefix web" `
            -WorkingDirectory $repoRoot
        $deadline = (Get-Date).AddSeconds(30)
        do {
            if (Test-ListeningPort 5173) {
                break
            }
            Start-Sleep -Milliseconds 500
        } while ((Get-Date) -lt $deadline)
        if (-not (Test-ListeningPort 5173)) {
            throw "Frontend did not start on port 5173"
        }
    }
    else {
        Write-Output "Frontend is already running"
    }

    Write-Output ""
    Write-Output "Cookies local stack is ready:"
    Write-Output "  Frontend: http://127.0.0.1:5173"
    Write-Output "  Backend:  http://127.0.0.1:8080"
    $mysqlPort = [Environment]::GetEnvironmentVariable(
        "COOKIES_MYSQL_PORT",
        "User"
    )
    if ([string]::IsNullOrWhiteSpace($mysqlPort)) {
        $mysqlPort = "3307"
    }
    Write-Output "  MySQL:    127.0.0.1:$mysqlPort (Docker)"
}
finally {
    Pop-Location
}
