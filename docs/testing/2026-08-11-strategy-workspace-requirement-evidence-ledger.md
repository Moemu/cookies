# Strategy Workspace requirement evidence ledger

Date: 2026-08-11

Purpose: prove or reject each numbered requirement in the approved implementation plan against the current working tree. This ledger does not turn a mutable working tree into a release candidate and does not authorize a commit, push, paid provider call, production migration, or deployment.

Evidence labels:

- **Local pass** — implementation plus deterministic test or rendered/runtime evidence exists.
- **Local pass; rollout gated** — the safe/default-off implementation exists, but external evidence is required before enablement.
- **Code pass; environment No-Go** — the fail-closed boundary and tests exist, but the current environment is not configured.

## Information architecture

| ID | Result | Authoritative evidence |
| --- | --- | --- |
| `IA-01` | Local pass | `workspaceRoute.ts`, `StageRail.tsx`, `strategy-workspace-route-v2.test.ts`. |
| `IA-02` | Local pass | Host router owns the stage; `strategy-app-route-v2.test.ts` rejects a second workspace stage source. |
| `IA-03` | Local pass | `useActivityStream.ts` changes resources/badges only; navigation and dense-activity Playwright cases prove no stage/focus/scroll mutation. |
| `IA-04` | Local pass | `StrategyWorkspaceShell.tsx` and `WorkspaceTopbar.tsx` expose research/materials/activity/history as contextual panels. |
| `IA-05` | Local pass | Strategy v3 optional experiment section and existing Insight experiment-center link are covered by Strategy UI/contract tests. |
| `IA-06` | Local pass | `workspaceRoute.ts` deterministic legacy mapping plus routing tests and browser redirect case. |
| `IA-07` | Local pass | `workspaceSessionState.ts` and session-state tests preserve per-stage scroll and revision-bound drafts. |
| `IA-08` | Local pass | Keyboard stage rail, skip link, 1440/1280/1024/768/390 px and reduced-motion Playwright acceptance. |

## Project Assistant and memory

| ID | Result | Authoritative evidence |
| --- | --- | --- |
| `AST-01` | Local pass | `ProjectAssistantDock.tsx` is mounted through the Strategy workspace shell seam only. |
| `AST-02` | Local pass | Dock collapse/resize/immersive preferences and Escape recovery are covered by Assistant and browser tests. |
| `AST-03` | Local pass | The workspace conversation remains mounted across all five URL stages; browser journey verifies continuity. |
| `AST-04` | Local pass | `context_manifest.go`, `useProjectContextManifest.ts`, v1 schema/fixture and context-manifest tests expose frozen Project/workspace/Brief/Strategy/source/memory refs. |
| `AST-05` | Local pass | Next-turn exclusion is a bounded request header; manifest rebuild still contains the source. Covered by source-exclusion Go/TS/E2E tests. |
| `AST-06` | Local pass | `assistant_proposal.go` creates proposals only; expected version/hash and explicit apply are verified in MySQL and TS tests. |
| `AST-07` | Local pass | `memory_feedback.go` prioritizes artifact manifest, recent window and deterministic/model summary fields; memory tests verify precedence. |
| `AST-08` | Local pass | Hash/stale/conflict checks discard or rebuild summary without blocking conversation; memory conflict/failure tests. |
| `AST-09` | Local pass | Activity-owned tasks survive dock close; unread/expanded state persists with owner/workspace isolation. |
| `AST-10` | Local pass | Contracts reject reasoning fields; UI exposes execution summaries/evidence only. |

## Activity and reliability

| ID | Result | Authoritative evidence |
| --- | --- | --- |
| `RUN-01` | Local pass | Agent/Job/Research/Document commands return durable IDs before execution; API and MySQL tests verify persistence. |
| `RUN-02` | Local pass | `activity.go`, TaskActivity schemas and Activity UI cover kind/status/phase/round/progress/summary/conclusions/actions. |
| `RUN-03` | Local pass | Projection maps queued/running/waiting/partial/succeeded/failed/cancelled/stalled and schema invalid-state tests reject leakage. |
| `RUN-04` | Local pass | JobRuntime renews every 15 seconds in composition; Research/Document handlers persist heartbeat, phase and checkpoint progress. |
| `RUN-05` | Local pass | Retry commands enqueue a new attempt against the same resource and retain checkpoints; Knowledge MySQL recovery tests. |
| `RUN-06` | Local pass | Cancel delegates to existing Job/Agent/Research/Document authority and preserves non-cancellable upstream semantics. |
| `RUN-07` | Local pass | SSE reducers reject duplicate/out-of-order cursors and selectively reconcile terminal resources. |
| `RUN-08` | Local pass | Bounded reconnect performs REST reconciliation before resubscription; 503/stream-recovery Playwright case. |
| `RUN-09` | Local pass | 60-second running-heartbeat threshold, stalled/diagnostic copy and no independent infinite poller. |
| `RUN-10` | Local pass | Revision-bound session drafts, domain checkpoints and partial artifacts survive failures/reloads. |

## Deep research

