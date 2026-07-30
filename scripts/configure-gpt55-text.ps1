param(
    [string]$Model = "gpt-5.5"
)

# Reuses the encrypted credential behind the current cookies.text.standard
# Adapter route. No API token is read, printed, or written by this script.

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $repoRoot ".env"

function Get-DotEnvValue([string]$Key) {
    if (-not (Test-Path -LiteralPath $envFile)) { return "" }
    $line = Get-Content -LiteralPath $envFile |
        Where-Object { $_ -match "^\s*$([regex]::Escape($Key))\s*=" } |
        Select-Object -Last 1
    if ($null -eq $line) { return "" }
    return (($line -split "=", 2)[1].Trim()).Trim('"').Trim("'")
}

function Set-DotEnvValue([string]$Key, [string]$Value) {
    if (-not (Test-Path -LiteralPath $envFile)) { return }
    $lines = @(Get-Content -LiteralPath $envFile)
    $pattern = "^\s*$([regex]::Escape($Key))\s*="
    $updated = $false
    for ($index = 0; $index -lt $lines.Count; $index++) {
        if ($lines[$index] -match $pattern) {
            $lines[$index] = "$Key=$Value"
            $updated = $true
        }
    }
    if (-not $updated) { $lines += "$Key=$Value" }
    [IO.File]::WriteAllLines($envFile, $lines, [Text.UTF8Encoding]::new($false))
}

Push-Location $repoRoot
try {
    $mysqlPassword = Get-DotEnvValue "COOKIES_MYSQL_PASSWORD"
    if ([string]::IsNullOrWhiteSpace($mysqlPassword)) {
        $mysqlPassword = "cookies_local_development_only"
    }
    $containerID = (& docker compose ps -q mysql).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($containerID)) {
        throw "The local MySQL container is not running."
    }
    $query = "SELECT r.id FROM provider_model_routes r WHERE r.organization_id IS NULL AND r.capability='text.generate' AND r.model_alias='cookies.text.standard' AND r.status='enabled' LIMIT 1"
    $routeID = (& docker exec -e "MYSQL_PWD=$mysqlPassword" $containerID mysql -N -s -u cookies cookies -e $query).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($routeID)) {
        throw "No enabled cookies.text.standard route exists."
    }
    $query = "SELECT rr.connection_id FROM provider_model_routes r JOIN provider_model_route_revisions rr ON rr.id=r.current_revision_id WHERE r.id='$routeID'"
    $connectionID = (& docker exec -e "MYSQL_PWD=$mysqlPassword" $containerID mysql -N -s -u cookies cookies -e $query).Trim()
    $query = "SELECT rr.connection_revision_id FROM provider_model_routes r JOIN provider_model_route_revisions rr ON rr.id=r.current_revision_id WHERE r.id='$routeID'"
    $connectionRevisionID = (& docker exec -e "MYSQL_PWD=$mysqlPassword" $containerID mysql -N -s -u cookies cookies -e $query).Trim()
    if ([string]::IsNullOrWhiteSpace($connectionID) -or [string]::IsNullOrWhiteSpace($connectionRevisionID)) {
        throw "The current Adapter route is incomplete."
    }
    $revisionID = "route_cookies_text_standard_gpt55_r2"
    $sql = @"
START TRANSACTION;
INSERT INTO provider_model_route_revisions (
  id, route_id, revision_number, connection_id, connection_revision_id, upstream_model, constraints_json
)
SELECT '$revisionID', '$routeID', COALESCE(MAX(revision_number), 0) + 1,
  '$connectionID', '$connectionRevisionID', '$Model',
  JSON_OBJECT(
    'api_mode', 'chat_completions',
    'text_response_mode', 'json_schema',
    'max_output_tokens', 8192,
    'output_token_parameter', 'max_completion_tokens',
    'source_provider', 'openai'
  )
FROM provider_model_route_revisions
WHERE route_id = '$routeID'
ON DUPLICATE KEY UPDATE id = VALUES(id);
UPDATE provider_model_routes SET current_revision_id = '$revisionID' WHERE id = '$routeID';
INSERT INTO provider_model_routes (id, organization_id, capability, model_alias, current_revision_id, status)
VALUES ('route_cookies_text_deep_review', NULL, 'text.generate', 'cookies.text.deep_review', NULL, 'enabled')
ON DUPLICATE KEY UPDATE status = 'enabled';
INSERT INTO provider_model_route_revisions (
  id, route_id, revision_number, connection_id, connection_revision_id, upstream_model, constraints_json
) VALUES (
  'route_cookies_text_deep_review_gpt55pro_r1', 'route_cookies_text_deep_review', 1,
  '$connectionID', '$connectionRevisionID', 'gpt-5.5-pro',
  JSON_OBJECT(
    'api_mode', 'responses',
    'text_response_mode', 'json_schema',
    'max_output_tokens', 32768,
    'output_token_parameter', 'max_output_tokens',
    'reasoning_effort', 'high',
    'background', TRUE,
    'poll_interval_ms', 1000,
    'source_provider', 'openai'
  )
)
ON DUPLICATE KEY UPDATE id = VALUES(id);
UPDATE provider_model_routes
SET current_revision_id = 'route_cookies_text_deep_review_gpt55pro_r1'
WHERE id = 'route_cookies_text_deep_review';
COMMIT;
"@
    & docker exec -i -e "MYSQL_PWD=$mysqlPassword" $containerID mysql -u cookies cookies -e $sql
    if ($LASTEXITCODE -ne 0) { throw "Saving the GPT-5.5 route revision failed." }

    Set-DotEnvValue "COOKIES_PROVIDER_TEXT_ADAPTER" "adapter_gateway"
    Set-DotEnvValue "COOKIES_STRATEGY_REAL_PROVIDER_ENABLED" "true"
    Set-DotEnvValue "COOKIES_STRATEGY_TEXT_MODEL_ALIAS" "cookies.text.standard"
    Set-DotEnvValue "COOKIES_STRATEGY_DEEP_REVIEW_MODEL_ALIAS" "cookies.text.deep_review"
    Write-Output "Configured cookies.text.standard -> $Model through Chat Completions."
    Write-Output "Configured cookies.text.deep_review -> gpt-5.5-pro through background Responses."
}
finally {
    Pop-Location
}
