$ErrorActionPreference = 'Stop'

function Invoke-Go {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)

    & go @Arguments
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}

function Invoke-Npm {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)

    & npm @Arguments
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}

& git diff --check
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

$goFiles = Get-ChildItem -Path .\cmd, .\internal -Recurse -File -Filter *.go |
    ForEach-Object { $_.FullName }

$unformatted = @($goFiles | ForEach-Object {
    & gofmt -l $_
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
})
if ($unformatted.Count -gt 0) {
    $unformatted | ForEach-Object { Write-Error "Unformatted Go file: $_" }
    exit 1
}

Invoke-Go vet ./...
Invoke-Go test ./...
Invoke-Go build ./cmd/cookies-api
Invoke-Go build ./cmd/cookies-migrate

Get-ChildItem -Path .\api\events, .\api\contracts, .\api\fixtures -File -Filter *.json | ForEach-Object {
    [System.IO.File]::ReadAllText($_.FullName) | ConvertFrom-Json | Out-Null
}

if (-not (Test-Path -LiteralPath '.\node_modules')) {
    Write-Error 'Frontend dependencies are missing. Run npm ci first.'
    exit 1
}

Invoke-Npm run check:server
Invoke-Npm test
Invoke-Npm run test:server
Invoke-Npm run build
Invoke-Npm run contract:check
