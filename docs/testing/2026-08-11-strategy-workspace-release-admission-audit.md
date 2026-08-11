# Strategy Workspace — Release admission and adversarial audit

> Audit date: 2026-08-11
> Decision: **HOLD — not admitted for production release**
> Scope: Strategy workspace UX rearchitecture, Assistant/Activity, deep research, Brief/Strategy/Review/Handoff, document parse/vision/PPT conversion, metrics and release safety
> Excluded by product decision: deep changes to downstream image/video/audio production

## 1. What is locally implemented

| Area | Local state | Evidence |
| --- | --- | --- |
| Stable workflow and navigation | Implemented | URL-authoritative `intake/brief/strategy/review/handoff`, contextual research/material/task/history panels, focus/scroll protections and desktop/mobile QA |
| Cross-stage AI Assistant | Implemented | Project context manifest, bounded memory, proposal adopt/edit/ignore flow, persistent immersive workspace mode, Activity acknowledgments and no exposed chain-of-thought |
| Deep research | Implemented | Non-blocking persisted rounds, bounded ReAct-style loop, source verification states, findings and explicit adoption diff |
| Brief and Strategy | Implemented | grouped decisions, visible 750 ms server autosave separated from explicit confirmation, AI suggestions, Strategy v3, structured creative strategy, section-level revision and second perspective |
| Review and Handoff | Implemented | atomic self-confirm path, formal role-based review, immutable StrategyPackage and handoff-only task overlay |
| Document parse UX | Implemented | truthful stages/page progress, preview, quality summary, partial/retry and original-text preservation |
| PDF visual fallback | Implemented, default-off/manual | fixed route inspection, exact selected pages, durable paid-task checkpoints, uncertainty reconciliation boundary and hybrid page merge |
| PPT/PPTX input conversion | Implemented, default-off | separate Gotenberg/LibreOffice adapter, immutable derived PDF, source/derived SHA lineage and three-attempt local ceiling |
| UX metrics | Implemented | privacy-bounded product events and project-scoped read model; product time-saving remains explicitly unmeasured |
| Offline vision evaluation | Implemented | versioned dataset/report, strict CLI, source/page deduplication, two independent blinded review sessions, independent adjudication, correction timing and cost/version lineage |
| Submission-unknown reconciliation | Implemented locally | pre-call intent, safe candidate/read APIs, administrator-only two-person proposal/confirmation, unique external-task binding and recovery-safe polling resume |

## 2. Latest local quality gates

The latest complete local gate run passed:

- `go test ./...`;
- `go vet ./...`;
- `npm test`: 208/208 passed after the Kanon integration merge;
- `npm run contract:check`;
- `npm run build`;
- `git diff --check`;
- targeted CLI, evaluator and JSON Schema suites after the final anti-duplication/adjudication hardening.

A 2026-08-11 CI-parity follow-up also passed the repository's `gofmt` path check, UTF-8 JSON parsing for every event/contract/fixture, explicit-exit-code `go vet ./...` and non-race `go test ./...`, both Go command builds, compatibility-server type checking and 37/37 server tests, 198/198 frontend tests, production build budgets, contract checks, and `git diff --check`. The Strategy browser suite then passed 11/11 again in 77.8 seconds; this run recorded 324 ms local LCP, 130 ms maximum center interaction, 1,851 ms dense-message readiness and 226 ms maximum dense-panel transition. The release runbook was corrected to match the actual CI commands and the implemented two-administrator document-vision reconciliation boundary. The Windows host has `CGO_ENABLED=0` and no C compiler, so `go test -race ./...` was not executed locally and remains an exact-candidate Linux CI gate; it is not counted as a local pass.

After the authorized `origin/main` synchronization, local merge commit `3503b515f4f61163e135b13a512c0ff64c33ecc6` joined prior local commit `46cd0531564d54979b1de0e8d0def74731d0c2a2` with upstream `5e4fd3783c1c55a0a5a384fba29c135e74802d4f`. The Strategy change set was restored without staging; the one `package.json` conflict retained both upstream Delivery contract coverage and the Strategy bundle budget. Combined verification passed `go vet ./...`, `go test ./...`, 204/204 frontend tests, 37/37 compatibility-server tests, 18/18 contract tests and the production bundle budget. A complete platform browser run then passed 20/20 in 115 seconds, covering the upstream Delivery journeys, the brand Strategy foundation and all Strategy workspace journeys in one MySQL lifecycle. It recorded 344 ms local LCP, 104 ms maximum center interaction, 1,760 ms dense-message readiness and 247 ms maximum dense-panel transition.

