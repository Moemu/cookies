# Strategy Workspace Phase 9 — hardening evidence

## Current outcome

The release candidate now has a versioned, privacy-bounded UX metrics read model and a release/rollback checklist. It is not production-released and no paid LAS call has been made.

## Metrics delivered

- Assistant command, first acknowledgment and first meaningful update events are inserted in the same MySQL transaction as their corresponding business writes.
- `/api/strategy/v1/projects/{project_id}/workspace-ux-metrics?days=1..90` reports assistant p50/p95 and missing updates, research outcomes/findings/adoption, document outcomes/latency/visual billable pages, and recovery counts.
- Event joins use organization and project scope. Assistant latency events share an immutable conversation-message resource ID.
- Attributes remain allowlisted and cannot contain prompts, document text, model reasoning or signed URLs.
- The response explicitly says `time_saving_measured=false` and `human_correction_baseline_not_collected`. Runtime speed and billable pages are measurable now; saved human time is not claimed until a before/after correction study exists.

## Adversarial findings resolved

1. An uncertain LAS submission was still retryable from the UI. Backend and frontend now require manual reconciliation for submission-unknown, invalid-checkpoint and checkpoint-persist failure states.
2. Non-contiguous visual page selections could have been expanded into unconfirmed pages. They are split into exact contiguous external tasks.
3. The old one-shot visual persistence implementation remained beside the durable task path. It was removed to eliminate a second state-transition implementation.
4. The shell skip target was not programmatically focusable. The main region now has `tabIndex=-1`, an explicit skip action and a visible focus outline.
5. Metrics infrastructure existed without real Assistant producers. Command/ack/meaningful-update events are now wired atomically.
6. “Faster” could have been presented as “time saved.” The contract separates runtime latency from the unavailable human-correction baseline.

## Rendered QA

- Browser: Codex in-app browser against `http://127.0.0.1:5173`.
- Desktop: 1280×720. All four prominent centers opened the expected stable URLs and headings. The Review center rendered without framework overlay, console warning/error or horizontal overflow.
- Mobile: 390×844. Research center navigation remained available in the collapsed rail; document/body width stayed 375 px and did not overflow the viewport.
- Interaction proof: Demand center → Research → Strategy → Review produced the matching `/briefs`, `/research`, `/strategies`, `/reviews` routes; mobile Review → Research changed both active navigation and heading.
- The browser-control host emitted an unrelated Statsig telemetry timeout; application console logs were empty.

### Final Strategy center acceptance

