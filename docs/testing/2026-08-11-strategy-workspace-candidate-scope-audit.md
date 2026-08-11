# Strategy Workspace candidate scope audit

Date: 2026-08-11

Decision: the restored working tree is internally coherent, contains no identified out-of-scope downstream production change and is staged as one exact local submission candidate, but it is **not an immutable release candidate**. Nothing in this audit authorizes committing, pushing, another paid provider call, a production migration or deployment.

## Repository baseline

- Branch: `codex/merge-mvp-main-20260804`.
- Current HEAD: `3503b515f4f61163e135b13a512c0ff64c33ecc6`.
- Merge parents: local `46cd0531564d54979b1de0e8d0def74731d0c2a2` and `origin/main` `5e4fd3783c1c55a0a5a384fba29c135e74802d4f`.
- The merge commit only synchronizes upstream code. The Strategy implementation is now staged separately as the exact audited local candidate.
- Safety snapshot retained: `042861e585a23ad46a1d0b5360f3cb2c6f517524` (`codex-pre-origin-main-merge-20260811`). It must not be dropped until the restored scope is represented by an intentionally reviewed immutable candidate or the user explicitly decides otherwise.
- Staged paths after the final refresh: 278. Unstaged paths: zero. Untracked paths: zero. Unmerged paths: zero.

## Complete path inventory

The full-file inventory contains 278 unique paths: 110 modifications and 168 additions relative to `HEAD`.

| Scope group | Files | Why it belongs |
| --- | ---: | --- |
| Strategy product | 99 | Five-stage workspace, Assistant, Activity, Brief/Strategy v3, review, immutable handoff, product metrics, API and browser/unit coverage. |
| Research and documents | 40 | Deep-research orchestration, source verification, document parse quality/progress/preview, conversion, visual fallback, reconciliation and tests. |
| Platform backend | 38 | API wiring, configuration, job recovery, identity isolation, provider capability and object-scope safety. |
| Contracts and fixtures | 35 | Versioned schemas and deterministic valid/partial/stale fixtures for the new boundaries. |
| Documentation | 22 | Approved plan, per-requirement ledger, phase evidence, evaluation protocol, runbooks and release/adversarial audits. |
| Migrations | 19 | Additive Platform, Provider and Strategy schema changes required by the implemented data contracts, including irreversible cleanup of legacy LAS temporary links. |
| Shared frontend and tests | 16 | Stable application routing, shell/navigation prominence, project bootstrap, shared types, stale-request protection in Delivery monitoring and cross-feature regression coverage with an explicit long-journey timeout budget. |
| Repository operations | 9 | Example configuration, one-bucket local profile, CI-parity/bundle/test gates, Playwright profile, safe LAS configuration helper, generated-output exclusion and line-ending policy. |

Sorted path-manifest SHA-256: `731fa146fa70774b94560f0430761ed4548a0523a08c20b163777425e904723f`.

## Shared-file rationale

The following broad paths were reviewed because they are the most likely place for accidental scope expansion:

- `.env.example`, `deployments/docker-compose.yml`, `internal/platform/config/*`: define one private TOS bucket, isolated object prefixes, default-off document vision/conversion, fixed model routes and the local conversion profile.
- `.gitattributes`, `internal/platform/migration/*`: keep SQL identities canonical across Windows/Linux while accepting only exact historical LF/CRLF variants and rejecting actual migration-content changes.
- `package.json`, `playwright.platform.config.ts`, `scripts/*`: retain upstream Delivery contract coverage while adding Strategy contract, browser and bundle-budget gates. The local CI-parity script now checks all event/contract/fixture JSON as UTF-8, avoids the Windows command-length limit during `gofmt`, and includes frontend plus contract tests.
- `cmd/cookies-api/main.go`, `internal/platform/httpserver/*`, `internal/platform/jobruntime/*`, `internal/platform/provider/*`: register the new APIs and durable activities without introducing a second job/runtime truth source.
- `src/App.tsx`, `src/components/*`, `src/context/ProjectContext.tsx`, `src/data/*`, `src/lib/router.ts`, `src/styles.css`, `src/types.ts`: provide the stable five-stage route, stronger center presence, progressive project hydration and cross-stage shell behavior.
- `e2e/strategy-brand-video-foundation.spec.ts`: updates the pre-existing Strategy foundation journey from the removed conversation tab to the canonical `intake` stage; it does not alter downstream production behavior.
- `src/components/DeliveryMonitoringPage.tsx`: prevents an older initial-load request from overwriting a newer simulation or alert-evaluation result. The full candidate E2E exposed this existing cross-feature race; the fix is limited to client request sequencing and does not change Delivery contracts or backend behavior.
- `e2e/delivery-approval-content-hash.spec.ts`: gives the existing two-version approval journey a 60-second scenario budget. Its assertions and network contracts are unchanged; the previous 30-second default terminated a normally progressing Windows run during the second ChangeSet cycle.

