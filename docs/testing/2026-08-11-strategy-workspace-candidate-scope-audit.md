# Strategy Workspace candidate scope audit

Date: 2026-08-11

Decision: the combined tree is internally coherent, contains the separately authorized Kanon creative integration but no unidentified scope expansion, and is represented by an intentionally split local Strategy commit plus this Kanon merge. It is **not a release-admitted candidate** until required GitHub Actions checks pass. Nothing in this audit authorizes pushing, another paid provider call, a production migration or deployment.

## Repository baseline

- Branch: `codex/merge-mvp-main-20260804`.
- Pre-candidate baseline: `3503b515f4f61163e135b13a512c0ff64c33ecc6`.
- Strategy/LAS candidate commit: `aa857769906d4f35dbdc2f0ff39c6f24bd8d9317` (278 paths).
- This audit accompanies the following merge of Kanon's completed integration branch `origin/codex/sync-completed-20260810` at `2c90c71d800b6e340b6fa4c6d01a6091dedb66a5` (nine commits).
- Safety snapshots retained: `8e6794adb699c2b0fd4adc99b698a6e8165cbed3` (`codex-pre-kanon-main-merge-20260811`) for the exact 278-path pre-Kanon candidate and `042861e585a23ad46a1d0b5360f3cb2c6f517524` (`codex-pre-origin-main-merge-20260811`) for the earlier upstream synchronization. They must not be dropped until the restored scope is represented by an intentionally reviewed immutable candidate or the user explicitly decides otherwise.
- Combined candidate paths at the pre-commit freeze: 294. Unstaged paths: zero. Untracked paths: zero. Unmerged paths: zero.

## Complete path inventory

The full-file inventory contains 294 unique paths: 124 modifications and 170 additions relative to the pre-candidate baseline.

| Scope group | Files | Why it belongs |
| --- | ---: | --- |
| Strategy product | 99 | Five-stage workspace, Assistant, Activity, Brief/Strategy v3, review, immutable handoff, product metrics, API and browser/unit coverage. |
| Research and documents | 40 | Deep-research orchestration, source verification, document parse quality/progress/preview, conversion, visual fallback, reconciliation and tests. |
| Platform backend | 40 | API wiring, configuration, job recovery, identity isolation, provider capability and object-scope safety, including Kanon's shared text-route transport normalization. |
| Contracts and fixtures | 35 | Versioned schemas and deterministic valid/partial/stale fixtures for the new boundaries. |
| Documentation | 22 | Approved plan, per-requirement ledger, phase evidence, evaluation protocol, runbooks and release/adversarial audits. |
| Migrations | 19 | Additive Platform, Provider and Strategy schema changes required by the implemented data contracts, including irreversible cleanup of legacy LAS temporary links. |
| Shared frontend and tests | 17 | Stable application routing, shell/navigation prominence, project bootstrap, shared types/API, stale-request protection in Delivery monitoring and cross-feature regression coverage with an explicit long-journey timeout budget. |
| Repository operations | 11 | Example configuration, one-bucket local profile, CI-parity/bundle/test gates, Playwright profile, safe LAS/Seed configuration helpers, generated-output exclusion and line-ending policy. |
| Kanon creative integration | 11 | Latest completed Kanon changes for brand-film and short-drama source authority, persisted creative media access, analyzers and focused regression coverage. |

Sorted path-manifest SHA-256: `ab67c3996de3f14b41ca726f6355b31f6b2011fe37ff2d5ed32f530e2f20fe9b`.

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

- No changed path exists under `internal/systems/delivery`. The authorized Kanon synchronization adds bounded changes to existing brand-film and short-drama creative/media paths; these are classified separately above and do not expand the Strategy implementation into new production stages.
- Creative handoff changes stop at the read-only StrategyPackage/task-overlay boundary; the downstream production state machine remains unchanged.
- No changed path is inside `dist`, `build`, `coverage`, `test-results`, `playwright-report`, `node_modules`, `vendor`, temporary directories, generated `output`, or log/backup artifacts. `/output/` is explicitly ignored so the LAS canary PDF cannot enter the candidate accidentally.
- No untracked file exceeds 1 MiB.
- A path-complete text scan found no private-key block, hardcoded cloud secret, bearer token, hardcoded password, or unfinished-work marker.
- Configuration examples contain names/placeholders only. Readiness output is restricted to fixed blocker codes and booleans.

