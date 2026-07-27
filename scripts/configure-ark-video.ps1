param(
    [string]$Model = "doubao-seedance-2-0-fast-260128",
    [string]$BaseURL = "https://ark.cn-beijing.volces.com/api/v3",
    [switch]$ReplaceExisting
)

# Stores the Ark API key encrypted in local MySQL. The key is read from a
# masked prompt and never written to .env, command history, or SQL text.

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

function Convert-SecureStringToPlainText([Security.SecureString]$Value) {
    $pointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($Value)
    try { return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($pointer) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($pointer) }
}

Push-Location $repoRoot
try {
    $env:GOTOOLCHAIN = "local"
    $masterKey = Get-DotEnvValue "COOKIES_PROVIDER_MASTER_KEY"
    $masterKeyVersion = Get-DotEnvValue "COOKIES_PROVIDER_MASTER_KEY_VERSION"
    if ([string]::IsNullOrWhiteSpace($masterKey) -or [string]::IsNullOrWhiteSpace($masterKeyVersion)) {
        throw "Set COOKIES_PROVIDER_MASTER_KEY and COOKIES_PROVIDER_MASTER_KEY_VERSION in .env first."
    }
    try {
        if ([Convert]::FromBase64String($masterKey).Length -ne 32) { throw "invalid length" }
    }
    catch {
        throw "COOKIES_PROVIDER_MASTER_KEY must be a base64-encoded 32-byte value."
    }
    $env:COOKIES_PROVIDER_MASTER_KEY = $masterKey
    $env:COOKIES_PROVIDER_MASTER_KEY_VERSION = $masterKeyVersion

    & docker compose up -d mysql | Out-Null
    Assert-LastExitCode "Unable to start local MySQL through docker compose."
    & go run ./cmd/cookies-migrate | Out-Null
    Assert-LastExitCode "Database migrations failed."

    $secureKey = Read-Host "Paste the Ark API key for Seedance (input stays hidden)" -AsSecureString
    $plainKey = Convert-SecureStringToPlainText $secureKey
    if ([string]::IsNullOrWhiteSpace($plainKey)) { throw "Ark API key must not be empty." }
    if ($plainKey -notmatch '^ark-[A-Za-z0-9-]{20,}$') {
        throw "Ark API key must be the complete value beginning with 'ark-'. Access Key IDs and internal gateway tokens are not accepted."
    }
    try {
        $encryptedJSON = $plainKey | & go run ./cmd/cookies-provider-credential
        Assert-LastExitCode "Credential encryption failed."
        $encrypted = $encryptedJSON | ConvertFrom-Json
    }
    finally {
        $plainKey = $null
    }

    $connectionID = "connection_ark_seedance"
    $routeID = "route_ark_seedance_video"
    $mysqlPassword = Get-DotEnvValue "COOKIES_MYSQL_PASSWORD"
    if ([string]::IsNullOrWhiteSpace($mysqlPassword)) { throw "COOKIES_MYSQL_PASSWORD is required in .env." }
    $exists = & docker exec -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -N -s -u cookies cookies -e "SELECT COUNT(*) FROM provider_connections WHERE id='$connectionID'"
    Assert-LastExitCode "Could not inspect the local Ark video route."
    if ([int]$exists -gt 0 -and -not $ReplaceExisting) {
        throw "The local Ark video route already exists. Rerun with -ReplaceExisting only when intentionally rotating it."
    }

    $ciphertextBase64 = $encrypted.ciphertext_base64
    $nonceBase64 = $encrypted.nonce_base64
    $sql = @"
START TRANSACTION;
UPDATE provider_model_routes SET current_revision_id = NULL WHERE id = '$routeID';
DELETE FROM provider_model_route_revisions WHERE route_id = '$routeID';
DELETE FROM provider_model_routes WHERE id = '$routeID';
DELETE FROM provider_credentials WHERE connection_id = '$connectionID';
UPDATE provider_connections SET current_revision_id = NULL WHERE id = '$connectionID';
DELETE FROM provider_connection_revisions WHERE connection_id = '$connectionID';
DELETE FROM provider_connections WHERE id = '$connectionID';
INSERT INTO provider_connections (id, connection_code, connection_type, current_revision_id, status)
VALUES ('$connectionID', 'ark-seedance', 'ark', NULL, 'enabled');
INSERT INTO provider_connection_revisions (id, connection_id, revision_number, base_url, timeout_seconds, max_response_bytes)
VALUES ('connection_ark_seedance_r1', '$connectionID', 1, '$($BaseURL.TrimEnd('/'))', 900, 209715200);
UPDATE provider_connections SET current_revision_id = 'connection_ark_seedance_r1' WHERE id = '$connectionID';
INSERT INTO provider_credentials (id, connection_id, credential_version, ciphertext, nonce, key_version, status, active_from)
VALUES ('credential_ark_seedance_v1', '$connectionID', 1, FROM_BASE64('$ciphertextBase64'), FROM_BASE64('$nonceBase64'), '$masterKeyVersion', 'active', UTC_TIMESTAMP(6));
INSERT INTO provider_model_routes (id, organization_id, capability, model_alias, current_revision_id, status)
VALUES ('$routeID', NULL, 'video.generate', 'cookies.video.standard', NULL, 'enabled');
INSERT INTO provider_model_route_revisions (id, route_id, revision_number, connection_id, connection_revision_id, upstream_model, constraints_json)
VALUES ('route_ark_seedance_video_r1', '$routeID', 1, '$connectionID', 'connection_ark_seedance_r1', '$Model',
  JSON_OBJECT('duration_seconds_min', 4, 'duration_seconds_max', 15, 'aspect_ratios', JSON_ARRAY('9:16', '16:9', '1:1'), 'resolutions', JSON_ARRAY('480p', '720p')));
UPDATE provider_model_routes SET current_revision_id = 'route_ark_seedance_video_r1' WHERE id = '$routeID';
COMMIT;
"@
    & docker exec -i -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -u cookies cookies -e $sql
    Assert-LastExitCode "Saving the encrypted Ark credential and video route failed."

    Write-Output "Ark video route configured: cookies.video.standard -> $Model"
    Write-Output "Set COOKIES_PROVIDER_VIDEO_ADAPTER=ark_video in .env, then run scripts/verify-ark-video.ps1."
}
finally {
    Pop-Location
}
