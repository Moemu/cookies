param(
    [ValidateSet("speech-2.8-hd", "speech-2.8-turbo", "speech-2.6-hd", "speech-2.6-turbo")]
    [string]$Model = "speech-2.8-turbo",
    [string]$WarmFemaleVoiceID = "Chinese (Mandarin)_News_Anchor",
    [string]$MatureFemaleVoiceID = "Chinese (Mandarin)_News_Anchor",
    [string]$YoungFemaleVoiceID = "Chinese (Mandarin)_News_Anchor",
    [switch]$ReplaceExisting
)

# Registers MiniMax Speech as an encrypted Provider route. The key is read
# from a masked prompt and never written to .env, arguments, logs, or source.
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $repoRoot ".env"

function Get-DotEnvValue([string]$Key) {
    if (-not (Test-Path -LiteralPath $envFile)) { throw "Missing $envFile. Copy .env.example to .env first." }
    $line = Get-Content -LiteralPath $envFile | Where-Object { $_ -match "^\s*$([regex]::Escape($Key))\s*=" } | Select-Object -Last 1
    if ($null -eq $line) { return "" }
    return (($line -split "=", 2)[1].Trim()).Trim('"').Trim("'")
}

function Set-DotEnvValue([string]$Key, [string]$Value) {
    $lines = @(Get-Content -LiteralPath $envFile)
    $pattern = "^\s*$([regex]::Escape($Key))\s*="
    $updated = $false
    for ($index = 0; $index -lt $lines.Count; $index++) {
        if ($lines[$index] -match $pattern) { $lines[$index] = "$Key=$Value"; $updated = $true }
    }
    if (-not $updated) { $lines += "$Key=$Value" }
    [IO.File]::WriteAllLines($envFile, $lines, [Text.UTF8Encoding]::new($false))
}

function Assert-LastExitCode([string]$Message) { if ($LASTEXITCODE -ne 0) { throw $Message } }
function Convert-SecureStringToPlainText([Security.SecureString]$Value) {
    $pointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($Value)
    try { return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($pointer) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($pointer) }
}

