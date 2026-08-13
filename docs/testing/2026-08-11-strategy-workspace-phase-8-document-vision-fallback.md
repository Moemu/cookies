# Strategy Workspace Phase 8 — Document vision fallback foundation

## Current outcome

Phase 8 now has a safe, manually triggered page-level fallback path for low-quality PDF/PPT text parses. The original Tika text and chunks remain available throughout the run and after any visual-provider failure. Automatic fallback is still disabled.

The production composition root now includes a real Volcengine LAS PDF operator adapter, but it is default-off. Unless `COOKIES_DOCUMENT_VISION_ENABLED=true`, an encrypted fixed route and the shared TOS bucket are all configured, the capability endpoint honestly returns `DOCUMENT_VISION_PROVIDER_DISABLED`. No account-level paid smoke test has been run, and automatic fallback remains disabled.

## Delivered architecture

- Added the cookies-owned `DocumentVisionParser` normalization boundary. Vendor request bodies, signed URLs and raw responses cannot enter Strategy contracts.
- Added an explicit capability inspection endpoint and a separate manual command endpoint. Inspection never invokes a model or creates cost.
- Limited eligibility to low-quality, partial PDF/PPT text parses. Unknown or longer-than-24-page documents require an explicit page selection; one request is capped at 24 pages.
- Added persisted document-level visual status and a page table created only for selected fallback pages.
- Added a resumable polling Job with a durable visual-attempt identity, per-range external-task checkpoints and real 72–92 progress milestones.
- Split a non-contiguous user selection such as `1, 3-5, 8` into three exact contiguous LAS tasks. The adapter never expands the range to `1-8`, so unconfirmed pages are not submitted or billed.
- Persisted `prepared → submitting → submitted/running → completed/failed` before and after each external transition. A crash while submission outcome is uncertain becomes `DOCUMENT_VISION_SUBMISSION_UNKNOWN`; it is never automatically resubmitted because that could duplicate cost.
- Persisted a deterministic submission intent and frozen route/input/output lineage before the paid call. Submission-unknown tasks now have an administrator-only, two-person proposal/confirmation workflow; accepted tasks resume polling without resubmit, while confirmed not-accepted tasks preserve text and permit only a later explicit user attempt.
- Preserved existing text/chunks on retry, cancellation, missing visual pages, invalid provider output and terminal failure.
- Added hybrid merge chunks with page locators and a second routing-quality check. A document becomes `ready` only when all pages are covered and the routing signal is no longer low; otherwise it remains usable as `partial`.
- Added deterministic enqueue idempotency tied to the persisted document update and a reconciler for the crash window between committing `queued` state and creating the Job.
- Added terminal Job reconciliation for failed/cancelled visual jobs so the UI cannot remain indefinitely in `queued` or `running`.
- Projected parse and visual-fallback events into bounded Strategy product events without prompts, document text or hidden reasoning.

## Provider-output safety

- Per-page Markdown: maximum 2 MiB.
- Combined response Markdown: maximum 8 MiB.
- Usage metadata and locator JSON: maximum 64 KiB each.
- Provider/model/route lineage is bounded; the returned route revision must equal the inspected route revision.
- Persisted locators allow only `page_number`, a bounded `section`, and bounded `bounding_boxes` data.
- Locator keys containing URL, token, secret, credential, authorization, bucket, object key or raw-response data are rejected.
- Duplicate pages, pages outside the explicit selection, empty pages, invalid JSON usage and route changes fail closed.

## UX behavior

- The materials drawer checks visual capability only for a selected low-quality partial document.
- Users see why fallback is recommended, whether the fixed route is available, and that no silent model substitution will occur.
- Long/unknown documents expose a compact page selector supporting values such as `1, 3-5, 8`.
- The user must confirm before a visual run starts. The task is non-blocking and remains visible in the shared Activity stream.
- Failure and partial-result states explicitly say that the original text was retained and offer a bounded retry when the capability remains available.
- The capability check itself explains that it does not call a model or incur model cost.

## Evaluation and rollout gate

`cmd/cookies-eval-document-vision` now reads the strict, versioned `document-vision-evaluation-dataset/v1` object. It rejects the former bare JSON array and incomplete evidence. The dataset binds every pair to source SHA/page lineage, frozen parser/model/route/prompt/converter versions, a cost-policy version, per-reviewer blinded measurements and adjudication. The report records case/review coverage, baseline/hybrid quality, worst regression, correction-count and measured correction-time reduction, latency, billable pages and cost. The required labelled categories are:

- text PDF;
- scanned PDF;
- two-column layout;
- table-dense layout;
- Chinese PPT;
- broken font map;
- header/footer noise;
- image-led document.

The initial automatic-enable gate requires at least three cases in each category (24 total), both review orders in every category, at least two independent blinded reviewers and adjudication per case, mean quality gain of at least 0.12, worst-case regression no greater than 0.08, at least 30% fewer human corrections and at least 30% less measured review/correction time. These are conservative rollout criteria, not measured results. `auto_enable_allowed` is evidence for a later canary decision, not a runtime switch. No checked-in corpus or runtime configuration can automatically enable fallback.

The collection, timing, redaction and no-causal-overclaim rules are frozen in [Phase 11 — document vision evaluation protocol](./2026-08-11-strategy-workspace-phase-11-document-vision-evaluation-protocol.md). Real source files, reviewer mappings and labelled outputs stay outside Git.

The evaluator bounds case count and labelled-output size. Short snippets use normalized rune edit similarity; long documents use linear bigram similarity to avoid quadratic freezes.

