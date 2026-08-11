param(
    [string]$Model = "doubao-seed-2-1-pro-260628",
    [string]$BaseURL = "https://ark.cn-beijing.volces.com/api/v3",
    [switch]$ReplaceExisting
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $repoRoot ".env"

if ($Model -notmatch '^[A-Za-z0-9._-]{1,128}$') {
    throw "Model contains unsupported characters."
}
$parsedBaseURL = $null
if (-not [Uri]::TryCreate($BaseURL, [UriKind]::Absolute, [ref]$parsedBaseURL) -or
    $parsedBaseURL.Scheme -ne "https" -or
    [string]::IsNullOrWhiteSpace($parsedBaseURL.Host)) {
    throw "BaseURL must be an absolute HTTPS URL."
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
        else {
            $line
        }
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
    Assert-LastExitCode "Unable to start local MySQL."
    & go run ./cmd/cookies-migrate | Out-Null
    Assert-LastExitCode "Database migrations failed."

    $secureKey = Read-Host "Paste the Ark API key for Seed web research (input stays hidden)" -AsSecureString
    $plainKey = Convert-SecureStringToPlainText $secureKey
    if ([string]::IsNullOrWhiteSpace($plainKey) -or
        $plainKey.Length -lt 20 -or
        $plainKey.Length -gt 512 -or
        $plainKey -match '\s') {
        throw "Ark API key appears incomplete or contains whitespace."
    }
    try {
        $encryptedJSON = $plainKey | & go run ./cmd/cookies-provider-credential
        Assert-LastExitCode "Credential encryption failed."
        $encrypted = $encryptedJSON | ConvertFrom-Json
    }
    finally {
        $plainKey = $null
    }

    $connectionID = "connection_ark_research"
    $routeID = "route_ark_research_web"
    $mysqlPassword = Get-DotEnvValue "COOKIES_MYSQL_PASSWORD"
    if ([string]::IsNullOrWhiteSpace($mysqlPassword)) {
        throw "COOKIES_MYSQL_PASSWORD is required in .env."
    }
    $exists = & docker exec -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -N -s -u cookies cookies -e "SELECT COUNT(*) FROM provider_connections WHERE id='$connectionID'"
    Assert-LastExitCode "Could not inspect the local Ark research route."
    if ([int]$exists -gt 0 -and -not $ReplaceExisting) {
        throw "The local Ark research route already exists. Use -ReplaceExisting only for an intentional rotation."
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
VALUES ('$connectionID', 'ark-research-web', 'ark', NULL, 'enabled');
INSERT INTO provider_connection_revisions
  (id, connection_id, revision_number, base_url, timeout_seconds, max_response_bytes)
VALUES ('connection_ark_research_r1', '$connectionID', 1, '$normalizedBaseURL', 300, 4194304);
UPDATE provider_connections
SET current_revision_id = 'connection_ark_research_r1'
WHERE id = '$connectionID';
INSERT INTO provider_credentials
  (id, connection_id, credential_version, ciphertext, nonce, key_version, status, active_from)
VALUES (
  'credential_ark_research_v1', '$connectionID', 1,
  FROM_BASE64('$ciphertextBase64'), FROM_BASE64('$nonceBase64'),
  '$masterKeyVersion', 'active', UTC_TIMESTAMP(6)
);
INSERT INTO provider_model_routes
  (id, organization_id, capability, model_alias, current_revision_id, status)
VALUES ('$routeID', NULL, 'research.web', 'cookies.research.web.standard', NULL, 'enabled');
INSERT INTO provider_model_route_revisions
  (id, route_id, revision_number, connection_id, connection_revision_id, upstream_model, constraints_json)
VALUES (
  'route_ark_research_web_r1', '$routeID', 1, '$connectionID',
  'connection_ark_research_r1', '$Model', JSON_OBJECT()
);
UPDATE provider_model_routes
SET current_revision_id = 'route_ark_research_web_r1'
WHERE id = '$routeID';
COMMIT;
"@
    & docker exec -i -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -u cookies cookies -e $sql
    Assert-LastExitCode "Saving the encrypted Ark research route failed."

    Set-DotEnvValue "COOKIES_RESEARCH_SEED_ENABLED" "true"
    Set-DotEnvValue "COOKIES_RESEARCH_SEED_MODEL_ALIAS" "cookies.research.web.standard"
    Write-Output "Seed web research configured: cookies.research.web.standard -> $Model"
}
finally {
    Pop-Location
}
