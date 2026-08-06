$ErrorActionPreference = 'Stop'

function Invoke-Go {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)

    & go @Arguments
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}

$goFiles = Get-ChildItem -Path . -Recurse -File -Filter *.go |
    Where-Object { $_.FullName -notmatch '[\\/]third_party[\\/]' } |
    ForEach-Object { $_.FullName }

$unformatted = @(& gofmt -l $goFiles)
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
if ($unformatted.Count -gt 0) {
    $unformatted | ForEach-Object { Write-Error "Unformatted Go file: $_" }
    exit 1
}

Invoke-Go vet ./...
Invoke-Go test ./...
Invoke-Go build ./cmd/cookies-api
Invoke-Go build ./cmd/cookies-migrate

Get-ChildItem -Path .\api\events -File -Filter *.json | ForEach-Object {
    Get-Content -Raw -LiteralPath $_.FullName | ConvertFrom-Json | Out-Null
}

if (-not (Test-Path -LiteralPath '.\node_modules')) {
    Write-Error 'Frontend dependencies are missing. Run npm ci first.'
    exit 1
}

& npm run check:server
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

& npm run test:server
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

& npm run build
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