- The dedicated Playwright suite `e2e/strategy-workspace-rearchitecture.spec.ts` passed 7/7 against the real local Go API, Vite UI and MySQL-backed Strategy state.
- The navigation case proves that Demand, Research, Strategy and Review are one visually prominent hub while retaining independent stable URLs, headings and `aria-current` state.
- The workflow case creates a fresh isolated Project, creates and starts its Strategy workspace, enters Brief, opens Research, reloads, opens the cross-stage Assistant and Activity panel, and verifies URL/panel persistence without console or page errors.
- The Assistant can expand into an immersive workspace without losing project or stage context. Its preference survives reload and reopen, Escape exits immersive mode without closing the Assistant, and closing an expanded Assistant always restores the main stage.
- The business-flow case enters one natural Chinese requirement containing product, objective, audience and proposition. All four directly grounded facts reach the understanding lens without repeat questions; confirmation freezes all visible facts, and the Brief business-objective group renders 2/2 confirmed.
- That same isolated Project now continues through a successor Brief for the missing channel, freezes Brief v2, generates Strategy Revision 1, uses solo self-confirmation without a submit-to-self step, atomically publishes StrategyPackage v1, selects a frozen commerce-pre-roll Route, creates and completes a task plan, generates the task-level Overlay, and reloads Creative Handoff while preserving the displayed package and handoff hashes.
- A separate designated-approver journey configures the Project review policy, submits Revision 1, verifies its assignment and candidate hash, creates Revision 2 concurrently, proves the old approval returns `REVIEW_STALE` and appears as invalidated, then resubmits and approves Revision 2 into Package v1.
- Approved conversion/video strategies with a matching frozen generic pre-roll now project a stable `route_{channel}_commerce_preroll` candidate for explicit Handoff selection. Missing Project product lineage remains blocked, and explicit game, short-drama or remake signals are not guessed as commerce.
- Optional AI second-perspective reads return `204 No Content` before an analysis exists, so normal Strategy/Review/Handoff reloads no longer emit expected-resource 404 console errors. The stage rail also reports `策略：可以生成` immediately after a Brief becomes generation-ready.
- The same isolated workflow injects repeated Activity SSE 503 responses. The UI exposes reconnecting/offline state while REST reconciliation remains available, then returns to live synchronization inside the 10-second acceptance bound after the stream recovers.
- The recovery case injects repeated HTTP 503 responses, verifies a visible retry state without freezing the shell, restores the service, retries successfully and confirms that navigation remains usable.
- Responsive checks passed at 1440, 1280, 1024, 768 and 390 px with no document overflow or page-header overlap. Reduced-motion preference remains honored.
- Keyboard acceptance uses the visible skip link to focus the main region, activates all four centers with Enter, and changes the five-stage rail with keyboard focus rather than pointer-only interaction.
- The rendered mobile audit found and fixed a real header collision: project context now moves below the center description and center tabs remain horizontally reachable.
- A new empty conversation no longer requests conversation memory before it has any messages, avoiding a meaningless memory 500 while keeping all independent workspace reads parallel.
- Research status no longer has a second component-level polling loop. Activity projection is the single Research task-status path, eliminating duplicate requests and competing state writes.
- The Research vertical slice now proves non-blocking editing, live Activity-backed round reconciliation, explicit versioned adoption into Brief, and a mobile supporting-panel geometry bound. Research start also waits for the project context manifest instead of failing after an optimistic click.
- Overlapping workspace loads are latest-request-wins, so a slow bootstrap response cannot restore pre-mutation state. The E2E workspace bootstrap helper also waits for the stable conversation-start control across the expected reload transition.
- Project bootstrap no longer blocks the requested route on one detail fan-out per visible Project. It loads the active Project snapshot and workbench first, then progressively hydrates the remaining Projects in batches of two. This removes project-count-dependent blocking from first interaction without inventing a time-saved claim.
- The authenticated shell now uses route-level lazy boundaries. The main JavaScript entry fell from 1,121.12 kB minified / 316.64 kB gzip to 417.26 kB / 122.35 kB (62.8% and 61.4% smaller respectively). `Pages` is 112.90 kB, the Strategy workspace route is 206.74 kB, and the 309.00 kB specialized-page bundle remains lazy; the four-center browser journey proves that specialized bundle is not requested while using Demand, Research, Strategy or Review.
- The production build now enforces explicit bundle ceilings: 450 kB for the entry, 140 kB for `Pages`, 230 kB for the Strategy route, and 500 kB for every JavaScript chunk. A future regression is therefore a failed build rather than an advisory warning.
- Adversarial reruns after the split exposed a real review-form race: a slow policy response could overwrite a user's newly selected designated-approver mode. Project-scoped policy loading is now separate from review loading, dirty form state rejects late hydration, and policy controls stay disabled while a save is in flight.
- Requirement-by-requirement auditing exposed an `IA-07` gap: stage changes reset scroll and unmounted unsaved input. The workspace now restores per-stage scroll and session-scoped drafts across Intake, Brief, Strategy, Review and Handoff, plus the contextual Research panel. Organization/user/Project/Workspace key isolation prevents cross-account draft reuse; revision-bound Review and Handoff keys reject stale reuse. Strategy chapter drafts remain visible across chapter switches and block publish until explicitly saved or discarded, while unresolved Review input blocks approval and unsaved Handoff answers block generation. Real-browser coverage verifies the five-stage recovery boundary without keeping every stage mounted.
- The final audit closed `AST-05`: the Assistant lists the exact sources frozen into the next-turn manifest, lets the user exclude or restore an individual source, sends the bounded exclusion list separately from the Message v2 body, and clears it only after a successful turn. Rebuilding the authoritative manifest still includes the source, proving that the control cannot delete material or mutate Brief.
- The final `BRF-07` audit found that “让 AI 帮我补充” previously only opened the panel. The Assistant now exposes three context-derived entry points, with the highest-priority Brief blocker and current stage in the request. The governed conversation prompt requires 2—3 differentiated candidates with rationale, evidence and explicit assumptions, rejects a generic template wall, and forbids writing a candidate before user confirmation.
- Deep Research reports now have a real Markdown download action, and HTML/HTM documents are routed through the bounded Tika text path with MIME validation and upload-surface support.
- Shared-schema MySQL tests previously competed for the global durable queue. JobRuntime integration fixtures now use a unique old schedule time, preserving production global-claim behavior while preventing tests from stealing newly queued Knowledge/Strategy jobs in parallel package runs.
- The migration gate now has a fail-closed rehearsal command that accepts only an empty `cookies_rehearsal_*` schema on loopback MySQL, requires explicit multi-statement support, records per-migration duration and concurrent-read latency, and never deletes the schema. A 25,000-row synthetic qualification on seven affected tables passed all 13 staged migrations with a 2,347 ms maximum migration and 132 ms maximum concurrent-read latency; it remains labelled `production_like=false`.
- One-bucket adversarial review closed three scope gaps: converted presentation PDFs now stay under the document's `assets/{org}/{project}/knowledge/{document}/derived/...` prefix; Knowledge and Provider output reads/deletes fail closed when persisted bucket/key lineage leaves the authorized scope; and LAS input/output paths now independently enforce explicit organization, project and document prefixes.
- Activity recovery review found that old queued work was being presented as a stale Worker execution. Stalled detection now applies only to running work with missing heartbeat, while reconnect cursors are isolated by organization, user, Project and Workspace.
- A real local schema-drift failure showed why migration IDs alone were insufficient: an earlier version of the document-vision task migration had been recorded before durable intent columns were added. A forward-only compatibility migration now repairs that shape, and `platform_schema_migrations` records SHA-256 checksums so any future edit to an applied migration fails closed. A real-MySQL integration test mutates an applied probe migration and verifies the rejection.
- Shared-schema integration workers now support optional organization/Project/job claim scopes while the production default remains the global queue. Test dispatches and jobs are transactionally scheduled against an injected future clock, preventing a concurrently running local API worker from stealing fixtures; repeated Strategy vertical slices and the five-package MySQL gate pass deterministically.
- Lease recovery now enforces the same maximum-attempt boundary as ordinary deferral: abandoned work with remaining attempts returns to the queue, while a final abandoned attempt becomes an explicit non-retryable `JOB_ATTEMPT_LIMIT_EXCEEDED` failure instead of looping forever. A real-MySQL test runs Worker A and Worker B as separate OS processes, preserves the JSON checkpoint across the first process exit, and verifies the replacement process completes attempt 2.
- The browser suite now emits repeatable local performance evidence with conservative regression ceilings. The final full run measured 832 ms LCP on the four-center route, a 256 ms maximum center interaction, 1,712 ms until 50 dense messages were ready, and a 193 ms maximum transition among Brief, 30 findings, 20 documents and 10 Activity cards. These are local deterministic acceptance measurements, not production percentiles or human time saved.

