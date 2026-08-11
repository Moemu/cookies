param(
    [string]$BaseURL = "https://operator.las.cn-beijing.volces.com/api/v1",
    [ValidateSet("normal", "detail")]
    [string]$ParseMode = "normal",
    [switch]$ReplaceExisting
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $repoRoot ".env"
$parsedBaseURL = $null
if (-not [Uri]::TryCreate($BaseURL, [UriKind]::Absolute, [ref]$parsedBaseURL) -or
    $parsedBaseURL.Scheme -ne "https" -or [string]::IsNullOrWhiteSpace($parsedBaseURL.Host) -or
    -not [string]::IsNullOrWhiteSpace($parsedBaseURL.Query) -or -not [string]::IsNullOrWhiteSpace($parsedBaseURL.Fragment)) {
    throw "BaseURL must be an absolute HTTPS URL without query or fragment."
}

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

function Set-DotEnvValue([string]$Key, [string]$Value) {
    $content = @(Get-Content -LiteralPath $envFile)
    $pattern = "^\s*$([regex]::Escape($Key))\s*="
    $updated = $false
    $next = foreach ($line in $content) {
        if ($line -match $pattern) {
            "$Key=$Value"
            $updated = $true
        }
        else { $line }
    }
    if (-not $updated) { $next += "$Key=$Value" }
    Set-Content -LiteralPath $envFile -Value $next -Encoding utf8
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
    $masterKey = Get-DotEnvValue "COOKIES_PROVIDER_MASTER_KEY"
    $masterKeyVersion = Get-DotEnvValue "COOKIES_PROVIDER_MASTER_KEY_VERSION"
    $singleBucket = Get-DotEnvValue "COOKIES_TOS_BUCKET"
    $blobProvider = Get-DotEnvValue "COOKIES_BLOB_PROVIDER"
    if ($blobProvider -ne "tos") {
        throw "Set COOKIES_BLOB_PROVIDER=tos before configuring LAS."
    }
    foreach ($requiredTOSKey in @("COOKIES_TOS_ENDPOINT", "COOKIES_TOS_REGION", "COOKIES_TOS_ACCESS_KEY", "COOKIES_TOS_SECRET_KEY")) {
        if ([string]::IsNullOrWhiteSpace((Get-DotEnvValue $requiredTOSKey))) {
            throw "Set $requiredTOSKey before configuring LAS. Do not use unscoped TOS_* aliases."
        }
    }
    if ([string]::IsNullOrWhiteSpace($singleBucket)) {
        throw "Set COOKIES_TOS_BUCKET to the one shared TOS bucket before configuring LAS."
    }
    if ($singleBucket -notmatch '^[A-Za-z0-9._-]{1,255}$') {
        throw "COOKIES_TOS_BUCKET contains unsupported characters."
    }
    if ([string]::IsNullOrWhiteSpace($masterKey) -or $masterKeyVersion -notmatch '^[A-Za-z0-9._-]{1,64}$') {
        throw "Set a valid COOKIES_PROVIDER_MASTER_KEY and COOKIES_PROVIDER_MASTER_KEY_VERSION first."
    }
    try {
        if ([Convert]::FromBase64String($masterKey).Length -ne 32) { throw "invalid length" }
    }
    catch { throw "COOKIES_PROVIDER_MASTER_KEY must be a base64-encoded 32-byte value." }

    # The Go commands below read process environment variables; validating .env
    # alone is not enough. Export the already-validated single-bucket settings so
    # migrations and readiness checks observe the same configuration as the script.
    $env:COOKIES_BLOB_PROVIDER = $blobProvider
    $env:COOKIES_TOS_ENDPOINT = Get-DotEnvValue "COOKIES_TOS_ENDPOINT"
    $env:COOKIES_TOS_REGION = Get-DotEnvValue "COOKIES_TOS_REGION"
    $env:COOKIES_TOS_ACCESS_KEY = Get-DotEnvValue "COOKIES_TOS_ACCESS_KEY"
    $env:COOKIES_TOS_SECRET_KEY = Get-DotEnvValue "COOKIES_TOS_SECRET_KEY"
    $env:COOKIES_TOS_SECURITY_TOKEN = Get-DotEnvValue "COOKIES_TOS_SECURITY_TOKEN"
    $env:COOKIES_TOS_BUCKET = $singleBucket
    $env:COOKIES_PROVIDER_MASTER_KEY = $masterKey
    $env:COOKIES_PROVIDER_MASTER_KEY_VERSION = $masterKeyVersion
    $env:COOKIES_DOCUMENT_VISION_ENABLED = "true"
    $env:COOKIES_DOCUMENT_VISION_MODEL_ALIAS = "cookies.document.vision.standard"

    & docker compose up -d mysql | Out-Null
    Assert-LastExitCode "Unable to start local MySQL."
    & go run ./cmd/cookies-migrate | Out-Null
    Assert-LastExitCode "Database migrations failed."

    $secureKey = Read-Host "Paste the LAS Operator API key (input stays hidden)" -AsSecureString
    $plainKey = Convert-SecureStringToPlainText $secureKey
    if ([string]::IsNullOrWhiteSpace($plainKey) -or $plainKey.Length -lt 16 -or
        $plainKey.Length -gt 512 -or $plainKey -match '\s') {
        throw "LAS API key appears incomplete or contains whitespace."
    }
    try {
        $encryptedJSON = $plainKey | & go run ./cmd/cookies-provider-credential
        Assert-LastExitCode "Credential encryption failed."
        $encrypted = $encryptedJSON | ConvertFrom-Json
    }
    finally { $plainKey = $null }

    $connectionID = "connection_las_document_vision"
    $routeID = "route_las_document_vision"
    $mysqlPassword = Get-DotEnvValue "COOKIES_MYSQL_PASSWORD"
    if ([string]::IsNullOrWhiteSpace($mysqlPassword)) {
        throw "COOKIES_MYSQL_PASSWORD is required in .env."
    }
    $exists = & docker exec -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -N -s -u cookies cookies -e "SELECT COUNT(*) FROM provider_connections WHERE id='$connectionID'"
    Assert-LastExitCode "Could not inspect the local LAS document route."
    if ([int]$exists -gt 0 -and -not $ReplaceExisting) {
        throw "The LAS document route already exists. Use -ReplaceExisting only for an intentional credential rotation."
    }

    $ciphertextBase64 = $encrypted.ciphertext_base64
    $nonceBase64 = $encrypted.nonce_base64
    $normalizedBaseURL = $BaseURL.TrimEnd("/")
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
VALUES ('$connectionID', 'las-document-vision', 'las_operator', NULL, 'enabled');
INSERT INTO provider_connection_revisions
  (id, connection_id, revision_number, base_url, timeout_seconds, max_response_bytes)
VALUES ('connection_las_document_vision_r1', '$connectionID', 1, '$normalizedBaseURL', 900, 8388608);
UPDATE provider_connections SET current_revision_id = 'connection_las_document_vision_r1' WHERE id = '$connectionID';
INSERT INTO provider_credentials
  (id, connection_id, credential_version, ciphertext, nonce, key_version, status, active_from)
VALUES (
  'credential_las_document_vision_v1', '$connectionID', 1,
  FROM_BASE64('$ciphertextBase64'), FROM_BASE64('$nonceBase64'),
  '$masterKeyVersion', 'active', UTC_TIMESTAMP(6)
);
INSERT INTO provider_model_routes
  (id, organization_id, capability, model_alias, current_revision_id, status)
VALUES ('$routeID', NULL, 'document.vision.parse', 'cookies.document.vision.standard', NULL, 'enabled');
INSERT INTO provider_model_route_revisions
  (id, route_id, revision_number, connection_id, connection_revision_id, upstream_model, constraints_json)
VALUES (
  'route_las_document_vision_r1', '$routeID', 1, '$connectionID',
  'connection_las_document_vision_r1', 'las_pdf_parse_doubao',
  JSON_OBJECT(
    'endpoint', '/submit', 'poll_endpoint', '/poll', 'operator_version', 'v1',
    'parse_mode', '$ParseMode', 'full_result', TRUE,
    'aspect_ratio_threshold', 0.334, 'poll_interval_ms', 2000
  )
);
UPDATE provider_model_routes SET current_revision_id = 'route_las_document_vision_r1' WHERE id = '$routeID';
COMMIT;
"@
    & docker exec -i -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -u cookies cookies -e $sql
    Assert-LastExitCode "Saving the encrypted LAS document route failed."

    Set-DotEnvValue "COOKIES_DOCUMENT_VISION_ENABLED" "true"
    Set-DotEnvValue "COOKIES_DOCUMENT_VISION_MODEL_ALIAS" "cookies.document.vision.standard"
    & go run ./cmd/cookies-check-document-vision-readiness
    Assert-LastExitCode "LAS route was saved but the read-only readiness check did not pass. Review its blocker codes before restarting cookies-api."
    Write-Output "LAS document vision configured for the shared bucket '$singleBucket'. Restart cookies-api to enable it."
}
finally { Pop-Location }
