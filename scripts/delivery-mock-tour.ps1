param(
    [ValidateSet("Prepare", "Status", "Reset")]
    [string]$Action = "Prepare",
    [string]$BaseUrl = "http://127.0.0.1:8080",
    [string]$ProjectId = "project_local",
    [string]$RunId = "delivery-tour-local",
    [string]$Username = "",
    [string]$Password = "",
    [switch]$ConfirmReset
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $repoRoot ".env"

function Get-DotEnvValue([string]$Key, [string]$Fallback) {
    if (-not (Test-Path -LiteralPath $envFile)) { return $Fallback }
    $line = Get-Content -LiteralPath $envFile |
        Where-Object { $_ -match "^\s*$([regex]::Escape($Key))\s*=" } |
        Select-Object -First 1
    if ($null -eq $line) { return $Fallback }
    return (($line -split "=", 2)[1].Trim()).Trim('"').Trim("'")
}

if ($RunId -notmatch '^[a-z0-9][a-z0-9_-]{2,63}$') {
    throw "RunId must match ^[a-z0-9][a-z0-9_-]{2,63}$"
}
if ($Action -eq "Reset" -and -not $ConfirmReset) {
    throw "Reset deletes this run's exact causal closure. Re-run with -ConfirmReset after checking ProjectId and RunId."
}

$loginUsername = if ($Username) { $Username } else { Get-DotEnvValue "COOKIES_ADMIN_USERNAME" "Admin" }
$loginPassword = if ($Password) { $Password } else { Get-DotEnvValue "COOKIES_ADMIN_PASSWORD" "123456" }
$origin = $BaseUrl.TrimEnd('/')
$loginBody = @{ username = $loginUsername; password = $loginPassword } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri "$origin/platform/v1/auth/login" -ContentType "application/json" `
    -Body $loginBody -SessionVariable cookiesSession | Out-Null

$encodedProject = [uri]::EscapeDataString($ProjectId)
$encodedRun = [uri]::EscapeDataString($RunId)
$runUrl = "$origin/api/delivery/v1/projects/$encodedProject/tour-runs/$encodedRun"
$method = "Get"
$requestUrl = $runUrl
if ($Action -eq "Prepare") {
    $method = "Post"
    $requestUrl = "${runUrl}:prepare"
}
if ($Action -eq "Reset") {
    $method = "Post"
    $requestUrl = "${runUrl}:reset"
}

$result = Invoke-RestMethod -WebSession $cookiesSession -Method $method -Uri $requestUrl
$run = if ($Action -eq "Reset") { $result.run } else { $result }
$summary = [ordered]@{
    action = $Action.ToLowerInvariant()
    project_id = $ProjectId
    run_id = $run.id
    owner_id = $run.owner_id
    status = $run.status
    source = $run.source
    case_count = @($run.cases).Count
    completed_steps = @($run.steps | Where-Object { $_.complete }).Count
    next_url = $run.suggested_next_url
}
if ($Action -eq "Reset") {
    $summary.deleted = $result.deleted
    $summary.isolation_key = $result.isolation_key
}
$summary | ConvertTo-Json -Depth 8
