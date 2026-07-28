param(
    [string]$AdapterBaseURL = "http://118.196.44.61:9060",
    [switch]$ReplaceExisting,
    [switch]$UseLegacyOpenAIImageToken
)

# Configures the local Provider Gateway route for the shared image Adapter.
# The bearer token is read from a masked prompt and never written to .env,
# command history, source control, or SQL text. Local MySQL stores only its
# encrypted form; the master key remains in the ignored .env file.

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
    $imageAdapter = Get-DotEnvValue "COOKIES_PROVIDER_IMAGE_ADAPTER"
    $allowInsecureHTTP = Get-DotEnvValue "COOKIES_PROVIDER_ALLOW_INSECURE_HTTP"

    if ($imageAdapter -ne "adapter_gateway") {
        throw "Set COOKIES_PROVIDER_IMAGE_ADAPTER=adapter_gateway in .env before running this script."
    }
    if ([string]::IsNullOrWhiteSpace($masterKey) -or [string]::IsNullOrWhiteSpace($masterKeyVersion)) {
        throw "Set COOKIES_PROVIDER_MASTER_KEY and COOKIES_PROVIDER_MASTER_KEY_VERSION in .env before running this script."
    }
    try {
        if ([Convert]::FromBase64String($masterKey).Length -ne 32) {
            throw "invalid length"
        }
    }
    catch {
        throw "COOKIES_PROVIDER_MASTER_KEY must be a base64-encoded 32-byte value."
    }
    if ($AdapterBaseURL -match '^http://' -and $allowInsecureHTTP -ne "true") {
        throw "The shared Adapter currently uses HTTP. Set COOKIES_PROVIDER_ALLOW_INSECURE_HTTP=true in local .env."
    }
    $env:COOKIES_PROVIDER_MASTER_KEY = $masterKey
    $env:COOKIES_PROVIDER_MASTER_KEY_VERSION = $masterKeyVersion

    $health = Invoke-RestMethod -Method Get -Uri ($AdapterBaseURL.TrimEnd('/') + "/healthz") -TimeoutSec 10
    if ($health.status -ne "ok") { throw "Adapter health check did not report status=ok." }

    & docker compose up -d mysql | Out-Null
    Assert-LastExitCode "Unable to start local MySQL through docker compose."
    & go run ./cmd/cookies-migrate | Out-Null
    Assert-LastExitCode "Database migrations failed."

    if ($UseLegacyOpenAIImageToken) {
        $plainToken = Get-DotEnvValue "COOKIES_OPENAI_IMAGE_API_KEY"
    }
    else {
        $token = Read-Host "Paste the deployed Adapter calling token (input stays hidden)" -AsSecureString
        $plainToken = Convert-SecureStringToPlainText $token
    }
    if ([string]::IsNullOrWhiteSpace($plainToken)) { throw "Adapter token must not be empty." }
    try {
        $encryptedJSON = $plainToken | & go run ./cmd/cookies-provider-credential
        Assert-LastExitCode "Credential encryption failed."
        $encrypted = $encryptedJSON | ConvertFrom-Json
    }
    finally {
        $plainToken = $null
    }

    $connectionID = "connection_adapter_shared"
    $routeID = "route_adapter_shared_image"
    $mysqlPassword = Get-DotEnvValue "COOKIES_MYSQL_PASSWORD"
    if ([string]::IsNullOrWhiteSpace($mysqlPassword)) { throw "COOKIES_MYSQL_PASSWORD is required in .env." }
    $baseURL = $AdapterBaseURL.TrimEnd('/') + "/v1"

    $exists = & docker exec -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -N -s -u cookies cookies -e "SELECT COUNT(*) FROM provider_connections WHERE id='$connectionID'"
    Assert-LastExitCode "Could not inspect the local Provider route."
    if ([int]$exists -gt 0 -and -not $ReplaceExisting) {
        throw "A local shared Adapter route already exists. Review it first, or rerun with -ReplaceExisting to replace only its known local IDs."
    }

    $ciphertextBase64 = $encrypted.ciphertext_base64
    $nonceBase64 = $encrypted.nonce_base64
    $sql = @"
START TRANSACTION;
DELETE FROM provider_model_route_revisions WHERE route_id = '$routeID';
DELETE FROM provider_model_routes WHERE id = '$routeID';
DELETE FROM provider_credentials WHERE connection_id = '$connectionID';
DELETE FROM provider_connection_revisions WHERE connection_id = '$connectionID';
DELETE FROM provider_connections WHERE id = '$connectionID';
INSERT INTO provider_connections (id, connection_code, connection_type, current_revision_id, status)
VALUES ('$connectionID', 'adapter-shared-http', 'adapter_gateway', NULL, 'enabled');
INSERT INTO provider_connection_revisions (id, connection_id, revision_number, base_url, timeout_seconds, max_response_bytes)
VALUES ('connection_adapter_shared_r1', '$connectionID', 1, '$baseURL', 210, 41943040);
UPDATE provider_connections SET current_revision_id = 'connection_adapter_shared_r1' WHERE id = '$connectionID';
INSERT INTO provider_credentials (id, connection_id, credential_version, ciphertext, nonce, key_version, status, active_from)
VALUES ('credential_adapter_shared_v1', '$connectionID', 1, FROM_BASE64('$ciphertextBase64'), FROM_BASE64('$nonceBase64'), '$masterKeyVersion', 'active', UTC_TIMESTAMP(6));
INSERT INTO provider_model_routes (id, organization_id, capability, model_alias, current_revision_id, status)
VALUES ('$routeID', NULL, 'image.generate', 'cookies.image.standard', NULL, 'enabled');
INSERT INTO provider_model_route_revisions (id, route_id, revision_number, connection_id, connection_revision_id, upstream_model, constraints_json)
VALUES ('route_adapter_shared_image_r1', '$routeID', 1, '$connectionID', 'connection_adapter_shared_r1', 'gpt-image-2', JSON_OBJECT('width', 1024, 'height', 1024, 'n', 1, 'output_format', 'png'));
UPDATE provider_model_routes SET current_revision_id = 'route_adapter_shared_image_r1' WHERE id = '$routeID';
COMMIT;
"@
    & docker exec -i -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -u cookies cookies -e $sql
    Assert-LastExitCode "Saving the encrypted Adapter credential and route failed."

    Write-Output "Adapter route is configured locally. Start the API with: go run ./cmd/cookies-api"
    Write-Output "The Adapter token was not written to .env or printed."
}
finally {
    Pop-Location
}
