param(
    [string]$CsvPath = "C:\Users\Admin\Desktop\zz_model_provider_202607231201.csv"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot

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

Push-Location $repoRoot
try {
    if (-not (Test-Path -LiteralPath $CsvPath)) {
        throw "Provider CSV was not found: $CsvPath"
    }
    $rows = Import-Csv -LiteralPath $CsvPath
    $gatewayRow = $rows |
        Where-Object { $_.name -eq "artsapi (gpt-image-2)" } |
        Select-Object -First 1
    if (-not $gatewayRow -or
        [string]::IsNullOrWhiteSpace($gatewayRow.api_key) -or
        [string]::IsNullOrWhiteSpace($gatewayRow.base_url)) {
        throw "Shared Adapter gateway configuration is missing from the CSV"
    }

    $masterKey = [Environment]::GetEnvironmentVariable(
        "COOKIES_PROVIDER_MASTER_KEY",
        "User"
    )
    if ([string]::IsNullOrWhiteSpace($masterKey)) {
        $masterKey = New-MasterKey
        [Environment]::SetEnvironmentVariable(
            "COOKIES_PROVIDER_MASTER_KEY",
            $masterKey,
            "User"
        )
    }
    $keyVersion = [Environment]::GetEnvironmentVariable(
        "COOKIES_PROVIDER_MASTER_KEY_VERSION",
        "User"
    )
    if ([string]::IsNullOrWhiteSpace($keyVersion)) {
        $keyVersion = "local-clawex-v1"
        [Environment]::SetEnvironmentVariable(
            "COOKIES_PROVIDER_MASTER_KEY_VERSION",
            $keyVersion,
            "User"
        )
    }

    $mysqlPort = $env:COOKIES_MYSQL_PORT
    if ([string]::IsNullOrWhiteSpace($mysqlPort)) {
        $mysqlPort = [Environment]::GetEnvironmentVariable(
            "COOKIES_MYSQL_PORT",
            "User"
        )
    }
    if ([string]::IsNullOrWhiteSpace($mysqlPort)) {
        $mysqlPort = "3307"
    }
    $env:COOKIES_MYSQL_PORT = $mysqlPort
    [Environment]::SetEnvironmentVariable(
        "COOKIES_MYSQL_PORT",
        $mysqlPort,
        "User"
    )
    $dockerDSN = "cookies:cookies_local_development_only@tcp(127.0.0.1:$mysqlPort)/cookies?parseTime=true&multiStatements=true"
    [Environment]::SetEnvironmentVariable(
        "COOKIES_MYSQL_DSN",
        $dockerDSN,
        "User"
    )
    [Environment]::SetEnvironmentVariable(
        "COOKIES_PROVIDER_IMAGE_ADAPTER",
        "adapter_gateway",
        "User"
    )
    [Environment]::SetEnvironmentVariable(
        "COOKIES_PROVIDER_ALLOW_INSECURE_HTTP",
        "true",
        "User"
    )
    [Environment]::SetEnvironmentVariable(
        "COOKIES_PROVIDER_OUTPUT_BUCKET",
        "cookies-provider-output",
        "User"
    )
    $localSettings = @{
        "COOKIES_ENV" = "local"
        "COOKIES_BLOB_PROVIDER" = "filesystem"
        "COOKIES_FILESYSTEM_BLOB_ROOT" = ".data/blobs"
        "COOKIES_LOCAL_ORGANIZATION_ID" = "org_local"
        "COOKIES_LOCAL_PRINCIPAL_KIND" = "user"
        "COOKIES_LOCAL_PRINCIPAL_ID" = "user_local"
        "COOKIES_LOCAL_PROJECT_ID" = "project_local"
        "COOKIES_LOCAL_SCOPES" = "project.read,project.write,assets.read,assets.write,provider.job.create,provider.text.generate,provider.vision.understand,creative.read,creative.write,strategy.read,strategy.write,strategy.confirm,strategy.review,strategy.approve,strategy.package.read"
        "COOKIES_STRATEGY_ENABLED" = "true"
        "COOKIES_STRATEGY_REAL_PROVIDER_ENABLED" = "true"
        "COOKIES_STRATEGY_TEXT_MODEL_ALIAS" = "cookies.text.standard"
        "COOKIES_STRATEGY_CRITIC_ENABLED" = "true"
        "COOKIES_STRATEGY_APPROVE_ENABLED" = "true"
        "COOKIES_STRATEGY_PACKAGE_TO_CREATIVE_ENABLED" = "true"
        "COOKIES_PROVIDER_TEXT_ADAPTER" = "adapter_gateway"
    }
    foreach ($setting in $localSettings.GetEnumerator()) {
        [Environment]::SetEnvironmentVariable(
            $setting.Key,
            $setting.Value,
            "User"
        )
    }

    $env:COOKIES_ENV = "local"
    $env:COOKIES_MYSQL_DSN = $dockerDSN
    $env:COOKIES_PROVIDER_MASTER_KEY = $masterKey
    $env:COOKIES_PROVIDER_MASTER_KEY_VERSION = $keyVersion

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

    & go run ./cmd/cookies-migrate | Out-Null
    Assert-LastExitCode "database migration failed"

    $encrypted = ($gatewayRow.api_key |
        & go run ./cmd/cookies-provider-credential |
        ConvertFrom-Json)
    Assert-LastExitCode "Adapter credential encryption failed"

    $baseURL = $gatewayRow.base_url.TrimEnd("/") + "/v1"
    $connectionSQL = @"
UPDATE provider_model_routes SET current_revision_id=NULL
  WHERE id IN (
    'route_clawex_minimax_m27',
    'route_clawex_seedance2',
    'route_clawex_gpt_image_2',
    'route_clawex_seedance_20',
    'route_clawex_seed_2_pro',
    'route_clawex_seedance_15_pro',
    'route_clawex_seedream_5',
    'route_clawex_enhance_image',
    'route_clawex_remove_background',
    'route_cookies_image_standard',
    'route_cookies_text_standard'
  );
DELETE FROM provider_model_route_revisions
  WHERE route_id IN (
    'route_clawex_minimax_m27',
    'route_clawex_seedance2',
    'route_clawex_gpt_image_2',
    'route_clawex_seedance_20',
    'route_clawex_seed_2_pro',
    'route_clawex_seedance_15_pro',
    'route_clawex_seedream_5',
    'route_clawex_enhance_image',
    'route_clawex_remove_background',
    'route_cookies_image_standard',
    'route_cookies_text_standard'
  );
DELETE FROM provider_model_routes
  WHERE id IN (
    'route_clawex_minimax_m27',
    'route_clawex_seedance2',
    'route_clawex_gpt_image_2',
    'route_clawex_seedance_20',
    'route_clawex_seed_2_pro',
    'route_clawex_seedance_15_pro',
    'route_clawex_seedream_5',
    'route_clawex_enhance_image',
    'route_clawex_remove_background',
    'route_cookies_image_standard',
    'route_cookies_text_standard'
  );
DELETE FROM provider_credentials WHERE connection_id='connection_clawex_adapter';
UPDATE provider_connections SET current_revision_id=NULL
  WHERE id='connection_clawex_adapter';
DELETE FROM provider_connection_revisions
  WHERE connection_id='connection_clawex_adapter';
DELETE FROM provider_connections WHERE id='connection_clawex_adapter';

INSERT INTO provider_connections
  (id,connection_code,connection_type,current_revision_id,status)
  VALUES (
    'connection_clawex_adapter',
    'clawex-shared-adapter',
    'adapter_gateway',
    NULL,
    'enabled'
  );
INSERT INTO provider_connection_revisions
  (id,connection_id,revision_number,base_url,timeout_seconds,max_response_bytes,config_json)
  VALUES (
    'connection_clawex_adapter_r1',
    'connection_clawex_adapter',
    1,
    '$baseURL',
    210,
    41943040,
    JSON_OBJECT('source','clawex_csv','temporary_insecure_http',TRUE)
  );
UPDATE provider_connections
  SET current_revision_id='connection_clawex_adapter_r1'
  WHERE id='connection_clawex_adapter';
INSERT INTO provider_credentials
  (id,connection_id,credential_version,ciphertext,nonce,key_version,status,active_from)
  VALUES (
    'credential_clawex_adapter_v1',
    'connection_clawex_adapter',
    1,
    FROM_BASE64('$($encrypted.ciphertext_base64)'),
    FROM_BASE64('$($encrypted.nonce_base64)'),
    '$($encrypted.key_version)',
    'active',
    UTC_TIMESTAMP(6)
  );
"@

    $routes = @(
        @{
            ID = "route_clawex_minimax_m27"
            Capability = "text.generate"
            Alias = "MiniMax-M2.7"
            Model = "MiniMax-M2.7"
            Endpoint = "/v1/chat/completions"
            SourceProvider = "openai"
        },
        @{
            ID = "route_clawex_seedance2"
            Capability = "video.generate"
            Alias = "superAdmin"
            Model = "superAdmin"
            Endpoint = "/v1/videos/generations"
            SourceProvider = "ark"
        },
        @{
            ID = "route_clawex_gpt_image_2"
            Capability = "image.generate"
            Alias = "gpt-image-2"
            Model = "gpt-image-2"
            Endpoint = "/v1/images/generations"
            SourceProvider = "artsapi-gateway"
        },
        @{
            ID = "route_clawex_seedance_20"
            Capability = "video.generate"
            Alias = "doubao-seedance-2-0-fast-260128"
            Model = "doubao-seedance-2-0-fast-260128"
            Endpoint = "/v1/videos/generations"
            SourceProvider = "seedance-gateway"
        },
        @{
            ID = "route_clawex_seed_2_pro"
            Capability = "text.generate"
            Alias = "doubao-seed-2-0-pro-260215"
            Model = "doubao-seed-2-0-pro-260215"
            Endpoint = "/v1/chat/completions"
            SourceProvider = "ark"
        },
        @{
            ID = "route_clawex_seedance_15_pro"
            Capability = "video.generate"
            Alias = "doubao-seedance-1-5-pro-251215"
            Model = "doubao-seedance-1-5-pro-251215"
            Endpoint = "/v1/videos/generations"
            SourceProvider = "ark"
        },
        @{
            ID = "route_clawex_seedream_5"
            Capability = "image.generate"
            Alias = "doubao-seedream-5-0-260128"
            Model = "doubao-seedream-5-0-260128"
            Endpoint = "/v1/images/generations"
            SourceProvider = "ark"
        },
        @{
            ID = "route_clawex_enhance_image"
            Capability = "image.enhance"
            Alias = "enhance-image"
            Model = "enhance-image"
            Endpoint = "/api/v1/tools-sync/enhance-image"
            SourceProvider = "volc-mediakit"
        },
        @{
            ID = "route_clawex_remove_background"
            Capability = "image.background.remove"
            Alias = "remove-image-background"
            Model = "remove-image-background"
            Endpoint = "/api/v1/tools-sync/remove-image-background"
            SourceProvider = "volc-mediakit-matting"
        },
        @{
            ID = "route_cookies_image_standard"
            Capability = "image.generate"
            Alias = "cookies.image.standard"
            Model = "gpt-image-2"
            Endpoint = "/v1/images/generations"
            SourceProvider = "artsapi-gateway"
        },
        @{
            ID = "route_cookies_text_standard"
            Capability = "text.generate"
            Alias = "cookies.text.standard"
            Model = "doubao-seed-2-0-pro-260215"
            Endpoint = "/v1/chat/completions"
            SourceProvider = "ark"
            TextResponseMode = "prompt_json"
            MaxOutputTokens = 4096
            Temperature = 0.3
            ThinkingMode = "disabled"
        }
    )

    $routeSQL = New-Object Text.StringBuilder
    foreach ($route in $routes) {
        $revisionID = "$($route.ID)_r1"
        $textConstraints = ""
        if ($route.Capability -eq "text.generate") {
            $responseMode = if ($route.TextResponseMode) { $route.TextResponseMode } else { "prompt_json" }
            $maxOutputTokens = if ($route.MaxOutputTokens) { $route.MaxOutputTokens } else { 8000 }
            $temperature = if ($null -ne $route.Temperature) { $route.Temperature } else { 0.3 }
            $thinkingMode = if ($route.ThinkingMode) { $route.ThinkingMode } else { "" }
            $thinkingConstraint = if ($thinkingMode) { "      ,'thinking_mode','$thinkingMode'" } else { "" }
            $textConstraints = @"
      ,'text_response_mode','$responseMode'
      ,'max_output_tokens',$maxOutputTokens
      ,'temperature',$temperature
$thinkingConstraint
"@
        }
        [void]$routeSQL.AppendLine(@"
INSERT INTO provider_model_routes
  (id,organization_id,capability,model_alias,current_revision_id,status)
  VALUES (
    '$($route.ID)',NULL,'$($route.Capability)','$($route.Alias)',NULL,'enabled'
  );
INSERT INTO provider_model_route_revisions
  (id,route_id,revision_number,connection_id,connection_revision_id,upstream_model,constraints_json)
  VALUES (
    '$revisionID',
    '$($route.ID)',
    1,
    'connection_clawex_adapter',
    'connection_clawex_adapter_r1',
    '$($route.Model)',
    JSON_OBJECT(
      'endpoint','$($route.Endpoint)',
      'source_provider','$($route.SourceProvider)',
      'source','clawex_csv'
      $textConstraints
    )
  );
UPDATE provider_model_routes
  SET current_revision_id='$revisionID'
  WHERE id='$($route.ID)';
"@)
    }

    $sql = $connectionSQL + [Environment]::NewLine + $routeSQL.ToString()
    & docker exec -e MYSQL_PWD=cookies_local_development_only cookies-mysql-1 mysql `
        -ucookies cookies -e $sql
    Assert-LastExitCode "import Provider connection, credential, and routes failed"

    $catalog = & docker exec `
        -e MYSQL_PWD=cookies_local_development_only `
        cookies-mysql-1 mysql -N -B -ucookies cookies -e @"
SELECT r.capability,r.model_alias,rr.upstream_model,r.status
FROM provider_model_routes r
JOIN provider_model_route_revisions rr
  ON rr.id=r.current_revision_id AND rr.route_id=r.id
WHERE rr.connection_id='connection_clawex_adapter'
ORDER BY r.capability,r.model_alias;
"@
    Assert-LastExitCode "verify imported Provider catalog failed"
    Write-Output "Imported Provider routes:"
    Write-Output $catalog
    Write-Output "Shared Adapter credential: encrypted and active"
    Write-Output "Persistent user environment: configured (secret not displayed)"
}
finally {
    Pop-Location
}
