# Strategy Workspace requirement-to-evidence completion audit

Date: 2026-08-11

Decision: the scoped implementation is complete and locally verified; production admission remains **HOLD** on external evidence and release authorization.

This audit maps every requirement in the approved plan to an implemented boundary. “Complete” means code, deterministic tests and local runtime evidence exist. It does not mean a paid provider call, production migration, commit, push or deployment was authorized.

| Requirement IDs | Local status | Authoritative implementation and proof |
| --- | --- | --- |
| `IA-01`–`IA-08` | Complete | URL-authoritative five-stage router, deterministic legacy redirects, contextual panels, stage/session/scroll recovery and responsive keyboard rail; `workspaceRoute.ts`, `workspaceSessionState.ts`, routing/session tests and 11/11 browser suite. |
| `AST-01`–`AST-10` | Complete | Project-scoped dock, resize/immersive/unread memory, frozen manifest, single-turn source exclusion, versioned memory compaction/rebuild, explicit proposal adoption and execution summaries without chain-of-thought; the entire shell and local preferences remount and persist at the organization/user/Project boundary. |
| `RUN-01`–`RUN-10` | Complete | Durable JobRuntime IDs, unified Activity snapshots/SSE, phase/round/heartbeat, bounded reconnect/reconciliation, stage retry/cancel/checkpoints and preserved artifacts. A real-MySQL probe runs Worker A and Worker B as separate OS processes, proves checkpoint recovery after A exits, and fails final abandoned attempts instead of infinitely re-queuing them. |
| `RES-01`–`RES-14` | Complete | One-call quick search plus non-blocking bounded deep-research loop, immutable input snapshots, persisted rounds/findings/source verification, server stopping, partial completion, downloadable report and version/hash-bound adoption diff; Phase 4 tests and real browser flow. |
| `BRF-01`–`BRF-08` | Complete | Decision-grouped Brief, semantic fact/inference/suggestion/question states, 750 ms server autosave with serialized optimistic writes and visible recovery state, grouped low-risk confirmation, explicit high-risk confirmation, accept/edit/ignore proposals, contextual 2—3 candidate requests and pre-freeze readiness summary; Brief/Assistant tests and rendered QA. Autosave persists `unconfirmed` facts and therefore cannot silently approve a high-risk decision. |
| `STG-01`–`STG-08` | Complete | StrategyDraft v3 with structured creative strategy, distinct channel strategy, optional experiment/research sections, impact-scoped section revisions, evidence/assumption/compliance diffs and optional fixed-alias second perspective; v3 schemas, Go/TS tests and E2E publish paths. |
| `REV-01`–`REV-06` | Complete | Atomic solo confirm-and-publish, designated role review, independent second perspective, revision/hash binding, stale invalidation and risk-aware decision summary; formal-review MySQL/E2E cases. |
| `HOF-01`–`HOF-07` | Complete | Read-only Creative Handoff derived from immutable StrategyPackage, explicit route and task overlay revisions, no Strategy writeback and unchanged downstream production state machines; frozen contracts and handoff tests/E2E. |
| `DOC-01`–`DOC-09` | Complete within the approved manual-fallback scope | Native text plus HTML/Office Tika routing, PDF/PPT page-quality signals, truthful milestones/page counts, bounded preview/locators/original access, partial preservation, failed-path retry and manual visual candidate/reconciliation. Real browser journeys now upload native text and a low-quality PDF through the local API, verify preview/lineage/quality, and keep visual fallback explicit. Automatic fallback remains deliberately off pending corpus evidence. |
| `OPS-01`–`OPS-07` | Complete for local scope | Allowlisted MySQL events/provider lineage, one TOS bucket with quarantine/assets/provider-output prefixes, scan promotion and server-only provider output. CORS/lifecycle remain explicitly recorded post-launch debt per `OPS-07`. |

## Final adversarial closures

- A background completion cannot change route, stage, focus or scroll.
- Switching organization, user or Project cannot reuse or overwrite another session's Assistant width, expanded state or unread cursor.
- AI suggestions and research findings cannot write business objects without explicit user action.
- A source exclusion affects one Assistant turn only and cannot mutate the authoritative manifest.
- Solo confirmation cannot accidentally create a self-review; formal decisions are revision/hash bound.
- Handoff changes cannot patch Strategy.
- Low-quality document text survives visual/converter/provider failure; paid submission uncertainty cannot blind-retry.
- Seed 429/503/timeouts, Tika 503/empty/oversized/timeouts, MySQL terminal-write deadlocks and asymmetric TOS/DB cleanup failures now have direct fail-closed tests.
- Duplicate, invalid or out-of-order conversation SSE cursors cannot regress the stored owner-scoped cursor or replay stale UI effects.
- Core workspace schemas now have an explicit valid/invalid matrix plus applicable stalled/stale/partial cases; partial Strategy v3 and lifecycle-state leakage are rejected.
- Integration tests no longer steal each other's global queue jobs when packages share one MySQL schema.

## Final local evidence

- `npm test`: 205/205 passed.
- `npm run build`: passed; entry 423.79 kB, `Pages` 112.92 kB, Strategy route 214.07 kB, all enforced budgets green.
- `go test ./...`, `go vet ./...`, `npm run contract:check`, and `git diff --check`: passed.
- Real MySQL: JobRuntime, Provider, Knowledge and Strategy packages passed together.
- Real browser: the exact post-migration combined platform suite passed 20/20 in 153.4 seconds, covering upstream Delivery, the Strategy foundation and the complete Strategy workspace against one local API/Tika/MySQL lifecycle without manual LAS/TOS environment overrides. It measured 376 ms LCP, 84.3 ms maximum center interaction, 2,754.0 ms dense-message readiness and 407.5 ms maximum dense-panel transition. The density case covers 50 long Chinese messages, 30 findings, 20 documents and 10 live activities without horizontal layout overflow. No relevant console warnings/errors were found; 1440/1280/1024/768/390 px automated coverage remains green.
- Real LAS PDF: the authorized one-page Shanghai canary completed with model `las_pdf_parse_doubao@v1`, one billable page and the expected synthetic marker. A post-canary review found and closed persistence of expiring LAS crop URLs; page and aggregate Markdown now remove remote image resources and signed temporary links before knowledge ingestion, while a forward migration sanitizes historical visual chunks and recomputes their text hashes.
- The post-audit document-quality follow-up passed both real-browser document journeys (plain-text parse/preview/Brief lineage and low-quality PDF/manual fallback) in 27.3 seconds. The 87-item [requirement evidence ledger](2026-08-11-strategy-workspace-requirement-evidence-ledger.md) maps every numbered plan requirement exactly once and separates local proof from rollout/environment gates.

## External admission gates

The following are not local implementation gaps and remain release blockers: a real paid PPTX conversion + LAS canary, a deidentified 24+ case blinded corpus with independent correction-time measurements, a production-like migration lock rehearsal using the now-qualified populated rehearsal command and approved real row-count baselines, target-environment credential/canary readiness, an immutable candidate SHA and required GitHub Actions checks, and a 2–4 week product-time baseline. The bounded real PDF canary and 25,000-row-per-table synthetic rehearsal passed, but the rehearsal is explicitly `production_like=false`. No causal “saved time” claim or automatic visual fallback is allowed before those gates pass.
