$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot\local-acceptance-common.ps1"

Push-Location $repoRoot
try {
    if (-not (Test-LocalHTTP "http://127.0.0.1:8080/readyz")) {
        & "$PSScriptRoot\start-local-adapter-provider.ps1"
        if ($LASTEXITCODE -ne 0) {
            throw "Cookies API startup failed"
        }
    }
    else {
        Write-Output "Cookies API is already running"
    }

    if (-not (Test-Path "$repoRoot\node_modules")) {
        & npm.cmd ci
        if ($LASTEXITCODE -ne 0) {
            throw "Frontend dependency installation failed"
        }
    }

    if (-not (Test-LocalListeningPort 5173)) {
        $frontendScript = Join-Path $repoRoot "scripts\start-local-frontend.ps1"
        Start-Process `
            -FilePath "powershell.exe" `
            -ArgumentList @(
                "-NoExit",
                "-NoProfile",
                "-ExecutionPolicy", "Bypass",
                "-File", "`"$frontendScript`""
            ) `
            -WorkingDirectory $repoRoot
        $deadline = (Get-Date).AddSeconds(60)
        do {
            if (Test-LocalHTTP "http://127.0.0.1:5173") {
                break
            }
            Start-Sleep -Milliseconds 500
        } while ((Get-Date) -lt $deadline)
        if (-not (Test-LocalHTTP "http://127.0.0.1:5173")) {
            throw "Frontend did not become ready at http://127.0.0.1:5173 within 60 seconds"
        }
    }
    else {
        Write-Output "Frontend is already running"
    }

    Write-Output ""
    Write-Output "Cookies local stack is ready:"
    Write-Output "  Frontend: http://127.0.0.1:5173"
    Write-Output "  Backend:  http://127.0.0.1:8080"
    $mysqlPort = Get-LocalAcceptanceSetting "COOKIES_MYSQL_PORT" "3307"
    Write-Output "  MySQL:    127.0.0.1:$mysqlPort (Docker)"
    Write-Output "  Text:     $script:SeedTextAlias -> $script:SeedTextModel"
    Write-Output "  Login:    Admin / 123456 (unless overridden)"
}
finally {
    Pop-Location
}
