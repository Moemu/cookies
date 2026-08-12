# Strategy Workspace Phase 7 — Document Parse v2

## Outcome

Phase 7 establishes the document parsing skeleton before any automatic visual fallback is introduced. The user now sees persisted parse progress, routing-quality signals, partial results, a bounded preview, source locators, and an authorized original-file link. Low-quality text remains usable and becomes a manual `shadow_fallback_recommended` candidate; it does not start a vision model automatically.

## Architecture delivered

- Added `platform-document-parse/v2` persistence fields and migration rehearsal for parse strategy, phase, progress, page counters, quality, preview status, and heartbeat.
- Routed Markdown and plain text through native extraction; HTML/HTM, DOCX, PDF, PPT, and PPTX use the configured Tika asynchronous path.
- Persisted checkpoints at queued `0`, scanning `5`, extracting `20`, quality checking `70`, chunking `85`, and terminal `100`.
- Treated quality as `routing_signal_not_accuracy`. The heuristic checks text volume, replacement/control characters, meaningful-character ratio, page density, locator coverage, and fragmented reading-order signals.
- Preserved extracted text and chunks as `partial` when quality is low. A failed retry also preserves an earlier usable result.
- Kept existing chunks searchable while a retry is in flight.
- Added a bounded preview endpoint: at most 40,000 runes and 24 source locators. The document list never returns extracted text.
- Added a no-store, project-authorized original-content proxy. Bucket names and object keys remain private.
- Made Activity use persisted document phase/progress/heartbeat instead of inferring document state from Job progress.
- Standardized new single-bucket keys:
  - `quarantine/{organization}/{project}/{session}`
  - `assets/{organization}/{project}/{asset-or-knowledge-document}/...`
  - `provider-output/{organization}/{project}/{digest}`
- Added exact quarantine-scope validation before signing, proxy upload, and promotion.

## UX delivered

- Replaced the reference-only evidence rail with a project materials drawer.
- Shows queue state, phase, percentage, page counters when truly available, quality tier and reasons, partial/failure recovery, current Brief references, and original-file access.
- Loads preview only after the user selects a document.
- Labels quality explicitly as a routing signal rather than an accuracy score.
- Explains that visual parsing is a shadow recommendation and will not silently add model cost.
- Supports `.md`, `.txt`, `.html`, `.htm`, `.docx`, `.pdf`, `.ppt`, and `.pptx` in the materials and conversation upload surfaces. HTML accepts only bounded HTML/XHTML/octet-stream MIME combinations; unrelated active-content MIME is rejected.
- Responsive behavior verified at 1440×820 and 820×900 without horizontal overflow.

## Verification evidence

- `go test ./...` — passed.
- `go vet ./...` — passed.
- `npm test` — 172/172 passed.
- `npm run contract:check` — passed.
- `npm run build` — passed; the existing large main-chunk warning remains non-blocking.
- Real MySQL migration rehearsal on local `127.0.0.1:3307/cookies` — passed.
- `TestKnowledgeCenterMySQLProjection` with real MySQL — passed, including partial-result reuse, preview, original bytes, Activity projection, cancellation, and retry.
- Strategy MySQL vertical slice and bounded product-event tests — passed.
- Browser QA — desktop and narrow layouts passed; document preview loaded, original link was scoped, no horizontal overflow, and no console errors were observed.
- `git diff --check` — passed.

## Adversarial review

### Resolved P0/P1 findings

1. A retry originally kept chunks in storage but temporarily excluded them from search. Search now accepts any document with persisted chunks, so a retry cannot make an earlier usable result disappear.
2. Existing documents without a measured quality score originally serialized an empty summary object. Null quality summaries are now omitted, avoiding fabricated measurements.
3. Job progress alone could lag at 85 after the document transaction completed. Activity now reads the document's persisted terminal phase and progress as authority.
4. Upload records could theoretically contain an out-of-scope quarantine key. Signing, proxy upload, and promotion now require the exact organization/project/session key.
5. The real MySQL cancellation test initially raced with the local QA worker. The worker was stopped and the isolated test rerun; this confirmed the domain logic rather than weakening the assertion.

### Explicit non-goals / follow-up

- No automatic image conversion or vision-model call is implemented in Phase 7. That belongs to the separately measured Phase 8 fallback shadow.
- No claim is made about time saved yet. Heartbeats and terminal timestamps now provide the data needed to measure queue time and parse duration before/after the later fallback rollout.
- Page-level progress is shown only when reliable total-page metadata exists. Otherwise the UI states that it is showing real milestones.
- The existing Vite main-chunk size warning remains a P2 performance follow-up and does not affect this phase's correctness.

## Post-merge `DOC-04` evidence follow-up

The final requirement ledger found that the original quality summary exposed image/table signal counts but not a stable density, and that `empty_pages` was never populated. It also found that a metadata key such as `empty_pages` could be mistaken for the total page count, while a dimension key could be over-counted as an image occurrence.

The quality evaluator now:

- keeps total-page, blank-page, image and table metadata semantics separate;
- accepts explicit zero counts without inventing a positive signal;
- rejects image width/height/dimension metadata as occurrence counts;
- reports bounded image/table signal density and blank-page ratio only when total pages are known;
- adds a visible blank-page routing warning while continuing to label all metrics as routing signals, not accuracy.

The Materials drawer presents text density, locator coverage, image/table signal density, blank pages and reading-order state in one compact responsive definition list. It explicitly says that metadata signal density is not a real page-share measurement. Go unit tests, 205/205 frontend tests, the production build (Strategy route 214.07 kB / 230 kB budget), the full local CI-parity script, and both real-browser document journeys pass after this closure.