The merge also exposed a Windows checkout hazard in content-addressed migrations: an untracked LF migration restored through Git stash became CRLF and correctly tripped the prior raw-byte checksum. SQL files are now declared `eol=lf`, while the migrator defines one canonical LF identity, recognizes only the exact legacy raw LF/CRLF variants, atomically upgrades a matching historical checksum, and still rejects statement, BOM, lone-CR or other byte changes. Unit tests, a real-MySQL historical-CRLF upgrade test, the full migration command and the 20-test browser run all passed after this hardening.

The production entry is now 423.93 kB minified / 124.08 kB gzip, down from 1,121.12 kB / 316.64 kB (62.2% and 60.8% smaller respectively). `Pages` is 112.92 kB, the Strategy workspace route is 214.07 kB, and the 288.78 kB specialized-page bundle is lazy. `npm run build` enforces 450 kB entry, 140 kB `Pages`, 230 kB Strategy-route and 500 kB per-chunk ceilings, so this boundary can no longer regress as an advisory-only warning. These are transfer/build measurements, not a human time-saving claim.

Additional local evidence already recorded by the phase reports:

- applied migration files are now content-addressed in `platform_schema_migrations`; a changed applied probe is rejected by a real-MySQL test, and a separate forward compatibility migration repairs the earlier document-vision intent schema drift without editing production facts in place;
- clean MySQL full migration rehearsal passed and its temporary database was removed;
- real MySQL Strategy/Knowledge/provider vertical slices passed for the implemented boundaries;
- local official Gotenberg 8.34.0 container health and a real PPTX-to-PDF adapter integration passed, then the service was stopped;
- desktop and 390 px mobile browser QA passed for the Strategy shell and four centers;
- a dedicated real-browser suite passed 11/11 in its final 68.3-second rerun, covering the prominent four-center hub, keyboard-only skip/center/stage navigation, one-sentence grounded requirement extraction, 750 ms server autosave without silent high-risk confirmation, per-stage scroll plus session-scoped unsaved input recovery across all five stages and the Research panel, native-text and low-quality-PDF parsing, successor Brief v2, Strategy Revision 1, unsaved-chapter publish blocking, solo self-confirmation, immutable StrategyPackage v1, explicit commerce-pre-roll Route selection, task-plan answer recovery and completion, task-level Overlay generation, hash-stable Handoff reload, designated-approver policy and Assignment rendering, Review comment recovery and decision blocking, stale Revision/hash rejection followed by successful successor approval, non-blocking Deep Research with Activity-backed rounds, Markdown report download and explicit Brief adoption, Assistant next-turn source exclusion with authoritative-context preservation, persistent immersive mode and Escape recovery, Activity SSE disconnect plus bounded automatic recovery, repeated center-request 503 recovery without shell freeze, long Chinese/high-density state without horizontal overflow, reduced motion, and 1440/1280/1024/768/390 px breakpoints. The same run records 832 ms local LCP, 256 ms maximum center interaction, 1,712 ms dense-message readiness and 193 ms maximum dense-panel transition; these are regression evidence, not production percentiles. The navigation journey also proves that the specialized-page bundle is not requested in the four Strategy centers, and the formal-review journey guards against a slow policy response overwriting active user edits;
- the generic approved pre-roll projection now becomes a stable commerce-pre-roll Handoff candidate only when the approved objective/channel evidence supports it; missing product lineage remains blocked and explicit game, short-drama or remake language fails closed instead of being mislabeled as commerce;
- optional second-perspective reads now use a successful empty response before analysis exists, eliminating normal-workflow 404 console noise without hiding actual endpoint failures;
- Project bootstrap now resolves the requested Project first and hydrates remaining Project records progressively in two-item batches; the Project list endpoint no longer causes a blocking detail request per item. Research also has one Activity-backed status path rather than competing Activity and component pollers;
- no production migration, deploy or push was performed; after explicit operator authorization, one bounded real LAS PDF canary completed locally through the Shanghai one-bucket TOS profile. The Strategy implementation is now represented by local commit `aa857769906d4f35dbdc2f0ff39c6f24bd8d9317`, followed by the Kanon merge containing this audit; neither has been pushed or covered by required CI.

