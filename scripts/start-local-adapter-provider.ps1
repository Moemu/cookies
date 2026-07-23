param(
    [string]$ListenAddress = "127.0.0.1:8080"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$required = @(
    "COOKIES_MYSQL_DSN",
    "COOKIES_PROVIDER_MASTER_KEY",
    "COOKIES_PROVIDER_MASTER_KEY_VERSION",
    "COOKIES_PROVIDER_IMAGE_ADAPTER",
    "COOKIES_PROVIDER_ALLOW_INSECURE_HTTP",
    "COOKIES_PROVIDER_OUTPUT_BUCKET",
    "COOKIES_ENV",
    "COOKIES_BLOB_PROVIDER",
    "COOKIES_FILESYSTEM_BLOB_ROOT",
    "COOKIES_LOCAL_ORGANIZATION_ID",
    "COOKIES_LOCAL_PRINCIPAL_KIND",
    "COOKIES_LOCAL_PRINCIPAL_ID",
    "COOKIES_LOCAL_PROJECT_ID",
    "COOKIES_LOCAL_SCOPES"
)

foreach ($name in $required) {
    $value = [Environment]::GetEnvironmentVariable($name, "User")
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "$name is not configured; run import-clawex-model-providers.ps1 first"
    }
    [Environment]::SetEnvironmentVariable($name, $value, "Process")
}
$env:COOKIES_HTTP_ADDR = $ListenAddress

Push-Location $repoRoot
try {
    & docker compose up -d mysql | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "start MySQL failed"
    }
    $deadline = (Get-Date).AddSeconds(60)
    do {
        $mysqlHealth = & docker inspect --format "{{.State.Health.Status}}" cookies-mysql-1 2>$null
        if ($mysqlHealth -eq "healthy") {
            break
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    if ($mysqlHealth -ne "healthy") {
        throw "MySQL did not become healthy"
    }
    & go build -o dist/cookies-api-adapter.exe ./cmd/cookies-api
    if ($LASTEXITCODE -ne 0) {
        throw "build cookies-api failed"
    }
    $existing = Get-NetTCPConnection `
        -LocalPort ([int]($ListenAddress.Split(":")[-1])) `
        -State Listen `
        -ErrorAction SilentlyContinue
    if ($existing) {
        throw "$ListenAddress is already in use"
    }
    $process = Start-Process `
        -FilePath (Resolve-Path "dist/cookies-api-adapter.exe") `
        -PassThru `
        -WindowStyle Hidden
    Write-Output "cookies_api_pid=$($process.Id)"
    Write-Output "cookies_api_url=http://$ListenAddress"
}
finally {
    Pop-Location
}
