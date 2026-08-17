param(
    [string]$ConnectionID = "connection_adapter_shared",
    [string]$Model = "doubao-seedance-2-0-fast-260128"
)

# Switches the existing cookies.video.standard route to an immutable revision
# backed by the shared Adapter. This script never reads or rewrites credentials.

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $repoRoot ".env"

function Get-DotEnvValue([string]$Key) {
    if (-not (Test-Path -LiteralPath $envFile)) {
        throw "Missing $envFile. Copy .env.example to .env first."
    }
    $line = Get-Content -LiteralPath $envFile |
        Where-Object { $_ -match "^\s*$([regex]::Escape($Key))\s*=" } |
        Select-Object -First 1
    if ($null -eq $line) { return "" }
    return (($line -split "=", 2)[1].Trim()).Trim('"').Trim("'")
}

function Assert-LastExitCode([string]$Message) {
    if ($LASTEXITCODE -ne 0) { throw $Message }
}

if ($ConnectionID -notmatch '^[A-Za-z0-9_-]+$' -or $Model -notmatch '^[A-Za-z0-9._-]+$') {
    throw "Connection ID and model may contain only letters, numbers, dot, underscore, and hyphen."
}

Push-Location $repoRoot
try {
    $mysqlPassword = Get-DotEnvValue "COOKIES_MYSQL_PASSWORD"
    if ([string]::IsNullOrWhiteSpace($mysqlPassword)) {
        throw "COOKIES_MYSQL_PASSWORD is required in .env."
    }

    $connection = & docker exec -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -N -B -u cookies cookies -e @"
SELECT c.current_revision_id, COUNT(pc.id)
FROM provider_connections c
LEFT JOIN provider_credentials pc
  ON pc.connection_id=c.id AND pc.status='active'
  AND pc.active_from <= UTC_TIMESTAMP(6)
  AND (pc.active_until IS NULL OR pc.active_until > UTC_TIMESTAMP(6))
WHERE c.id='$ConnectionID' AND c.connection_type='adapter_gateway' AND c.status='enabled'
GROUP BY c.current_revision_id;
"@
    Assert-LastExitCode "Could not inspect the Adapter connection."
    if ([string]::IsNullOrWhiteSpace($connection)) {
        throw "Enabled adapter_gateway connection '$ConnectionID' was not found."
    }
    $parts = $connection -split "`t"
    if ($parts.Length -ne 2 -or [string]::IsNullOrWhiteSpace($parts[0]) -or [int]$parts[1] -lt 1) {
        throw "Adapter connection '$ConnectionID' has no current revision or active credential."
    }
    $connectionRevisionID = $parts[0]

    $route = & docker exec -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -N -B -u cookies cookies -e @"
SELECT id, COALESCE((SELECT MAX(revision_number) FROM provider_model_route_revisions rr WHERE rr.route_id=r.id),0)
FROM provider_model_routes r
WHERE r.organization_id IS NULL AND r.capability='video.generate' AND r.model_alias='cookies.video.standard' AND r.status='enabled';
"@
    Assert-LastExitCode "Could not inspect cookies.video.standard."
    if ([string]::IsNullOrWhiteSpace($route)) {
        throw "Enabled global cookies.video.standard route was not found."
    }
    $routeParts = $route -split "`t"
    $routeID = $routeParts[0]
    $revisionNumber = [int64]$routeParts[1] + 1
    $revisionID = "${routeID}_adapter_r${revisionNumber}"

    $sql = @"
START TRANSACTION;
INSERT INTO provider_model_route_revisions
  (id,route_id,revision_number,connection_id,connection_revision_id,upstream_model,constraints_json)
VALUES (
  '$revisionID','$routeID',$revisionNumber,'$ConnectionID','$connectionRevisionID','$Model',
  JSON_OBJECT(
    'endpoint','/v1/videos/generations',
    'poll_endpoint','/v1/videos/generations/{task_id}',
    'source_provider','seedance',
    'duration_seconds_min',4,
    'duration_seconds_max',15,
    'aspect_ratios',JSON_ARRAY('9:16','16:9','1:1'),
    'resolutions',JSON_ARRAY('480p','720p'),
    'video_input_modes',JSON_ARRAY('text_only','reference_image','first_last_frame'),
    'video_audio_policies',JSON_ARRAY('silent','generated_audio')
  )
);
UPDATE provider_model_routes
SET current_revision_id='$revisionID', version=version+1
WHERE id='$routeID';
COMMIT;
"@
    & docker exec -i -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -u cookies cookies -e $sql
    Assert-LastExitCode "Switching cookies.video.standard to Adapter failed."

    Write-Output "Configured cookies.video.standard through Adapter revision $revisionID."
    Write-Output "Set COOKIES_PROVIDER_VIDEO_ADAPTER=adapter_gateway and restart cookies-api."
}
finally {
    Pop-Location
}
