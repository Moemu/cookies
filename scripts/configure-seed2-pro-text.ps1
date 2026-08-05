param(
    [string]$Model = "doubao-seed-2-0-pro-260215"
)

# Reuses the already encrypted company Adapter credential. No token is read
# from command-line arguments, written to .env, or embedded in SQL.

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $repoRoot ".env"

function Get-DotEnvValue([string]$Key) {
    $line = Get-Content -LiteralPath $envFile |
        Where-Object { $_ -match "^\s*$([regex]::Escape($Key))\s*=" } |
        Select-Object -First 1
    if ($null -eq $line) { return "" }
    return (($line -split "=", 2)[1].Trim()).Trim('"').Trim("'")
}

function Set-DotEnvValue([string]$Key, [string]$Value) {
    $content = @(Get-Content -LiteralPath $envFile)
    $pattern = "^\s*$([regex]::Escape($Key))\s*="
    $updated = $false
    $next = foreach ($line in $content) {
        if ($line -match $pattern) {
            "$Key=$Value"
            $updated = $true
        }
        else {
            $line
        }
    }
    if (-not $updated) { $next += "$Key=$Value" }
    Set-Content -LiteralPath $envFile -Value $next -Encoding utf8
}

Push-Location $repoRoot
try {
    if (-not (Test-Path -LiteralPath $envFile)) {
        throw "Missing $envFile. Copy .env.example to .env first."
    }
    $mysqlPassword = Get-DotEnvValue "COOKIES_MYSQL_PASSWORD"
    if ([string]::IsNullOrWhiteSpace($mysqlPassword)) {
        throw "COOKIES_MYSQL_PASSWORD is required in .env."
    }
    & docker compose up -d mysql | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Unable to start local MySQL." }
    & go run ./cmd/cookies-migrate | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Database migrations failed." }

    $connectionID = "connection_adapter_shared"
    $connectionRevisionID = "connection_adapter_shared_r1"
    $routeID = "route_adapter_shared_text_seed2"
    $routeRevisionID = "route_adapter_shared_text_seed2_r1"
    $connection = & docker exec -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -N -s -u cookies cookies -e "SELECT COUNT(*) FROM provider_connections WHERE id='$connectionID' AND current_revision_id='$connectionRevisionID' AND status='enabled'"
    if ($LASTEXITCODE -ne 0 -or [int]$connection -ne 1) {
        throw "The encrypted shared Adapter connection is not configured. Configure the existing image Adapter first."
    }
    $sql = @"
START TRANSACTION;
UPDATE provider_model_routes SET current_revision_id = NULL WHERE id = '$routeID';
DELETE FROM provider_model_route_revisions WHERE route_id = '$routeID';
DELETE FROM provider_model_routes WHERE id = '$routeID';
INSERT INTO provider_model_routes (id, organization_id, capability, model_alias, current_revision_id, status)
VALUES ('$routeID', NULL, 'text.generate', 'cookies.text.standard', NULL, 'enabled');
INSERT INTO provider_model_route_revisions (
  id, route_id, revision_number, connection_id, connection_revision_id, upstream_model, constraints_json
) VALUES (
  '$routeRevisionID', '$routeID', 1, '$connectionID', '$connectionRevisionID', '$Model',
  JSON_OBJECT(
    'text_response_mode', 'prompt_json',
    'max_output_tokens', 8192,
    'output_token_parameter', 'max_tokens',
    'temperature', 0.2
  )
);
UPDATE provider_model_routes SET current_revision_id = '$routeRevisionID' WHERE id = '$routeID';
COMMIT;
"@
    & docker exec -i -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -u cookies cookies -e $sql
    if ($LASTEXITCODE -ne 0) { throw "Saving the Seed-2-pro text route failed." }

    Set-DotEnvValue "COOKIES_PROVIDER_TEXT_ADAPTER" "adapter_gateway"
    Set-DotEnvValue "COOKIES_STRATEGY_REAL_PROVIDER_ENABLED" "true"
    Set-DotEnvValue "COOKIES_STRATEGY_TEXT_MODEL_ALIAS" "cookies.text.standard"
    Set-DotEnvValue "COOKIES_STRATEGY_DEEP_REVIEW_MODEL_ALIAS" "cookies.text.standard"
    Write-Output "Seed-2-pro text route configured through cookies.text.standard."
}
finally {
    Pop-Location
}
