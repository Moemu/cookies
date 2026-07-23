param(
    [string]$CsvPath = "C:\Users\Admin\Desktop\zz_model_provider_202607231201.csv",
    [string]$AdapterProviderName = "artsapi (gpt-image-2)",
    [string]$ListenAddress = "127.0.0.1:18080"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$apiProcess = $null

function Assert-LastExitCode([string]$message) {
    if ($LASTEXITCODE -ne 0) {
        throw $message
    }
}

function New-MasterKey {
    $bytes = New-Object byte[] 32
    $rng = New-Object Security.Cryptography.RNGCryptoServiceProvider
    try {
        $rng.GetBytes($bytes)
    }
    finally {
        $rng.Dispose()
    }
    return [Convert]::ToBase64String($bytes)
}

function New-HTTPClient {
    Add-Type -AssemblyName System.Net.Http
    $client = New-Object System.Net.Http.HttpClient
    $client.Timeout = [TimeSpan]::FromSeconds(15)
    return $client
}

Push-Location $repoRoot
try {
    if (-not (Test-Path -LiteralPath $CsvPath)) {
        throw "Provider CSV was not found: $CsvPath"
    }
    $providerRow = Import-Csv -LiteralPath $CsvPath |
        Where-Object { $_.name -eq $AdapterProviderName } |
        Select-Object -First 1
    if (-not $providerRow -or [string]::IsNullOrWhiteSpace($providerRow.api_key)) {
        throw "Adapter provider row or API key is missing"
    }
    if ([string]::IsNullOrWhiteSpace($providerRow.base_url)) {
        throw "Adapter base URL is missing"
    }

    & docker compose up -d mysql | Out-Null
    Assert-LastExitCode "start MySQL failed"
    $deadline = (Get-Date).AddSeconds(60)
    do {
        $mysqlHealth = & docker inspect --format "{{.State.Health.Status}}" cookies-mysql-1 2>$null
        if ($mysqlHealth -eq "healthy") {
            break
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    if ($mysqlHealth -ne "healthy") {
        throw "MySQL did not become healthy"
    }

    $env:COOKIES_ENV = "local"
    # This workstation also runs a Windows MySQL on 127.0.0.1:3306. Docker
    # Desktop exposes the compose MySQL through its IPv6/WSL relay.
    $env:COOKIES_MYSQL_DSN = "cookies:cookies_local_development_only@tcp([::1]:3306)/cookies?parseTime=true&multiStatements=true"
    & go run ./cmd/cookies-migrate | Out-Null
    Assert-LastExitCode "database migration failed"

    $env:COOKIES_PROVIDER_MASTER_KEY = New-MasterKey
    $env:COOKIES_PROVIDER_MASTER_KEY_VERSION = "local-shared-v1"
    $encrypted = ($providerRow.api_key |
        & go run ./cmd/cookies-provider-credential |
        ConvertFrom-Json)
    Assert-LastExitCode "Adapter credential encryption failed"

    $baseURL = $providerRow.base_url.TrimEnd("/") + "/v1"
    $sql = @"
UPDATE provider_model_routes SET current_revision_id=NULL WHERE id='route_adapter_shared_image';
DELETE FROM provider_model_route_revisions WHERE id='route_adapter_shared_image_r1';
DELETE FROM provider_model_routes WHERE id='route_adapter_shared_image';
DELETE FROM provider_credentials WHERE id='credential_adapter_shared_v1';
UPDATE provider_connections SET current_revision_id=NULL WHERE id='connection_adapter_shared';
DELETE FROM provider_connection_revisions WHERE id='connection_adapter_shared_r1';
DELETE FROM provider_connections WHERE id='connection_adapter_shared';
INSERT INTO provider_connections
  (id,connection_code,connection_type,current_revision_id,status)
  VALUES ('connection_adapter_shared','adapter-shared-http','adapter_gateway',NULL,'enabled');
INSERT INTO provider_connection_revisions
  (id,connection_id,revision_number,base_url,timeout_seconds,max_response_bytes)
  VALUES ('connection_adapter_shared_r1','connection_adapter_shared',1,'$baseURL',210,41943040);
UPDATE provider_connections
  SET current_revision_id='connection_adapter_shared_r1'
  WHERE id='connection_adapter_shared';
INSERT INTO provider_credentials
  (id,connection_id,credential_version,ciphertext,nonce,key_version,status,active_from)
  VALUES (
    'credential_adapter_shared_v1','connection_adapter_shared',1,
    FROM_BASE64('$($encrypted.ciphertext_base64)'),
    FROM_BASE64('$($encrypted.nonce_base64)'),
    '$($encrypted.key_version)','active',UTC_TIMESTAMP(6)
  );
INSERT INTO provider_model_routes
  (id,organization_id,capability,model_alias,current_revision_id,status)
  VALUES ('route_adapter_shared_image',NULL,'image.generate','cookies.image.standard',NULL,'enabled');
INSERT INTO provider_model_route_revisions
  (id,route_id,revision_number,connection_id,connection_revision_id,upstream_model,constraints_json)
  VALUES (
    'route_adapter_shared_image_r1','route_adapter_shared_image',1,
    'connection_adapter_shared','connection_adapter_shared_r1','gpt-image-2',
    JSON_OBJECT('width',1024,'height',1024,'n',1,'output_format','png')
  );
UPDATE provider_model_routes
  SET current_revision_id='route_adapter_shared_image_r1'
  WHERE id='route_adapter_shared_image';
"@
    & docker exec -e MYSQL_PWD=cookies_local_development_only cookies-mysql-1 mysql `
        -ucookies cookies -e $sql
    Assert-LastExitCode "Adapter connection and route initialization failed"

    & go build -o dist/cookies-api-integration.exe ./cmd/cookies-api
    Assert-LastExitCode "cookies-api build failed"

    $env:COOKIES_HTTP_ADDR = $ListenAddress
    $env:COOKIES_BLOB_PROVIDER = "filesystem"
    $env:COOKIES_FILESYSTEM_BLOB_ROOT = ".data/integration-blobs"
    $env:COOKIES_LOCAL_ORGANIZATION_ID = "org_local"
    $env:COOKIES_LOCAL_PRINCIPAL_KIND = "user"
    $env:COOKIES_LOCAL_PRINCIPAL_ID = "user_local"
    $env:COOKIES_LOCAL_PROJECT_ID = "project_local"
    $env:COOKIES_LOCAL_SCOPES = "project.read,project.write,assets.read,assets.write,provider.job.create"
    $env:COOKIES_PROVIDER_IMAGE_ADAPTER = "adapter_gateway"
    $env:COOKIES_PROVIDER_ALLOW_INSECURE_HTTP = "true"
    $env:COOKIES_PROVIDER_OUTPUT_BUCKET = "cookies-provider-output"

    $apiProcess = Start-Process `
        -FilePath (Resolve-Path "dist/cookies-api-integration.exe") `
        -PassThru `
        -WindowStyle Hidden

    $client = New-HTTPClient
    try {
        $origin = "http://$ListenAddress"
        $ready = $false
        for ($attempt = 0; $attempt -lt 40; $attempt++) {
            try {
                $readyResponse = $client.GetAsync("$origin/readyz").GetAwaiter().GetResult()
                if ($readyResponse.IsSuccessStatusCode) {
                    $ready = $true
                    break
                }
            }
            catch {
                # Process startup may race this probe.
            }
            Start-Sleep -Milliseconds 500
        }
        if (-not $ready) {
            throw "cookies-api did not become ready"
        }

        $body = @{
            capability = "image.generate"
            model_alias = "cookies.image.standard"
            input = @{
                prompt = "A minimal red circle centered on a plain white background, no text"
                width = 1024
                height = 1024
            }
            project_context_version = 1
        } | ConvertTo-Json -Depth 4 -Compress
        $request = New-Object System.Net.Http.HttpRequestMessage(
            [System.Net.Http.HttpMethod]::Post,
            "$origin/platform/v1/projects/project_local/model/jobs"
        )
        $request.Headers.Add(
            "Idempotency-Key",
            "adapter-shared-integration-$([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())"
        )
        $request.Content = New-Object System.Net.Http.StringContent(
            $body,
            [Text.Encoding]::UTF8,
            "application/json"
        )
        $response = $client.SendAsync($request).GetAwaiter().GetResult()
        $responseText = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        if (-not $response.IsSuccessStatusCode) {
            throw "create image job failed with HTTP $([int]$response.StatusCode): $responseText"
        }
        $job = $responseText | ConvertFrom-Json
        Write-Output "job_id=$($job.id)"
        Write-Output "create_status=$($job.provider_status)"

        $terminal = $false
        for ($attempt = 0; $attempt -lt 90; $attempt++) {
            Start-Sleep -Seconds 2
            $poll = $client.GetAsync(
                "$origin/platform/v1/projects/project_local/model/jobs/$($job.id)"
            ).GetAwaiter().GetResult()
            $pollText = $poll.Content.ReadAsStringAsync().GetAwaiter().GetResult()
            if (-not $poll.IsSuccessStatusCode) {
                throw "poll image job failed with HTTP $([int]$poll.StatusCode)"
            }
            $job = $pollText | ConvertFrom-Json
            if ($job.provider_status -in @(
                "succeeded",
                "partially_succeeded",
                "failed",
                "cancelled",
                "expired"
            )) {
                $terminal = $true
                break
            }
        }
        Write-Output "final_provider_status=$($job.provider_status)"
        Write-Output "final_execution_status=$($job.execution_status)"
        Write-Output "asset_refs=$($job.project_asset_refs.Count)"
        if ($job.error) {
            Write-Output "job_error_code=$($job.error.code)"
        }
        if (-not $terminal) {
            throw "image job did not become terminal within 180 seconds"
        }
        if ($job.provider_status -ne "succeeded") {
            throw "image job ended as $($job.provider_status)"
        }
    }
    finally {
        if ($client) {
            $client.Dispose()
        }
    }
}
finally {
    if ($apiProcess -and -not $apiProcess.HasExited) {
        Stop-Process -Id $apiProcess.Id -Force
    }
    $cleanupSQL = @"
UPDATE provider_model_routes SET current_revision_id=NULL
  WHERE id='route_adapter_shared_image';
DELETE FROM provider_model_route_revisions
  WHERE id='route_adapter_shared_image_r1';
DELETE FROM provider_model_routes
  WHERE id='route_adapter_shared_image';
DELETE FROM provider_credentials
  WHERE id='credential_adapter_shared_v1';
UPDATE provider_connections SET current_revision_id=NULL
  WHERE id='connection_adapter_shared';
DELETE FROM provider_connection_revisions
  WHERE id='connection_adapter_shared_r1';
DELETE FROM provider_connections
  WHERE id='connection_adapter_shared';
"@
    & docker exec `
        -e MYSQL_PWD=cookies_local_development_only `
        cookies-mysql-1 mysql -ucookies cookies -e $cleanupSQL 2>$null
    Pop-Location
}
