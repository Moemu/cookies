[CmdletBinding()]
param(
    [switch]$SkipDemoSeed
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot\local-acceptance-common.ps1"

Assert-LocalCommand docker
Assert-LocalCommand go
Initialize-LocalAcceptanceEnvironment

Push-Location $script:LocalAcceptanceRepoRoot
try {
    Write-Host "[database] Starting MySQL and waiting for a healthy container..."
    & docker compose up -d --wait mysql
    if ($LASTEXITCODE -ne 0) {
        throw "Unable to start local MySQL."
    }

    Write-Host "[database] Applying database migrations..."
    & go run ./cmd/cookies-migrate
    if ($LASTEXITCODE -ne 0) {
        throw "Database migrations failed."
    }

    if (-not $SkipDemoSeed) {
        Write-Host "[database] Preparing the local organization, user, project, and canonical demo data..."
        & go run ./cmd/cookies-seed
        if ($LASTEXITCODE -ne 0) {
            throw "Local acceptance data seeding failed."
        }
    }

    Assert-SeedTextRoute
    $mysqlPort = Get-LocalAcceptanceSetting "COOKIES_MYSQL_PORT" "3307"
    Write-Host ""
    Write-Host "Database is ready for manual acceptance:"
    Write-Host "  MySQL: 127.0.0.1:$mysqlPort"
    Write-Host "  Text:  $script:SeedTextAlias -> $script:SeedTextModel"
}
finally {
    Pop-Location
}