## Official capability research

- The reviewed [Volcengine LAS PDF parsing operator documentation](https://docs.volcengine.com/docs/6492/2172371?lang=zh) defines operator `las_pdf_parse_doubao`, version `v1`, asynchronous `/submit` and `/poll` calls, page ranges, detail Markdown, bounding boxes, output TOS paths and billable-page reporting.
- The real adapter accepts only `application/pdf`, uses one same-account/same-region TOS bucket for source and provider output, and sends the credential only in the Bearer header.
- Route, operator version, parse mode, response limit and poll interval are frozen into an encrypted-provider route revision and copied into the durable task checkpoint. Plaintext credentials, signed URLs and raw vendor responses are not persisted.
- PPTX remains a separate conversion concern. LAS direct-PPT behavior is not assumed. When the reviewed Gotenberg/LibreOffice converter is disabled, capability inspection returns `DOCUMENT_VISION_CONVERTER_DISABLED`; when enabled, the worker first creates a traceable PDF derivative in the same TOS bucket and then submits only that PDF to LAS.

The adapter can be configured locally with `scripts/configure-las-document-vision.ps1`. The script validates the shared bucket and master key, encrypts the API key from hidden input, writes the fixed LAS route and enables the feature flags. It does not run a paid document parse. The feature remains default-off until configuration and an explicit account smoke test.

## Adversarial review findings resolved

1. **Oversized or malicious model output:** added content, metadata and locator bounds plus a strict locator allowlist.
2. **Route drift or silent fallback:** the provider route must match the inspected fixed route; mismatch is terminal and explicit.
3. **Concurrent manual starts:** document state is claimed before page rows are written, and a losing request releases its transaction before re-reading state.
4. **Crash after persisted queue state:** deterministic Job idempotency and orphan reconciliation recreate the missing Job.
5. **Worker failure/cancellation:** terminal Job reconciliation closes the document state while preserving prior chunks.
6. **Repeated polling and competing state:** no visual-specific frontend poller was added; the existing Activity stream remains the shared authority.
7. **Evaluation freeze on long content:** long-document scoring is linear rather than quadratic.
8. **Mobile navigation regression:** browser QA found the strategy-task master/detail grid overflowing at 390 px; it now collapses to one column and no longer causes page-level horizontal scrolling.
9. **Non-contiguous selection overbilling:** exact selections are split into contiguous upstream tasks; no min/max expansion is used.
10. **Crash between submit and checkpoint:** the database records `submitting` first. An uncertain outcome is surfaced for manual reconciliation and is not silently retried.
11. **Unbounded vendor response:** the LAS HTTP body is capped by the immutable route response limit before JSON decoding.
12. **Unsupported presentation presented as available:** MIME support is part of capability inspection. PPT/PPTX is available only when the independently configured converter and fixed PDF route both pass inspection; otherwise the UI gives the exact disabled reason.

## Verification evidence

- Targeted Knowledge, Strategy, HTTP and command Go tests passed.
- Real MySQL `TestKnowledgeCenterMySQLProjection` passed, including page persistence, hybrid chunks, orphaned-Job recovery, terminal failure reconciliation and prior-text preservation.
- A fresh-schema MySQL fault-injection rehearsal passed for pre-call intent persistence, safe candidate discovery, distinct-operator confirmation, unique external-task binding, reconciliation-specific scheduling recovery, and the confirmed-not-accepted retry boundary; its temporary database was removed and verified absent.
- Real MySQL provider-route resolution passed for encrypted `las_operator` connections and `document.vision.parse` route revisions.
- LAS adapter fixture tests passed for submit/poll normalization, exact page ranges, TOS scoping, credential non-persistence, failed/timeout tasks and oversized responses.
- The LAS configuration script passes PowerShell AST syntax validation. It was not executed because it prompts for a real key and mutates local configuration.
- Full `go test ./...` passed.
- Full `go vet ./...` passed.
- Frontend tests passed: 179/179.
- Production frontend build passed; the existing large main-chunk warning remains non-blocking.
- Browser QA passed at the default desktop viewport for the prominent four-center navigation and materials drawer.
- Browser QA at 390×844 found and then verified the task-center overflow fix: document/body width is 375 px with no horizontal overflow; all four center buttons remain present as the collapsed icon rail.
- Browser console reported no application errors during the final check.

## Rollout follow-up and remaining blockers

- Completed follow-up on 2026-08-11: one explicitly approved one-page PDF canary ran through the Shanghai same-region TOS/LAS route, returned the expected synthetic marker and reported one billable page. A subsequent persistence review removed signed temporary crop URLs from stored Markdown and added historical sanitization.
- Run a real-account PPTX conversion plus LAS smoke test in the canary environment. The code path is implemented but remains default-off until the account route, converter capacity, and fonts have been verified.
- Collect and adjudicate the real deidentified corpus under the Phase 11 blind/crossover protocol, run the evaluator, and record real quality, correction time, end-to-end latency and billable-page cost. The protocol and evaluator are implemented; no real corpus has been collected, so current thresholds remain gates rather than measured claims.
- Validate the implemented administrator reconciliation workflow in the real canary account. The reviewed public REST contract still does not document client idempotency or reverse lookup; the official LAS CLI documents account-side task list/status commands, but their matching fields and visibility must be verified before operational use. See [document vision submission reconciliation](../runbooks/document-vision-submission-reconciliation.md).
- Keep automatic fallback off unless the measured report passes the gate and the product owner approves rollout.
