param(
    [string]$ListenAddress = "127.0.0.1:18082"
)

# Paid, explicit local smoke test. It reads no Ark credential: the API resolves
# the encrypted credential and immutable route from local MySQL.

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $repoRoot ".env"
$apiProcess = $null

function Get-DotEnvValue([string]$Key, [string]$Fallback = "") {
    $line = Get-Content -LiteralPath $envFile |
        Where-Object { $_ -match "^\s*$([regex]::Escape($Key))\s*=" } |
        Select-Object -First 1
    if ($null -eq $line) { return $Fallback }
    return (($line -split "=", 2)[1].Trim()).Trim('"').Trim("'")
}

function Assert-LastExitCode([string]$Message) {
    if ($LASTEXITCODE -ne 0) { throw $Message }
}

Push-Location $repoRoot
try {
    if ((Get-DotEnvValue "COOKIES_PROVIDER_VIDEO_ADAPTER") -ne "ark_video") {
        throw "Set COOKIES_PROVIDER_VIDEO_ADAPTER=ark_video in .env first."
    }
    & docker compose up -d mysql | Out-Null
    Assert-LastExitCode "Unable to start local MySQL through docker compose."
    $env:GOTOOLCHAIN = "local"
    & go run ./cmd/cookies-migrate | Out-Null
    Assert-LastExitCode "Database migrations failed."
    & go build -o dist/cookies-api-ark-video-verify.exe ./cmd/cookies-api
    Assert-LastExitCode "cookies-api build failed."

    $env:COOKIES_HTTP_ADDR = $ListenAddress
    $apiProcess = Start-Process -FilePath (Resolve-Path "dist/cookies-api-ark-video-verify.exe") -PassThru -WindowStyle Hidden
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

    $loginBody = @{
        username = Get-DotEnvValue "COOKIES_ADMIN_USERNAME" "Admin"
        password = Get-DotEnvValue "COOKIES_ADMIN_PASSWORD" "123456"
    } | ConvertTo-Json
    Invoke-RestMethod -Method Post -Uri "$origin/platform/v1/auth/login" -ContentType "application/json" `
        -Body $loginBody -SessionVariable cookiesSession | Out-Null

    $body = @{
        capability = "video.generate"
        model_alias = "cookies.video.standard"
        project_context_version = 1
        source_system = "creative"
        source_task_id = "seedance_smoke"
        input = @{
            prompt = "Five-second vertical beverage pre-roll, blue and white technology style, centered product, steady camera push, no text"
            duration_seconds = 5
            aspect_ratio = "9:16"
            resolution = "720p"
        }
    } | ConvertTo-Json -Depth 5
    $job = Invoke-RestMethod -WebSession $cookiesSession -Method Post `
        -Uri "$origin/platform/v1/projects/project_local/model/jobs" `
        -Headers @{ "Idempotency-Key" = "ark-video-verify-$([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())" } `
        -ContentType "application/json" -Body $body
    Write-Output "Created video Provider job: $($job.id)"

    $deadline = (Get-Date).AddMinutes(15)
    do {
        Start-Sleep -Seconds 5
        $job = Invoke-RestMethod -WebSession $cookiesSession -Method Get `
            -Uri "$origin/platform/v1/projects/project_local/model/jobs/$($job.id)"
        Write-Output "Provider status: $($job.provider_status), progress: $($job.progress)%"
        if ($job.provider_status -in @("succeeded", "partially_succeeded", "failed", "cancelled", "expired")) { break }
    } while ((Get-Date) -lt $deadline)

    if ($job.provider_status -ne "succeeded" -or $job.project_asset_refs.Count -lt 1) {
        throw "Ark video verification failed: status=$($job.provider_status), asset_refs=$($job.project_asset_refs.Count)."
    }
    Write-Output "Verification passed. Video asset: $($job.project_asset_refs[0].asset_version.asset_id)"
}
finally {
    if ($apiProcess -and -not $apiProcess.HasExited) {
        Stop-Process -Id $apiProcess.Id -Force
    }
    Pop-Location
}
