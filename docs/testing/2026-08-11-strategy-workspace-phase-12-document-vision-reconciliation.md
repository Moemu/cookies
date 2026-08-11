# Strategy Workspace Phase 12 — document vision submission reconciliation

## Outcome

The submission-unknown boundary now has a local, administrator-only two-person resolution workflow. It preserves the pre-call paid-submission intent, exposes only bounded reconciliation metadata, prevents reuse of an external task ID, and separates resuming an accepted LAS task from authorizing a new user attempt.

No real LAS request, production migration, deployment, commit, or push was performed.

Follow-up on 2026-08-11: a separately authorized one-page PDF canary later completed through the configured Shanghai account and reported one billable page. It did not exercise the submission-unknown reconciliation path; real-account reconciliation and PPTX-derived canary evidence remain pending.

## Implemented controls

- The worker prepares and persists a deterministic 64-hex intent, provider/model/route lineage, exact pages, TOS input/output identity, and checkpoint before calling LAS submit.
- A submit timeout marks the external task `unknown`; the document retains its Tika chunks and blocks automatic and user retry until reconciled.
- Only authenticated human administrators receive `knowledge.document_vision.reconcile`. Service principals, members, and auditors fail closed.
- A safe candidate-list API exposes document identity, exact pages, intent, provider/model/route, error, and timestamp, but no content, credential, object path, checkpoint JSON, console URL, or raw response.
- Operator A records an opaque evidence reference and proposes `accepted` with an exact external task ID or `not_accepted` without one.
- Operator B must use a different authenticated user, can read the proposal before acting, and independently approves or rejects it.
- Database checks enforce distinct proposer/confirmer identities and consistent proposal status/timestamps. Generated nullable bindings plus unique indexes prevent one external task ID from being reserved or bound more than once per organization.
- Accepted decisions resume polling with a reconciliation-specific Job idempotency key and never resubmit. Scheduling failure is recovered before generic orphan recovery.
- Not-accepted decisions preserve prior text, close the old task, and allow a later explicit user attempt with a new attempt ID; no paid task starts automatically.
- Active in-flight submissions are not candidates: the document must already be failed with a reconciliation-required error.

## Verification

- Targeted Knowledge, Identity, HTTP, contract, and composition-root tests pass.
- A fresh temporary MySQL schema applied every migration successfully.
- `TestKnowledgeCenterMySQLProjection` fault-injected a timeout after source transmission and proved intent preservation, candidate discovery, same-actor rejection, second-actor proposal inspection, accepted-task recovery without resubmit, scheduler-outage recovery, and the not-accepted explicit-retry path.
- The temporary database was dropped and its absence verified.
- JSON Schema fixtures validate the safe candidate and proposal response contracts.

## Remaining external evidence

- Confirm with Volcengine support whether a supported client idempotency or reverse-lookup mechanism exists outside the reviewed public REST contract.
- In the approved canary account, validate what `las-cli task list/status` exposes and whether the frozen route metadata is sufficient for unambiguous human matching.
- Run the separately approved bounded PDF and PPTX-derived paid canaries. Until then, visual fallback remains default-off/manual and the production release decision remains HOLD.
