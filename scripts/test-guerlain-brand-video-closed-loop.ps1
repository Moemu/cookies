[CmdletBinding()]
param(
    [switch]$SkipFrontendBuild,
    [switch]$RunBrowserFoundation
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)][string]$Label,
        [Parameter(Mandatory = $true)][scriptblock]$Command
    )

    Write-Host "[closed-loop] $Label"
    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "$Label failed with exit code $LASTEXITCODE."
    }
}

Push-Location $repoRoot
try {
    Invoke-Checked "Project/Brief context gate" {
        & go test ./internal/systems/strategy -run "TestProjectBriefCompatibility|TestCreativeHandoff" -count=1
    }
    Invoke-Checked "Strategy-to-delivery brand-video spine" {
        & go test ./internal/systems/creative -run TestStrategyBrandVideoClosedLoop -count=1
    }
    Invoke-Checked "Creative task planner route repair" {
        & .\node_modules\.bin\tsx.cmd --test test/creative-task-planner.test.ts
    }
    if (-not $SkipFrontendBuild) {
        Invoke-Checked "Frontend production build" {
            & npm.cmd run build
        }
    }
    if ($RunBrowserFoundation) {
        Invoke-Checked "Browser/API foundation" {
            & npx.cmd playwright test e2e/strategy-brand-video-foundation.spec.ts --config=playwright.platform.config.ts
        }
    }
    Write-Host "[closed-loop] All selected acceptance gates passed."
}
finally {
    Pop-Location
}
