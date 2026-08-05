param(
    [ValidateSet("MiniMax-M2.7", "MiniMax-M2.7-highspeed")]
    [string]$Model = "MiniMax-M2.7",
    [switch]$ReplaceExisting
)

# Configures the local Strategy text route for MiniMax's OpenAI-compatible
# API. The API key is read from a masked prompt and saved only as encrypted
# credential material in local MySQL; it is never written to .env, shell
# history, command arguments, source control, or output.

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $repoRoot ".env"

function Get-DotEnvValue([string]$Key) {
    if (-not (Test-Path -LiteralPath $envFile)) {
        throw "Missing $envFile. Copy .env.example to .env first."
    }
    $line = Get-Content -LiteralPath $envFile |
        Where-Object { $_ -match "^\s*$([regex]::Escape($Key))\s*=" } |
        Select-Object -Last 1
    if ($null -eq $line) { return "" }
    return (($line -split "=", 2)[1].Trim()).Trim('"').Trim("'")
}

function Set-DotEnvValue([string]$Key, [string]$Value) {
    $lines = @(Get-Content -LiteralPath $envFile)
    $pattern = "^\s*$([regex]::Escape($Key))\s*="
    $updated = $false
    for ($index = 0; $index -lt $lines.Count; $index++) {
        if ($lines[$index] -match $pattern) {
            $lines[$index] = "$Key=$Value"
            $updated = $true
        }
    }
    if (-not $updated) {
        $lines += "$Key=$Value"
    }
    [IO.File]::WriteAllLines($envFile, $lines, [Text.UTF8Encoding]::new($false))
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
    $mysqlPassword = Get-DotEnvValue "COOKIES_MYSQL_PASSWORD"
    if ([string]::IsNullOrWhiteSpace($mysqlPassword)) { throw "COOKIES_MYSQL_PASSWORD is required in .env." }
    $env:COOKIES_PROVIDER_MASTER_KEY = $masterKey
    $env:COOKIES_PROVIDER_MASTER_KEY_VERSION = $masterKeyVersion

    & docker compose up -d mysql | Out-Null
    Assert-LastExitCode "Unable to start local MySQL through docker compose."
    & go run ./cmd/cookies-migrate | Out-Null
    Assert-LastExitCode "Database migrations failed."

    $secureKey = Read-Host "Paste a newly rotated MiniMax API key (input stays hidden)" -AsSecureString
    $plainKey = Convert-SecureStringToPlainText $secureKey
    if ([string]::IsNullOrWhiteSpace($plainKey) -or $plainKey -notmatch '^sk-[A-Za-z0-9_-]{20,}$') {
        throw "MiniMax API key must be the complete value beginning with 'sk-'."
    }
    try {
        $encryptedJSON = $plainKey | & go run ./cmd/cookies-provider-credential
        Assert-LastExitCode "Credential encryption failed."
        $encrypted = $encryptedJSON | ConvertFrom-Json
    }
    finally {
        $plainKey = $null
    }

    $connectionID = "connection_minimax_text"
    $routeID = "route_cookies_text_standard"
    $existingRoute = & docker exec -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -N -s -u cookies cookies -e "SELECT COUNT(*) FROM provider_model_routes WHERE organization_id IS NULL AND capability='text.generate' AND model_alias='cookies.text.standard'"
    Assert-LastExitCode "Could not inspect the local Strategy text route."
    if ([int]$existingRoute -gt 0 -and -not $ReplaceExisting) {
        throw "A global cookies.text.standard route already exists. Review it first, or rerun with -ReplaceExisting to intentionally replace it."
    }

    $ciphertextBase64 = $encrypted.ciphertext_base64
    $nonceBase64 = $encrypted.nonce_base64
    $sql = @"
START TRANSACTION;
UPDATE provider_model_routes SET current_revision_id = NULL
  WHERE organization_id IS NULL AND capability = 'text.generate' AND model_alias = 'cookies.text.standard';
DELETE rr FROM provider_model_route_revisions rr
  JOIN provider_model_routes r ON r.id = rr.route_id
  WHERE r.organization_id IS NULL AND r.capability = 'text.generate' AND r.model_alias = 'cookies.text.standard';
DELETE FROM provider_model_routes
  WHERE organization_id IS NULL AND capability = 'text.generate' AND model_alias = 'cookies.text.standard';
DELETE FROM provider_credentials WHERE connection_id = '$connectionID';
UPDATE provider_connections SET current_revision_id = NULL WHERE id = '$connectionID';
DELETE FROM provider_connection_revisions WHERE connection_id = '$connectionID';
DELETE FROM provider_connections WHERE id = '$connectionID';
INSERT INTO provider_connections (id, connection_code, connection_type, current_revision_id, status)
VALUES ('$connectionID', 'minimax-openai-compatible', 'adapter_gateway', NULL, 'enabled');
INSERT INTO provider_connection_revisions (id, connection_id, revision_number, base_url, timeout_seconds, max_response_bytes)
VALUES ('connection_minimax_text_r1', '$connectionID', 1, 'https://api.minimax.io/v1', 120, 4194304);
UPDATE provider_connections SET current_revision_id = 'connection_minimax_text_r1' WHERE id = '$connectionID';
INSERT INTO provider_credentials (id, connection_id, credential_version, ciphertext, nonce, key_version, status, active_from)
VALUES ('credential_minimax_text_v1', '$connectionID', 1, FROM_BASE64('$ciphertextBase64'), FROM_BASE64('$nonceBase64'), '$($encrypted.key_version)', 'active', UTC_TIMESTAMP(6));
INSERT INTO provider_model_routes (id, organization_id, capability, model_alias, current_revision_id, status)
VALUES ('$routeID', NULL, 'text.generate', 'cookies.text.standard', NULL, 'enabled');
INSERT INTO provider_model_route_revisions (id, route_id, revision_number, connection_id, connection_revision_id, upstream_model, constraints_json)
VALUES ('route_cookies_text_standard_minimax_r1', '$routeID', 1, '$connectionID', 'connection_minimax_text_r1', '$Model',
  JSON_OBJECT('text_response_mode', 'prompt_json', 'max_output_tokens', 2048,
    'output_token_parameter', 'max_completion_tokens', 'temperature', 0.3, 'reasoning_split', TRUE,
    'source_provider', 'minimax'));
UPDATE provider_model_routes SET current_revision_id = 'route_cookies_text_standard_minimax_r1' WHERE id = '$routeID';
COMMIT;
"@
    & docker exec -i -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -u cookies cookies -e $sql
    Assert-LastExitCode "Saving the encrypted MiniMax credential and Strategy route failed."

    Set-DotEnvValue "COOKIES_PROVIDER_TEXT_ADAPTER" "adapter_gateway"
    Set-DotEnvValue "COOKIES_STRATEGY_REAL_PROVIDER_ENABLED" "true"
    Set-DotEnvValue "COOKIES_STRATEGY_TEXT_MODEL_ALIAS" "cookies.text.standard"
    Write-Output "Strategy text route configured: cookies.text.standard -> $Model"
    Write-Output "The MiniMax key was encrypted in local MySQL and was not written to .env. Restart cookies-api to activate it."
}
finally {
    Pop-Location
}
