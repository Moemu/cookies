# Document vision submission reconciliation

Use this runbook only when a document is blocked by `DOCUMENT_VISION_SUBMISSION_UNKNOWN`, `DOCUMENT_VISION_SUBMISSION_INVALID`, or `DOCUMENT_VISION_CHECKPOINT_FAILED`. The purpose is to prevent a second paid LAS submission when the first request may already have been accepted.

## Safety rules

- Two different authenticated organization administrators are required. Both need project access and the `knowledge.document_vision.reconcile` scope; the scope is not granted to members, auditors, or service identities.
- Never paste an API key, authorization header, signed URL, bucket/object path, raw LAS response, or provider-console URL into Cookies. Store the investigation in the approved ticket system and send only an opaque reference such as `ticket:LAS-2026-001`.
- An ambiguous result is not `not_accepted`. Reject or leave the proposal pending and escalate to provider support. Never retry merely because a task was not found in one search.
- `approve:false` rejects the Cookies proposal; it does not assert that LAS rejected the upstream request.

## Operator A: investigate and propose

1. List safe, project-scoped candidates:

   `GET /platform/v1/projects/{project_id}/knowledge/document-vision-reconciliation-candidates?limit=50`

   Record the returned document, attempt, task index, exact pages, intent ID, provider/model/route lineage, error code, and timestamp in the ticket. The API intentionally omits credentials, object-storage coordinates, checkpoint bodies, provider raw responses, and document content.

2. In the same LAS account and region used by the frozen route, inspect account-side tasks. The official `@volcengine/las-cli` package documents `las-cli task list` and `las-cli task status <task_id> --operator las_pdf_parse_doubao`; it also documents wait and kill commands. Use list/status only for reconciliation unless a separate change authorizes another action.

3. Cross-check account, region, operator, submission time window, selected page range, route/model lineage, and task state. If necessary, open a provider-support ticket. The reviewed LAS REST contract does not document lookup by the Cookies intent ID, so do not treat a weak name/time match as proof.

4. If LAS definitely accepted the request, propose `accepted` with the exact external task ID:

   ```json
   {
     "task_index": 0,
     "expected_intent_id": "<64-lowercase-hex-intent>",
     "decision": "accepted",
     "external_task_id": "<exact-las-task-id>",
     "evidence_ref": "ticket:LAS-2026-001"
   }
   ```

   Send it to:

   `POST /platform/v1/projects/{project_id}/knowledge/documents/{document_id}/vision-reconciliations`

5. If LAS or provider support definitely proves the request was not accepted, propose `not_accepted` without `external_task_id`. If proof is incomplete, stop.

## Operator B: independently verify and confirm

1. Authenticate with a different administrator account.
2. Read the proposal before acting:

   `GET /platform/v1/projects/{project_id}/knowledge/document-vision-reconciliations/{reconciliation_id}`

3. Independently inspect the ticket and LAS account evidence. Do not rely only on Operator A's conclusion.
4. To accept the proposal, send `{"approve":true}` to:

   `POST /platform/v1/projects/{project_id}/knowledge/document-vision-reconciliations/{reconciliation_id}/confirm`

   To reject a flawed or insufficient proposal, send `{"approve":false}`. A rejected proposal leaves the paid task blocked and permits a corrected proposal.

## Expected outcomes

- Approved `accepted`: Cookies binds the unique external task ID to the frozen intent, changes the task to `submitted`, and schedules polling with a reconciliation-specific idempotency key. It does not call LAS submit again. A scheduler outage leaves an applied-but-unscheduled record that the reconciler retries safely.
- Approved `not_accepted`: Cookies marks the old task failed, keeps existing Tika text/chunks available, and permits a later explicit user visual-fallback attempt with a new attempt ID. It does not automatically start that paid attempt.
- Rejected proposal: the uncertain task remains blocked.

After applying a result, verify the candidate has disappeared from the candidate list and retain the ticket according to the operational audit policy.

## Current provider boundary

The reviewed public LAS submit/poll documentation returns a task ID only after submit and does not document a client idempotency key or reverse lookup by client request ID. The official CLI package documents account-side task listing and status lookup, which can support human investigation but is not yet an automated proof mechanism. A real-account canary must validate task-list visibility and matching fields before production use.

References: [LAS operator overview](https://www.volcengine.com/docs/6492/2196029?lang=zh), [LAS PDF parse operator](https://www.volcengine.com/docs/6492/2172371?lang=zh), [official LAS CLI package](https://www.npmjs.com/package/@volcengine/las-cli).
