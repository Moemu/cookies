param(
    [string]$DotEnvPath = (Join-Path $PSScriptRoot "..\.env"),
    [switch]$ProbeEndpoint
)

$ErrorActionPreference = "Stop"

function Get-DotEnvValue {
    param([Parameter(Mandatory = $true)][string]$Name)

    if (-not (Test-Path -LiteralPath $DotEnvPath)) {
        throw "Missing local .env file: $DotEnvPath"
    }
    $line = Get-Content -LiteralPath $DotEnvPath |
        Where-Object { $_ -match ("^\s*" + [regex]::Escape($Name) + "\s*=") } |
        Select-Object -Last 1
    if ($null -eq $line) {
        return ""
    }
    return (($line -split "=", 2)[1]).Trim().Trim('"').Trim("'")
}

$adapter = Get-DotEnvValue "COOKIES_PROVIDER_AUDIO_ADAPTER"
$endpoint = Get-DotEnvValue "COOKIES_VOLCENGINE_ASR_ENDPOINT"
$authMode = Get-DotEnvValue "COOKIES_VOLCENGINE_ASR_AUTH_MODE"
$appID = Get-DotEnvValue "COOKIES_VOLCENGINE_ASR_APP_ID"
$accessToken = Get-DotEnvValue "COOKIES_VOLCENGINE_ASR_ACCESS_TOKEN"
$apiKey = Get-DotEnvValue "COOKIES_VOLCENGINE_ASR_API_KEY"
$resourceID = Get-DotEnvValue "COOKIES_VOLCENGINE_ASR_RESOURCE_ID"
$model = Get-DotEnvValue "COOKIES_VOLCENGINE_ASR_MODEL"

if ($adapter -ne "volcengine_asr") {
    throw "Set COOKIES_PROVIDER_AUDIO_ADAPTER=volcengine_asr in .env first."
}
if (-not [Uri]::IsWellFormedUriString($endpoint, [UriKind]::Absolute) -or
    ([Uri]$endpoint).Scheme -ne "https") {
    throw "COOKIES_VOLCENGINE_ASR_ENDPOINT must be an absolute HTTPS URL."
}
if ([string]::IsNullOrWhiteSpace($resourceID) -or [string]::IsNullOrWhiteSpace($model)) {
    throw "ASR resource ID and model must not be empty."
}

$headers = @{
    "X-Api-Resource-Id" = $resourceID
    "X-Api-Request-Id" = [Guid]::NewGuid().ToString()
    "X-Api-Sequence" = "-1"
}
switch ($authMode) {
    "legacy" {
        if ([string]::IsNullOrWhiteSpace($appID) -or [string]::IsNullOrWhiteSpace($accessToken)) {
            throw "Legacy ASR auth requires APP ID and Access Token."
        }
        $headers["X-Api-App-Key"] = $appID
        $headers["X-Api-Access-Key"] = $accessToken
    }
    "api_key" {
        if ([string]::IsNullOrWhiteSpace($apiKey)) {
            throw "API-key ASR auth requires an API Key."
        }
        $headers["X-Api-Key"] = $apiKey
    }
    default {
        throw "COOKIES_VOLCENGINE_ASR_AUTH_MODE must be legacy or api_key."
    }
}

Write-Output ("ASR local configuration is structurally valid: auth={0}, resource={1}, model={2}" -f $authMode, $resourceID, $model)
Write-Output "Credentials were not printed."

if (-not $ProbeEndpoint) {
    Write-Output "Endpoint probe skipped. Run again with -ProbeEndpoint to send a one-second synthetic silent WAV."
    exit 0
}

$sampleRate = 16000
$dataSize = $sampleRate * 2
$stream = New-Object System.IO.MemoryStream
$writer = New-Object System.IO.BinaryWriter($stream)
$writer.Write([Text.Encoding]::ASCII.GetBytes("RIFF"))
$writer.Write([int](36 + $dataSize))
$writer.Write([Text.Encoding]::ASCII.GetBytes("WAVE"))
$writer.Write([Text.Encoding]::ASCII.GetBytes("fmt "))
$writer.Write([int]16)
$writer.Write([int16]1)
$writer.Write([int16]1)
$writer.Write([int]$sampleRate)
$writer.Write([int]($sampleRate * 2))
$writer.Write([int16]2)
$writer.Write([int16]16)
$writer.Write([Text.Encoding]::ASCII.GetBytes("data"))
$writer.Write([int]$dataSize)
$writer.Write((New-Object byte[] $dataSize))
$writer.Flush()
$syntheticAudio = [Convert]::ToBase64String($stream.ToArray())
$writer.Dispose()
$stream.Dispose()

$probeUID = if ($authMode -eq "legacy") { $appID } else { "cookies-local-config-probe" }
$body = @{
    user = @{ uid = $probeUID }
    audio = @{ data = $syntheticAudio }
    request = @{ model_name = $model }
} | ConvertTo-Json -Depth 4

try {
    $response = Invoke-WebRequest -Method Post -Uri $endpoint -Headers $headers `
        -ContentType "application/json" -Body $body -UseBasicParsing
    $statusCode = $response.Headers["X-Api-Status-Code"]
    $message = $response.Headers["X-Api-Message"]
    $logID = $response.Headers["X-Tt-Logid"]
} catch {
    $webResponse = $_.Exception.Response
    if ($null -eq $webResponse) {
        throw "ASR endpoint probe could not reach the service: $($_.Exception.Message)"
    }
    $statusCode = $webResponse.Headers["X-Api-Status-Code"]
    $message = $webResponse.Headers["X-Api-Message"]
    $logID = $webResponse.Headers["X-Tt-Logid"]
    if ([string]::IsNullOrWhiteSpace($statusCode)) {
        throw "ASR endpoint rejected the probe without a Volcengine status header (HTTP $([int]$webResponse.StatusCode))."
    }
}

Write-Output ("ASR endpoint responded: status={0}, message={1}, log_id={2}" -f $statusCode, $message, $logID)
if ($statusCode -eq "20000000") {
    Write-Output "The endpoint accepted the probe."
} elseif ($statusCode -eq "20000003" -and $message -match "silence|no valid speech") {
    Write-Output "ASR authentication and entitlement are available; the synthetic probe was correctly classified as silence."
} elseif ($statusCode -eq "45000001" -or $statusCode -eq "45000002") {
    Write-Output "The authenticated request reached ASR audio validation. Full transcription still requires a real speech sample."
} else {
    throw "ASR endpoint probe was not accepted. Keep the configuration marked unverified until the credential or account entitlement is corrected."
}