| ID | Result | Authoritative evidence |
| --- | --- | --- |
| `RES-01` | Local pass | Conversation web search remains a single grounded request and excludes deep-research runs. |
| `RES-02` | Local pass | `research_orchestrator.go` implements the persisted plan/search/read/extract/cross-check/synthesize/audit loop. |
| `RES-03` | Local pass | Run creation freezes manifest/reference versions and input hash; service/MySQL tests. |
| `RES-04` | Local pass | Iteration rows persist objective, action summary, source count, candidate findings and gaps. |
| `RES-05` | Local pass | Finding schema/model carries claim, support/conflict sources, time scope, confidence, status, target and implication. |
| `RES-06` | Local pass | tentative/verified/conflicting/invalid transitions are schema- and verifier-tested. |
| `RES-07` | Local pass | Canonical-domain independence and single-source status are enforced by source-verifier tests. |
| `RES-08` | Local pass | Server stopping covers coverage, cross-check, diminishing returns, round/time/token/source budgets. |
| `RES-09` | Local pass | Defaults are six rounds/900 seconds/72,000 tokens; client cannot raise server limits. |
| `RES-10` | Local pass | Activity reconciliation and Research drawer show persisted rounds/findings while the run is active. |
| `RES-11` | Local pass | Terminal run creates a Markdown report artifact and versioned adoption proposals; download E2E. |
| `RES-12` | Local pass | Base version/hash changes make proposals stale; remap creates a successor proposal. |
| `RES-13` | Local pass | Research UI shows operations, source/confidence and accept/edit/ignore; apply is expected-version/idempotency bound. |
| `RES-14` | Local pass | Verified findings survive external failure and terminal status becomes `partially_completed`. |

## Brief

| ID | Result | Authoritative evidence |
| --- | --- | --- |
| `BRF-01` | Local pass | Brief editor renders decision groups rather than field cards; UI source test. |
| `BRF-02` | Local pass | Objective, product/evidence, audience/context, channel/conversion, constraints and risk/unknown groups are defined and rendered. |
| `BRF-03` | Local pass | Deterministic/model patch logic preserves short valid user text; additions remain explicit proposals with rationale. |
| `BRF-04` | Local pass | Field-state contract/UI distinguish fact, inference, suggestion and question. |
| `BRF-05` | Local pass | Proposal accept/edit/ignore commands are versioned, idempotent and audited. |
| `BRF-06` | Local pass | Group confirmation is allowed for low-risk fields; high-risk facts remain individually explicit. |
| `BRF-07` | Local pass | Assistant offers three current-context candidate requests; test rejects a generic template wall. |
| `BRF-08` | Local pass | Readiness summary displays blockers/warnings/assumptions and frozen version before confirmation. |

## Strategy

| ID | Result | Authoritative evidence |
| --- | --- | --- |
| `STG-01` | Local pass | v3 schema, v3-only writer and successor migration add structured `creative_strategy`. |
| `STG-02` | Local pass | Schema/Go model require objective, message hierarchy, territories/proof/adaptations, tone, mandatories and avoidances. |
| `STG-03` | Local pass | Channel strategy remains a distinct section; Handoff has no Strategy section-patch command. |
| `STG-04` | Local pass | Experiment/research/advanced sections use progressive disclosure in the Strategy editor. |
| `STG-05` | Local pass | Revision request carries impact scope and produces section diff before apply. |
| `STG-06` | Local pass | Research proposals target concrete Strategy section/field paths and cannot apply as an unlocated reference. |
| `STG-07` | Local pass | Diff model/UI separate evidence, assumption and compliance change sets. |
| `STG-08` | Local pass | Second perspective uses fixed `cookies.text.deep_review`, isolated prompt/context and suggestion-only storage. |

## Review

| ID | Result | Authoritative evidence |
| --- | --- | --- |
| `REV-01` | Local pass | `review_confirm.go` atomically confirms/publishes solo work; browser UI has no submit-to-self step. |
| `REV-02` | Local pass | Designated/leader policies alone render reviewer assignment/progress and formal submit. |
| `REV-03` | Local pass | Deep-review status is independent and optional; empty/failure response does not gate approval. |
| `REV-04` | Local pass | Review binds candidate revision/hash; concurrent successor invalidates stale approval in MySQL/E2E. |
| `REV-05` | Local pass | Review summary renders one-line decision, audience, channel, evidence, risks and changes. |
| `REV-06` | Local pass | Risk-aware grouped confirmation plus explicit claim/compliance blockers are tested. |

## Creative handoff

| ID | Result | Authoritative evidence |
| --- | --- | --- |
| `HOF-01` | Local pass | User-facing navigation and page copy use “创意交接/创意任务拆解”. |
| `HOF-02` | Local pass | Handoff creation requires immutable package ID/hash, never a mutable draft ref. |
| `HOF-03` | Local pass | Handoff projection reads creative strategy, channel strategy and Brief lineage from the frozen package. |
| `HOF-04` | Local pass | Route/task changes create plan/overlay revisions only. |
| `HOF-05` | Local pass | Backend/API/TS boundary tests prove no `patchStrategySection('channel_strategy', ...)` path. |
| `HOF-06` | Local pass | Existing `creative-task-strategy/v2` and Creative intake fixtures/contracts pass. |
| `HOF-07` | Local pass | Candidate-scope audit contains no downstream image/video/audio production module change. |