## Merge integration review

- The Kanon integration branch changed 23 paths relative to the common `origin/main` baseline. Twenty paths change the final candidate; three overlapping Strategy components deliberately retain the newer, already-accepted workspace implementation because the Kanon side was a pre-rearchitecture UI restoration that would remove Assistant, session recovery, complete Brief and direct brand handoff behavior.
- Five stash-restoration conflicts were resolved: `gateway_config.go` keeps both Kanon's text/vision transport ceilings and the LAS document-vision route constraints; the three Strategy components keep the newer implementation; `src/styles.css` keeps Kanon's non-Strategy creative styles and the new Strategy workspace styles. No unmerged path or conflict marker remains.
- The upstream Strategy foundation E2E now asserts the stable five-stage navigation and canonical `/intake` URL.
- SQL files are declared `eol=lf`; the migrator's canonical checksum is CRLF/LF tolerant for exact equivalent historical bytes. One exact pre-candidate checksum for the document-parse migration is allowlisted because it differs only by one verified terminal LF; the compatibility is migration- and hash-specific, while BOM, lone-CR, unknown blank-line or statement changes remain fail-closed.
- All 294 audited paths are represented by the two local commit boundaries, and both safety stashes remain available pending push/CI confirmation.

## Verified combined gates

After merging `origin/main` and restoring the Strategy tree, the combined repository passed:

- `go vet ./...`;
- `go test ./...`;
- 208/208 frontend tests;
- 37/37 compatibility-server tests;
- 18/18 contract tests;
- production frontend build and all bundle budgets;
- a complete 20/20 platform Playwright run covering upstream Delivery plus Strategy journeys;
- real-MySQL migration execution and the historical-CRLF checksum upgrade case;
- `git diff --check`.

After the Kanon merge resolution, the hardened `scripts/check.ps1` passed again as one local CI-parity entry, including formatting, JSON, Go vet/test/build, compatibility-server checks, 208 frontend tests, 37 compatibility-server tests, production build budgets and 18 contract tests. This merge-final run completed in 65.7 seconds. The production entry measured 423.93 kB, `Pages` 112.92 kB and the Strategy workspace route 214.07 kB, all within enforced budgets.

The merge-final 20/20 browser run completed in 149.2 seconds and measured 308 ms local LCP, 108.5 ms maximum center interaction and 1,666.4 ms dense-message readiness; the slowest dense supporting-panel readiness was 186.0 ms. It ran directly against the normal local environment without manually overriding LAS/TOS variables. These are local regression measurements, not production percentiles or evidence of human time saved.

## Remaining admission boundaries

This audit proves scope coherence, not release completion. Production admission remains blocked by evidence or authorization outside this working-tree review:

1. The local Strategy and Kanon merge commits exist, but there is no exact-candidate GitHub Actions result until the branch is pushed with authorization.
2. The local Shanghai one-bucket TOS profile, encrypted LAS route and credential are configured, and one bounded real PDF canary completed with one billable page. Production/canary-environment credential rollout and a real PPTX-derived LAS request remain external release evidence.
3. The deidentified blinded document corpus and independent correction-time evidence do not yet exist; automatic visual fallback remains off.
4. The two-to-four-week product workflow baseline does not exist; `time_saving_measured=false` remains required.
5. A production-like migration rehearsal with approved real row-count distributions and rollback observation has not been authorized or supplied.
6. The Windows host cannot run the Linux/CGO race gate; `go test -race ./...` remains an exact-candidate CI requirement.

Before any push, verify the clean two-commit boundary and final tree identity, review the diff against this document, rerun any gate invalidated by subsequent edits, and then monitor every required GitHub Actions check to green. Push still requires separate explicit authorization.
