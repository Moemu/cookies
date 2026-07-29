[CmdletBinding()]
param(
    [switch]$PrepareOnly
)

$ErrorActionPreference = 'Stop'

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path

function Assert-Command {
    param([Parameter(Mandatory = $true)][string]$Name)

    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command '$Name' was not found in PATH."
    }
}

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)][string]$Command,
        [Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments
    )

    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed with exit code ${LASTEXITCODE}: $Command $($Arguments -join ' ')"
    }
}

Assert-Command docker
Assert-Command go
Assert-Command npm

Push-Location $repositoryRoot
try {
    $mysqlPort = $env:COOKIES_MYSQL_PORT
    if ([string]::IsNullOrWhiteSpace($mysqlPort)) {
        $mysqlPort = '3307'
    }
    $env:COOKIES_MYSQL_PORT = $mysqlPort
    if ([string]::IsNullOrWhiteSpace($env:COOKIES_MYSQL_DSN)) {
        $env:COOKIES_MYSQL_DSN = "cookies:cookies_local_development_only@tcp(127.0.0.1:$mysqlPort)/cookies?parseTime=true&multiStatements=true"
    }

    Write-Host 'Starting MySQL and waiting for it to become healthy...'
    Invoke-Checked docker compose up -d --wait mysql

    Write-Host 'Applying database migrations and seeding the canonical Go demo...'
    Invoke-Checked go run ./cmd/cookies-seed

    if (-not (Test-Path (Join-Path $repositoryRoot 'node_modules'))) {
        Write-Host 'Installing frontend dependencies (first run only)...'
        Invoke-Checked npm ci
    }

    $env:COOKIES_ENV = 'local'
    $env:COOKIES_HTTP_ADDR = ':8080'
    $env:COOKIES_LOCAL_ORGANIZATION_ID = 'org_local'
    $env:COOKIES_LOCAL_PRINCIPAL_KIND = 'user'
    $env:COOKIES_LOCAL_PRINCIPAL_ID = 'user_local'
    $env:COOKIES_LOCAL_PROJECT_ID = 'project_local'
    $env:COOKIES_LOCAL_SCOPES = 'project.read,project.write,assets.read,assets.write,provider.job.create,provider.text.generate,provider.vision.understand'
    $env:COOKIES_BLOB_PROVIDER = 'filesystem'
    $env:COOKIES_FILESYSTEM_BLOB_ROOT = (Join-Path $repositoryRoot '.data\blobs')
    $env:COOKIES_SCANNER_MODE = 'noop'

    if ($PrepareOnly) {
        Write-Host 'Development dependencies and Go demo seed are ready.'
        return
    }

    $shell = Get-Command pwsh -ErrorAction SilentlyContinue
    if (-not $shell) {
        $shell = Get-Command powershell.exe -ErrorAction Stop
    }
    $shellArguments = @('-NoLogo', '-NoProfile', '-NoExit', '-Command')

    $backend = Start-Process `
        -FilePath $shell.Source `
        -ArgumentList ($shellArguments + @('go run ./cmd/cookies-api')) `
        -WorkingDirectory $repositoryRoot `
        -PassThru

    Start-Sleep -Seconds 1

    $frontend = Start-Process `
        -FilePath $shell.Source `
        -ArgumentList ($shellArguments + @('npm run dev -- --host 127.0.0.1')) `
        -WorkingDirectory $repositoryRoot `
        -PassThru

    Write-Host ''
    Write-Host "Backend started in a new window (PID $($backend.Id)): http://127.0.0.1:8080"
    Write-Host "Frontend started in a new window (PID $($frontend.Id)): http://localhost:5173"
    Write-Host 'Close those windows or press Ctrl+C in each one to stop the application.'
    Write-Host 'Run "docker compose stop mysql" when you also want to stop MySQL.'
}
finally {
    Pop-Location
}
