param(
    [string]$ListenAddress = "127.0.0.1:8080",
    [switch]$Foreground,
    [switch]$SkipDatabasePreparation,
    [switch]$SkipTika
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot\local-acceptance-common.ps1"

Assert-LocalCommand docker
Assert-LocalCommand go
Initialize-LocalAcceptanceEnvironment
$env:COOKIES_HTTP_ADDR = $ListenAddress
$tikaPort = Get-LocalAcceptanceSetting "COOKIES_TIKA_PORT" "9998"
$tikaURL = "http://127.0.0.1:$tikaPort"
if ($SkipTika) {
    Set-LocalAcceptanceProcessSetting "COOKIES_RESEARCH_TIKA_ENABLED" "false"
}
else {
    Set-LocalAcceptanceProcessSetting "COOKIES_RESEARCH_TIKA_ENABLED" "true"
    Set-LocalAcceptanceProcessSetting "COOKIES_RESEARCH_TIKA_BASE_URL" $tikaURL
    Set-LocalAcceptanceProcessSetting "COOKIES_RESEARCH_TIKA_VERSION" "3.2.3.0"
    Set-LocalAcceptanceProcessSetting "COOKIES_RESEARCH_TIKA_TIMEOUT_SECONDS" "120"
    Set-LocalAcceptanceProcessSetting "COOKIES_RESEARCH_TIKA_MAX_OUTPUT_BYTES" "20971520"
}

Push-Location $repoRoot
try {
    if (-not $SkipDatabasePreparation) {
        & "$PSScriptRoot\start-local-database.ps1"
        if ($LASTEXITCODE -ne 0) {
            throw "Local database preparation failed."
        }
    }
    else {
        Assert-SeedTextRoute
    }

    if (-not $SkipTika) {
        Write-Host "[backend] Starting Apache Tika and waiting for its API..."
        & docker compose up -d tika
        if ($LASTEXITCODE -ne 0) {
            throw "Unable to start Apache Tika."
        }
        $tikaDeadline = (Get-Date).AddSeconds(60)
        do {
            if (Test-LocalHTTP "$tikaURL/version") {
                break
            }
            Start-Sleep -Milliseconds 500
        } while ((Get-Date) -lt $tikaDeadline)
        if (-not (Test-LocalHTTP "$tikaURL/version")) {
            throw "Apache Tika did not become ready at $tikaURL."
        }
    }

    $port = [int]($ListenAddress.Split(":")[-1])
    $readyURL = "http://$ListenAddress/readyz"
    if (Test-LocalHTTP $readyURL) {
        Write-Host "Backend is already running: http://$ListenAddress"
        return
    }
    if (Test-LocalListeningPort $port) {
        throw "$ListenAddress is already in use by another process."
    }

    $binaryDirectory = Join-Path $repoRoot ".data\bin"
    New-Item -ItemType Directory -Force -Path $binaryDirectory | Out-Null
    $executable = Join-Path $binaryDirectory "cookies-api-adapter.exe"
    Write-Host "[backend] Building the Go API..."
    & go build -o $executable ./cmd/cookies-api
    if ($LASTEXITCODE -ne 0) {
        throw "Building cookies-api failed."
    }

    $executable = (Resolve-Path $executable).Path
    Write-Host ""
    Write-Host "Backend configuration:"
    Write-Host "  API:   http://$ListenAddress"
    Write-Host "  Text:  $script:SeedTextAlias -> $script:SeedTextModel"
    Write-Host "  Tika:  $(if ($SkipTika) { 'disabled' } else { $tikaURL })"
    Write-Host "  Login: Admin / 123456 (unless overridden by local environment)"

    if ($Foreground) {
        Write-Host "[backend] Press Ctrl+C to stop."
        & $executable
        if ($LASTEXITCODE -ne 0) {
            throw "Backend stopped with exit code $LASTEXITCODE."
        }
        return
    }

    $process = Start-Process `
        -FilePath $executable `
        -WorkingDirectory $repoRoot `
        -WindowStyle Hidden `
        -PassThru
    $deadline = (Get-Date).AddSeconds(30)
    do {
        if (Test-LocalHTTP $readyURL) {
            break
        }
        if ($process.HasExited) {
            throw "Backend exited before it became ready (exit code $($process.ExitCode))."
        }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    if (-not (Test-LocalHTTP $readyURL)) {
        throw "Backend did not become ready at $readyURL."
    }
    Write-Host "Backend started: http://$ListenAddress (PID $($process.Id))"
}
finally {
    Pop-Location
}
