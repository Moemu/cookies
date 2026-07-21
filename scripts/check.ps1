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

if (-not (Test-Path -LiteralPath '.\web\node_modules')) {
    Write-Error 'Frontend dependencies are missing. Run npm ci --prefix web first.'
    exit 1
}

& npm run check --prefix web
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

& npm run contract:check --prefix web
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