Push-Location $repoRoot
try {
    $masterKey = Get-DotEnvValue "COOKIES_PROVIDER_MASTER_KEY"
    $masterKeyVersion = Get-DotEnvValue "COOKIES_PROVIDER_MASTER_KEY_VERSION"
    $mysqlPassword = Get-DotEnvValue "COOKIES_MYSQL_PASSWORD"
    if ([string]::IsNullOrWhiteSpace($masterKey) -or [string]::IsNullOrWhiteSpace($masterKeyVersion)) { throw "Provider master key configuration is required." }
    if ([string]::IsNullOrWhiteSpace($mysqlPassword)) { throw "COOKIES_MYSQL_PASSWORD is required." }
    if ([string]::IsNullOrWhiteSpace($WarmFemaleVoiceID)) { throw "WarmFemaleVoiceID is required; obtain it from MiniMax get_voice." }
    $env:COOKIES_PROVIDER_MASTER_KEY = $masterKey
    $env:COOKIES_PROVIDER_MASTER_KEY_VERSION = $masterKeyVersion
    & docker compose up -d mysql | Out-Null
    Assert-LastExitCode "Unable to start local MySQL."
    & go run ./cmd/cookies-migrate | Out-Null
    Assert-LastExitCode "Database migrations failed."

    $existing = & docker exec -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -N -s -u cookies cookies -e "SELECT COUNT(*) FROM provider_model_routes WHERE organization_id IS NULL AND capability='speech.synthesize' AND model_alias='cookies.speech.brand'"
    Assert-LastExitCode "Could not inspect the MiniMax Speech route."
    if ([int]$existing -gt 0 -and -not $ReplaceExisting) { throw "cookies.speech.brand already exists. Use -ReplaceExisting after review." }

    $secureKey = Read-Host "Paste the MiniMax API key (input stays hidden)" -AsSecureString
    $plainKey = Convert-SecureStringToPlainText $secureKey
    if ([string]::IsNullOrWhiteSpace($plainKey)) { throw "MiniMax API key is required." }
    try {
        $encryptedJSON = $plainKey | & go run ./cmd/cookies-provider-credential
        Assert-LastExitCode "Credential encryption failed."
        $encrypted = $encryptedJSON | ConvertFrom-Json
    }
    finally { $plainKey = $null }

    $warmVoiceIDSQL = $WarmFemaleVoiceID.Replace("'", "''")
    $matureVoiceIDSQL = $MatureFemaleVoiceID.Replace("'", "''")
    $youngVoiceIDSQL = $YoungFemaleVoiceID.Replace("'", "''")
    $sql = @"
START TRANSACTION;
UPDATE provider_model_routes SET current_revision_id = NULL WHERE organization_id IS NULL AND capability='speech.synthesize' AND model_alias IN ('cookies.speech.brand','cookies.speech.standard');
DELETE rr FROM provider_model_route_revisions rr JOIN provider_model_routes r ON r.id=rr.route_id WHERE r.organization_id IS NULL AND r.capability='speech.synthesize' AND r.model_alias IN ('cookies.speech.brand','cookies.speech.standard');
DELETE FROM provider_model_routes WHERE organization_id IS NULL AND capability='speech.synthesize' AND model_alias IN ('cookies.speech.brand','cookies.speech.standard');
DELETE FROM provider_credentials WHERE connection_id='connection_minimax_speech';
UPDATE provider_connections SET current_revision_id=NULL WHERE id='connection_minimax_speech';
DELETE FROM provider_connection_revisions WHERE connection_id='connection_minimax_speech';
DELETE FROM provider_connections WHERE id='connection_minimax_speech';
INSERT INTO provider_connections (id,connection_code,connection_type,current_revision_id,status) VALUES ('connection_minimax_speech','minimax-speech','minimax_speech',NULL,'enabled');
INSERT INTO provider_connection_revisions (id,connection_id,revision_number,base_url,timeout_seconds,max_response_bytes) VALUES ('connection_minimax_speech_r1','connection_minimax_speech',1,'https://api.minimaxi.com',60,16777216);
UPDATE provider_connections SET current_revision_id='connection_minimax_speech_r1' WHERE id='connection_minimax_speech';
INSERT INTO provider_credentials (id,connection_id,credential_version,ciphertext,nonce,key_version,status,active_from) VALUES ('credential_minimax_speech_v1','connection_minimax_speech',1,FROM_BASE64('$($encrypted.ciphertext_base64)'),FROM_BASE64('$($encrypted.nonce_base64)'),'$($encrypted.key_version)','active',UTC_TIMESTAMP(6));
INSERT INTO provider_model_routes (id,organization_id,capability,model_alias,current_revision_id,status) VALUES ('route_cookies_speech_brand',NULL,'speech.synthesize','cookies.speech.brand',NULL,'enabled');
INSERT INTO provider_model_route_revisions (id,route_id,revision_number,connection_id,connection_revision_id,upstream_model,constraints_json) VALUES ('route_cookies_speech_brand_minimax_r1','route_cookies_speech_brand',1,'connection_minimax_speech','connection_minimax_speech_r1','$Model',JSON_OBJECT('voice_aliases',JSON_OBJECT('cookies.voice.brand.warm_female','$warmVoiceIDSQL','cookies.voice.brand.mature_female','$matureVoiceIDSQL','cookies.voice.brand.young_female','$youngVoiceIDSQL')));
UPDATE provider_model_routes SET current_revision_id='route_cookies_speech_brand_minimax_r1' WHERE id='route_cookies_speech_brand';
INSERT INTO provider_model_routes (id,organization_id,capability,model_alias,current_revision_id,status) VALUES ('route_cookies_speech_standard',NULL,'speech.synthesize','cookies.speech.standard',NULL,'enabled');
INSERT INTO provider_model_route_revisions (id,route_id,revision_number,connection_id,connection_revision_id,upstream_model,constraints_json) VALUES ('route_cookies_speech_standard_minimax_r1','route_cookies_speech_standard',1,'connection_minimax_speech','connection_minimax_speech_r1','$Model',JSON_OBJECT('voice_aliases',JSON_OBJECT('cookies.voice.brand.warm_female','$warmVoiceIDSQL','cookies.voice.brand.mature_female','$matureVoiceIDSQL','cookies.voice.brand.young_female','$youngVoiceIDSQL')));
UPDATE provider_model_routes SET current_revision_id='route_cookies_speech_standard_minimax_r1' WHERE id='route_cookies_speech_standard';
COMMIT;
"@
    & docker exec -i -e "MYSQL_PWD=$mysqlPassword" cookies-mysql-1 mysql -u cookies cookies -e $sql
    Assert-LastExitCode "Saving the MiniMax Speech route failed."
    Set-DotEnvValue "COOKIES_PROVIDER_SPEECH_ADAPTER" "minimax_speech"
    Write-Output "MiniMax Speech configured: cookies.speech.brand -> $Model"
    Write-Output "Restart cookies-api, then use the capability probe before generating narration."
}
finally { Pop-Location }