## Automated verification

- `go test ./...` passed before the metrics increment; targeted Strategy/API/Provider/Knowledge tests passed after it.
- `go vet ./...` passed before the metrics increment.
- Frontend tests passed at 175/175 before the new accessibility test; the targeted contract suite passed after the metrics contract was added.
- Production frontend build passed before the metrics increment; the existing >500 KiB main-chunk warning remains.
- Real MySQL Knowledge fallback, encrypted LAS route and workspace UX metrics tests passed.
- PowerShell AST parsing passed for `scripts/configure-las-document-vision.ps1`.
- Final post-UI gates passed: `npm test` 198/198, `go test ./...`, `go vet ./...`, `npm run contract:check`, `npm run build`, `git diff --check`, the real-MySQL process-restart probe, and the 11/11 Strategy browser suite.
- The final Strategy browser rerun including prominent centers, formal review, Deep Research report download, Assistant next-turn source exclusion, slow-policy race protection, per-stage scroll, five-stage unsaved-draft recovery and failure recovery passed 7/7 in 62.7 seconds.
- The final post-hardening browser rerun passed the same 7/7 suite in 57.7 seconds after Activity scope, migration, queue-isolation and one-bucket storage changes.
- The production build emits a 417.26 kB minified entry and a 210.45 kB Strategy route, passing the enforced entry/route/per-chunk budgets without Vite's former >500 kB warning. These measurements are bundle bytes, not a claim of saved human time.
- The latest post-audit production build keeps the 417.26 kB entry and emits a 212.72 kB Strategy route, below the enforced 230 kB route ceiling.

## Remaining gates

- The complete local gate set is green, including the broad Strategy, Knowledge, Provider and JobRuntime MySQL slices. No live image credential is required by this deterministic acceptance boundary.
- A safe populated migration rehearsal command and a 25,000-row-per-table synthetic report now exist. The synthetic run passed all 13 staged migrations with zero concurrent read errors, a 2,347 ms slowest migration and 132 ms maximum concurrent-read latency, but it is explicitly `production_like=false`. Repeat it with approved real row-count baselines on a production-like clone and isolate the Provider CHECK replacement lock before release.
- Run the labelled document corpus and collect independent correction-time evidence before claiming saved time or enabling automatic visual fallback.
- Execute the already implemented PPT/PPTX conversion and LAS route against a real account only after explicit paid-call approval.
- Rotate and verify external credentials. The separately supplied plaintext deployment runbook targets another repository and is not a Cookies deployment source.
- Freeze an immutable candidate and obtain required GitHub Actions evidence only after commit/push authorization.