A final submission-readiness follow-up passed `scripts/check.ps1` in 58.3 seconds and the combined platform browser suite 20/20 in 153.4 seconds after the historical sanitization migration was added. It also closed three locally discovered reliability defects: expiring LAS image URLs are removed from page and aggregate Markdown before persistence, the filesystem-backed Playwright service cannot inherit enabled LAS/converter flags from `.env`, and stale Delivery monitoring loads cannot overwrite a newer simulation or alert result. The browser suite ran without manual environment overrides and recorded 376 ms local LCP, 84.3 ms maximum center interaction, 2,754.0 ms dense-message readiness and 407.5 ms maximum dense-panel transition.

The subsequent authorized Kanon integration merges `origin/codex/sync-completed-20260810` at `2c90c71d800b6e340b6fa4c6d01a6091dedb66a5` as a distinct second-parent boundary. Five conflicts were resolved while retaining both the fixed provider transport limits and document-vision constraints, the newer Strategy workspace behavior, and Kanon's non-Strategy creative/media changes. The merge-final `scripts/check.ps1` passed in 65.7 seconds and the platform browser suite passed 20/20 in 149.2 seconds; the combined candidate contains 294 paths with no unstaged, untracked or unmerged path at the pre-commit freeze.

## 3. Release blockers

### B1 — PDF canary passed; PPTX-derived LAS proof is still missing

A bounded one-page PDF request completed against the provisioned Shanghai account and bucket. External task `5f2b73369c5a267af5d7-ccl` completed with provider `volcengine_las`, model `las_pdf_parse_doubao@v1`, one billable page and the expected synthetic marker. The follow-up review found that LAS page Markdown contained an expiring signed crop URL; the adapter now replaces remote image resources and signed LAS/TOS links with stable source-preview placeholders before any knowledge chunk is persisted, with direct regression coverage for both page-level and aggregate Markdown. A forward-only migration also removes those resources from historical `document_vision` chunks and recomputes `text_sha256`; its local run reduced the synthetic canary chunk from 1,090 to 604 characters with zero remaining signature/vendor-URL markers and a valid hash.

No real-account PPTX-derived LAS request has yet verified fonts, slide count, derived-object access and billing. The pre-fix local synthetic PDF chunk is disposable canary evidence and must not be treated as production content.

Required remaining evidence: explicitly approved PPTX conversion + LAS canary, external task ID, source/derived SHA lineage, selected vs returned pages, latency, billable pages, actual cost and an operator-confirmed result for any uncertain submission.

### B2 — Real blinded corpus is missing

The Phase 11 protocol, Schema and evaluator are ready, but no real deidentified dataset has been collected. Therefore there is no measured quality gain, worst regression, correction-count reduction, correction-time reduction or category-level long-tail result.

Required evidence: at least 3 non-overlapping cases in each of 8 categories, 2 independent blinded review measurements and an independent adjudicator per case, frozen parser/model/route/prompt/converter/cost versions, and a report with no blockers.

### B3 — Product-level time baseline is missing

Offline matched-page correction time is not equivalent to total task time. The 2—4 week pre-release workflow baseline and comparable post-release cohort do not exist. UI and metrics must continue to say `time_saving_measured=false`; no “节省 X%” claim is admissible.

### B4 — Production-like migration lock rehearsal is still missing

A repeatable, fail-closed rehearsal command now exists and a populated synthetic qualification has passed: 25,000 rows in each of seven affected tables, 13 staged migrations, zero concurrent read errors, 2,347 ms slowest migration and 132 ms maximum concurrent-read latency on local MySQL 8.4.10. The report truthfully records `production_like=false`. This validates the harness and exposes table-specific timing, but does not predict locking at the real data distribution or close this gate. The provider CHECK replacement and larger Strategy/Knowledge tables still require the approved production row-count baseline on a production-like clone with lock/rollback observation.

### B5 — Local candidate exists; required CI evidence does not

The local candidate is intentionally split into Strategy/LAS commit `aa857769906d4f35dbdc2f0ff39c6f24bd8d9317` and the following Kanon integration merge with second parent `2c90c71d800b6e340b6fa4c6d01a6091dedb66a5`. Neither boundary has been pushed, so no GitHub Actions result covers the release scope. Before delivery, push only after authorization and monitor every required check to green.

The path-complete [candidate scope audit](2026-08-11-strategy-workspace-candidate-scope-audit.md) classifies every current file, records the manifest hash, explains shared-file changes, separately identifies the authorized Kanon creative paths, and finds no generated artifact or credential material. It reduces packaging ambiguity but does not replace exact-candidate CI.

The [requirement evidence ledger](2026-08-11-strategy-workspace-requirement-evidence-ledger.md) separately maps all 87 numbered requirements to implementation and verification evidence. Its document-quality review found and closed a local `DOC-04` gap: parser metadata now exposes bounded image/table signal density and blank-page evidence without treating dimensions as counts or presenting the signals as accuracy. The full local CI-parity script and the two document browser journeys pass after that change.