## Document parsing and visual fallback

| ID | Result | Authoritative evidence |
| --- | --- | --- |
| `DOC-01` | Local pass | Upload response/UI expose accepted document identity, filename/MIME/size, run/activity and selected parser path. |
| `DOC-02` | Local pass | Native text/Markdown route and bounded Tika HTML/Office route have parser/router tests. |
| `DOC-03` | Local pass; rollout gated | PDF/PPT text-first flow and selected-page vision task exist; automatic whole-document vision is absent. |
| `DOC-04` | Local pass | `document_quality.go` now measures text density, mojibake/control rates, blank-page metadata, reading-order signal, image/table signal density and locator coverage; unit/UI tests prevent dimension/count confusion and accuracy claims. |
| `DOC-05` | Local pass; rollout gated | Low-quality reason and shadow recommendation are visible; manual bounded fallback requires confirmation. |
| `DOC-06` | Local pass; rollout gated | Durable document-level progress/quality shipped first; page vision is isolated and default-off. |
| `DOC-07` | Local pass | Original download, extracted preview, chunk/page locators and quality warning are rendered. |
| `DOC-08` | Local pass | Milestone/page progress comes from checkpoints; unknown totals show an honest indeterminate explanation. |
| `DOC-09` | Local pass | Text/preview survive visual failure; retry targets failed parse/vision/conversion stage and reconciliation prevents blind paid retries. |

## Metrics, security and storage

| ID | Result | Authoritative evidence |
| --- | --- | --- |
| `OPS-01` | Local pass | Product events and project-scoped UX metric read model persist/query MySQL; no administrator UI was introduced. |
| `OPS-02` | Local pass | Event allowlist/schema reject prompt, document text, nested arbitrary payload and model reasoning. |
| `OPS-03` | Local pass | Strategy skill attempts, research iterations, vision tasks and Job/Agent terminal records persist alias/route/model/prompt/usage/latency/terminal lineage as applicable. |
| `OPS-04` | Local pass; production rollout gated | One-bucket prefix builders and cross-project tests exist; the local Shanghai profile uses the private `cookies-storage-sh-a8f3k2` bucket with scoped quarantine/assets/provider-output prefixes. Target-environment IAM and credential rollout remain release gates. |
| `OPS-05` | Local pass | Quarantine is unreadable as a business asset; scan/validation copy-promotes into an authorized assets prefix with asymmetric-failure tests. |
| `OPS-06` | Local pass | Provider output handles are server-only, project-scoped and normalized; frontend receives no durable provider URL. |
| `OPS-07` | Local pass | CORS/lifecycle are explicitly recorded as post-launch operations debt in the release runbook. |

## SLO and Definition-of-Done evidence

| Boundary | Current evidence |
| --- | --- |
| Ack/meaningful update | Transactional product events and local browser timing; production p95 is not yet claimed. |
| Heartbeat/stalled | Worker composition uses 15 seconds; Activity marks only running work stalled after 60 seconds. |
| SSE recovery | Reconnect delay caps at five seconds plus bounded jitter and performs REST reconciliation; live injected recovery passes. |
| Autosave | Visible serialized server autosave fires after 750 ms; high-risk confirmation remains separate. |
| Research non-blocking | Browser journey edits Brief while deep research continues through Activity. |
| Navigation stability | Background completion, reconnect and dense-task cases preserve URL stage, stage scroll and focus. |
| Responsive/accessibility/performance | 1440/1280/1024/768/390 px, keyboard, skip link, reduced motion, dense data and bundle budgets pass locally. |
| Migration/history | Dry-run/apply tooling, backup manifest, successor lineage and historical package hash checks exist; production-like rehearsal remains external. |
| Reverse review | Phase reports record adversarial findings and closures; current scope/security/requirement audits re-check the merged tree. |

## Evidence that is deliberately not claimed

The following remain outside local proof and prevent production admission:

1. No required GitHub Actions result exists until the intentionally split candidate is pushed; this ledger does not treat a local commit as CI evidence.
2. Local Shanghai one-bucket TOS, encrypted LAS route and credential readiness are configured, but target-environment credential/IAM rollout remains unverified.
3. One authorized one-page PDF canary proved the local account route, expected marker and billable-page reporting. A real PPTX-derived conversion + LAS canary remains pending.
4. No deidentified blinded corpus proves hybrid quality/correction-time benefit; automatic fallback remains off.
5. No two-to-four-week product baseline proves human time savings; metrics must continue to report `time_saving_measured=false`.
6. No approved production-like clone/row-count distribution proves migration lock behavior and rollback timing.
7. The Windows host cannot supply the exact Linux/CGO `go test -race ./...` evidence required from candidate CI.

These are external evidence or authorization gates, not permission to weaken the implementation or substitute a mock result.
