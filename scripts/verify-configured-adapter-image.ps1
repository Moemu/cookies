param(
    [string]$ListenAddress = "127.0.0.1:18081"
)

# Verifies the route created by configure-adapter-image.ps1. It never reads or
# prints a bearer token: the API decrypts the local MySQL credential itself.

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$apiProcess = $null

function Assert-LastExitCode([string]$Message) {
    if ($LASTEXITCODE -ne 0) { throw $Message }
}

Push-Location $repoRoot
try {
    & docker compose up -d mysql | Out-Null
    Assert-LastExitCode "Unable to start local MySQL through docker compose."
    $env:GOTOOLCHAIN = "local"
    & go run ./cmd/cookies-migrate | Out-Null
    Assert-LastExitCode "Database migrations failed."

    & go build -o dist/cookies-api-adapter-verify.exe ./cmd/cookies-api
    Assert-LastExitCode "cookies-api build failed."
    $env:COOKIES_HTTP_ADDR = $ListenAddress
    $apiProcess = Start-Process -FilePath (Resolve-Path "dist/cookies-api-adapter-verify.exe") -PassThru -WindowStyle Hidden

    $origin = "http://$ListenAddress"
    $deadline = (Get-Date).AddSeconds(30)
    do {
        try {
            $ready = Invoke-RestMethod -Method Get -Uri "$origin/readyz" -TimeoutSec 2
            if ($ready.status -eq "ready") { break }
        } catch {
            Start-Sleep -Milliseconds 500
        }
    } while ((Get-Date) -lt $deadline)
    if ($ready.status -ne "ready") { throw "cookies-api did not become ready within 30 seconds." }

    $body = @{
        capability = "image.generate"
        model_alias = "cookies.image.standard"
        project_context_version = 1
        input = @{ prompt = "A minimal blue technology advertising poster, no text"; width = 1024; height = 1024 }
    } | ConvertTo-Json -Depth 4
    $job = Invoke-RestMethod -Method Post -Uri "$origin/platform/v1/projects/project_local/model/jobs" -Headers @{
        "Idempotency-Key" = "adapter-verify-$([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())"
        "Content-Type" = "application/json"
    } -Body $body
    Write-Output "Created Provider job: $($job.id)"

    $deadline = (Get-Date).AddMinutes(4)
    do {
        Start-Sleep -Seconds 2
        $job = Invoke-RestMethod -Method Get -Uri "$origin/platform/v1/projects/project_local/model/jobs/$($job.id)"
        Write-Output "Provider status: $($job.provider_status)"
        if ($job.provider_status -in @("succeeded", "partially_succeeded", "failed", "cancelled", "expired")) { break }
    } while ((Get-Date) -lt $deadline)

    if ($job.provider_status -ne "succeeded" -or $job.project_asset_refs.Count -lt 1) {
        throw "Real Adapter verification failed: status=$($job.provider_status), asset_refs=$($job.project_asset_refs.Count)."
    }
    Write-Output "Verification passed. Generated asset: $($job.project_asset_refs[0].asset_version.asset_id)"
}
finally {
    if ($apiProcess -and -not $apiProcess.HasExited) {
        Stop-Process -Id $apiProcess.Id -Force
    }
    Pop-Location
}