## Explicit exclusions and hygiene findings

- No changed path exists under `internal/systems/delivery` or a downstream image, video, or audio production module.
- Creative handoff changes stop at the read-only StrategyPackage/task-overlay boundary; the downstream production state machine remains unchanged.
- No changed path is inside `dist`, `build`, `coverage`, `test-results`, `playwright-report`, `node_modules`, `vendor`, temporary directories, generated `output`, or log/backup artifacts. `/output/` is explicitly ignored so the LAS canary PDF cannot enter the candidate accidentally.
- No untracked file exceeds 1 MiB.
- A path-complete text scan found no private-key block, hardcoded cloud secret, bearer token, hardcoded password, or unfinished-work marker.
- Configuration examples contain names/placeholders only. Readiness output is restricted to fixed blocker codes and booleans.

## Merge integration review

- The sole textual merge conflict was `package.json`; its resolution keeps both upstream Delivery contract tests and the Strategy contract/bundle-budget checks.
- The upstream Strategy foundation E2E now asserts the stable five-stage navigation and canonical `/intake` URL.
- SQL files are declared `eol=lf`; the migrator's canonical checksum is CRLF/LF tolerant for exact equivalent historical bytes. One exact pre-candidate checksum for the document-parse migration is allowlisted because it differs only by one verified terminal LF; the compatibility is migration- and hash-specific, while BOM, lone-CR, unknown blank-line or statement changes remain fail-closed.
- All 278 audited paths are staged, no path remains unstaged or untracked, and the pre-merge safety stash is still available.

## Verified combined gates

After merging `origin/main` and restoring the Strategy tree, the combined repository passed:

- `go vet ./...`;
- `go test ./...`;
- 205/205 frontend tests;
- 37/37 compatibility-server tests;
- 18/18 contract tests;
- production frontend build and all bundle budgets;
- a complete 20/20 platform Playwright run covering upstream Delivery plus Strategy journeys;
- real-MySQL migration execution and the historical-CRLF checksum upgrade case;
- `git diff --check`.

The hardened `scripts/check.ps1` then passed as one local CI-parity entry, including formatting, JSON, Go vet/test/build, compatibility-server checks, 205 frontend tests, 37 compatibility-server tests, production build budgets and 18 contract tests. The exact post-migration run completed in 58.3 seconds. LAS Markdown now removes remote image resources and signed temporary URLs before persistence, a forward migration cleans historical document-vision chunks and recomputes their hashes, Playwright explicitly disables document vision/conversion in its filesystem-backed process, and Delivery monitoring rejects stale initial-load results after a newer user operation.

The exact post-migration 20/20 browser run completed in 153.4 seconds and measured 376 ms local LCP, 84.3 ms maximum center interaction, 2,754.0 ms dense-message readiness and 407.5 ms maximum dense-panel transition. It ran directly against the normal local environment without manually overriding LAS/TOS variables. These are local regression measurements, not production percentiles or evidence of human time saved.

## Remaining admission boundaries

This audit proves scope coherence, not release completion. Production admission remains blocked by evidence or authorization outside this working-tree review:

1. There is no immutable Strategy candidate commit and therefore no exact-candidate GitHub Actions result.
2. The local Shanghai one-bucket TOS profile, encrypted LAS route and credential are configured, and one bounded real PDF canary completed with one billable page. Production/canary-environment credential rollout and a real PPTX-derived LAS request remain external release evidence.
3. The deidentified blinded document corpus and independent correction-time evidence do not yet exist; automatic visual fallback remains off.
4. The two-to-four-week product workflow baseline does not exist; `time_saving_measured=false` remains required.
5. A production-like migration rehearsal with approved real row-count distributions and rollback observation has not been authorized or supplied.
6. The Windows host cannot run the Linux/CGO race gate; `go test -race ./...` remains an exact-candidate CI requirement.

Before any future candidate commit, recompute the staged inventory and manifest hash against `HEAD`, verify that no unstaged or untracked path exists, review the staged diff against this document, and rerun any gate invalidated by subsequent edits. Commit or push still requires separate explicit authorization.
