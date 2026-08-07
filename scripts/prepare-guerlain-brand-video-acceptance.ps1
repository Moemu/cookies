[CmdletBinding()]
param(
    [switch]$SkipDatabasePreparation,
    [switch]$SkipBackend
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot\local-acceptance-common.ps1"

Assert-LocalCommand docker
Assert-LocalCommand go
Initialize-LocalAcceptanceEnvironment

Push-Location $repoRoot
try {
    if (-not $SkipDatabasePreparation) {
        & "$PSScriptRoot\start-local-database.ps1"
        if ($LASTEXITCODE -ne 0) {
            throw "Local database preparation failed."
        }
    }

    Write-Host "[acceptance] Seeding an isolated Guerlain Project..."
    & go run ./cmd/cookies-seed
    if ($LASTEXITCODE -ne 0) {
        throw "Guerlain acceptance seed failed."
    }

    $mysqlUser = Get-LocalAcceptanceSetting "COOKIES_MYSQL_USER" "cookies"
    $mysqlPassword = Get-LocalAcceptanceSetting "COOKIES_MYSQL_PASSWORD" "cookies_local_development_only"
    $mysqlDatabase = Get-LocalAcceptanceSetting "COOKIES_MYSQL_DATABASE" "cookies"
    $query = @"
SELECT p.id, p.name, b.name, pr.name, p.status, p.project_context_version
FROM projects p
JOIN brands b ON b.organization_id = p.organization_id AND b.id = p.primary_brand_id
JOIN project_products pp ON pp.organization_id = p.organization_id AND pp.project_id = p.id
JOIN products pr ON pr.organization_id = pp.organization_id AND pr.id = pp.product_id
WHERE p.organization_id = 'org_local'
  AND p.id = 'project_guerlain_abeille_royale_acceptance';
"@
    $row = @(& docker compose exec -T -e "MYSQL_PWD=$mysqlPassword" mysql mysql `
        -N -B "-u$mysqlUser" $mysqlDatabase -e $query 2>&1)
    if ($LASTEXITCODE -ne 0 -or $row.Count -ne 1) {
        throw "The isolated Guerlain Project could not be verified: $($row -join [Environment]::NewLine)"
    }

    $routeQuery = @"
SELECT r.model_alias, rr.upstream_model, r.status, c.connection_type, c.status, COUNT(pc.id)
FROM provider_model_routes r
JOIN provider_model_route_revisions rr
  ON rr.id = r.current_revision_id AND rr.route_id = r.id
JOIN provider_connections c
  ON c.id = rr.connection_id AND c.current_revision_id = rr.connection_revision_id
LEFT JOIN provider_credentials pc
  ON pc.connection_id = c.id AND pc.status = 'active'
WHERE r.organization_id IS NULL
  AND r.capability = 'video.generate'
  AND r.model_alias = 'cookies.video.standard'
GROUP BY r.model_alias, rr.upstream_model, r.status, c.connection_type, c.status;
"@
    $routeRows = @(& docker compose exec -T -e "MYSQL_PWD=$mysqlPassword" mysql mysql `
        -N -B "-u$mysqlUser" $mysqlDatabase -e $routeQuery 2>&1)
    $routeRow = $routeRows | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -First 1
    $routeFields = @($routeRow -split "`t")
    if ($LASTEXITCODE -ne 0 -or $routeFields.Count -ne 6 -or
        $routeFields[0] -ne "cookies.video.standard" -or
        $routeFields[1] -ne "doubao-seedance-2-0-fast-260128" -or
        $routeFields[2] -ne "enabled" -or
        $routeFields[3] -ne "adapter_gateway" -or
        $routeFields[4] -ne "enabled" -or
        [int]$routeFields[5] -lt 1) {
        throw "The cookies.video.standard Adapter route is not ready: $($routeRows -join [Environment]::NewLine)"
    }

    if (-not $SkipBackend -and -not (Test-LocalHTTP "http://127.0.0.1:8080/readyz")) {
        & "$PSScriptRoot\start-local-adapter-provider.ps1" -SkipDatabasePreparation
        if ($LASTEXITCODE -ne 0) {
            throw "Local API failed to start."
        }
    }

    Write-Host ""
    Write-Host "Guerlain acceptance foundation is ready:"
    Write-Host "  Project: project_guerlain_abeille_royale_acceptance"
    Write-Host "  Brand:   brand_guerlain / Guerlain"
    Write-Host "  Product: product_guerlain_abeille_royale / Abeille Royale Advanced Youth Watery Oil"
    Write-Host "  Video:   cookies.video.standard -> Seedance 2.0 (Adapter / GlobalRouter)"
    Write-Host "  API:     http://127.0.0.1:8080/readyz"
    Write-Host "  UI:      http://127.0.0.1:5173/projects/project_guerlain_abeille_royale_acceptance/strategy/workspaces"
}
finally {
    Pop-Location
}