### B6 — Local credential readiness passed; release-environment readiness remains external

Any credential previously placed in plaintext deployment material must be rotated before release. The encrypted LAS route, shared TOS bucket, Gotenberg capacity/font pack and canary-only environment configuration must be inspected without copying secrets into `.env`, SQL, logs or the evaluation corpus.

The repository includes `cookies-check-document-vision-readiness`, a read-only, fail-closed CLI that uses the application configuration loader and Provider capability projection while exposing only fixed blocker codes. The LAS configuration script validates the canonical `COOKIES_TOS_*` inputs before accepting a hidden LAS key and invokes this checker after storing the encrypted route. The current local Shanghai environment has now passed readiness and the bounded PDF canary without printing a credential value. Release admission still requires the independently managed target environment to pass the same check and bounded canary; local readiness does not authorize copying local secrets or treating `.env` as deployment material.

### B7 — Submission-unknown workflow is implemented but not validated against a real account

The runtime persists a frozen submission intent before the network call, stops blind retries, and now provides administrator-only candidate discovery, proposal inspection, two-person confirmation, unique external-task binding, and reconciliation-specific scheduling recovery. An accepted decision resumes polling the supplied LAS task ID without resubmitting; a confirmed not-accepted decision preserves prior text and permits only a later explicit user attempt.

The remaining blocker is external proof quality. The reviewed public REST contract does not expose lookup by the Cookies intent ID, and account-side task-list fields have not been verified in the provisioned account. Operators must use the approved ticket and provider account/support evidence; ambiguous evidence remains blocked. The repository currently exposes this workflow as an operator API and runbook, not an administrator UI.

Official LAS materials reviewed on 2026-08-11 describe asynchronous submit followed by polling with the returned job/task ID; the reviewed PDF operator contract does not document a client-supplied idempotency key or lookup-by-client-request endpoint. The official `@volcengine/las-cli` package documents account-side task list/status/wait commands, but that does not establish a public REST idempotency guarantee or prove unambiguous matching. Provider support and the real-account canary must confirm the supported mechanism: [LAS operator overview](https://www.volcengine.com/docs/6492/2196029?lang=zh), [PDF parse operator](https://www.volcengine.com/docs/6492/2172371?lang=zh), [official LAS CLI](https://www.npmjs.com/package/@volcengine/las-cli).

## 4. Adversarial conclusions

The current architecture fails closed in the main high-risk paths:

- background completion cannot navigate the user or mutate stage/focus/scroll;
- research findings and AI suggestions cannot become Brief/Strategy facts without explicit adoption;
- self-confirm and formal role review are separate, so solo users do not submit unnecessary reviews;
- handoff cannot mutate the published StrategyPackage;
- Tika text survives visual/converter/provider failures;
- paid LAS submission uncertainty cannot be retried blindly;
- automatic visual fallback has no runtime path from an evaluator report and remains manually triggered;
- a report cannot pass by duplicating source pages, claiming two reviewers without two measurements, self-adjudicating, omitting review time, mixing presentation output without converter lineage, or using only one case per category;
- a passing offline report still does not authorize production enablement.

Residual risks are external evidence, real-account reconciliation matching, rollout packaging and the lack of an administrator UI rather than an identified silent local retry or data-corruption path. This statement must be revisited after the real account and corpus are available.

## 5. Admission sequence

1. Rotate/verify credentials and reproduce the locally proven encrypted-route/shared-bucket configuration in one isolated release canary environment.
2. Validate the implemented submission-intent and two-person reconciliation workflow against the canary account; confirm task-list matching fields and provider-support escalation, then retain the evidence ticket.
3. Rehearse all migrations on a production-like clone; record lock time, row counts, backup and rollback decision.
4. Retain the completed bounded PDF evidence and, with explicit paid-test approval, run one PPTX conversion + LAS canary; reconcile billing.
5. Collect the deidentified blinded corpus under Phase 11 and archive the dataset hash and report.
6. Keep manual fallback unless the report passes and the product owner separately approves automatic fallback.
7. Preserve the clean local two-commit candidate, push only with authorization, and monitor every required CI check to green.
8. Run final desktop/mobile/accessibility smoke tests against the exact candidate and execute the release checklist.
9. Release to canary, observe 15-minute and 24-hour metrics, then decide expand/hold/rollback. Never infer causal time savings from runtime latency alone.

Until every step is evidenced, the admission decision remains **HOLD**.
