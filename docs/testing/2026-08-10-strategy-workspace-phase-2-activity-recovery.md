# Strategy workspace Phase 2 Activity and recovery evidence

Date: 2026-08-10

Status: passed for the local development gate. No commit, push, production migration, or deployment was performed.

## Scope delivered

- Added one read-only `TaskActivity` projection over the existing AgentTask, platform Job, ResearchRun, and KnowledgeDocument authorities.
- Added project/workspace-scoped Activity REST snapshots and SSE full-snapshot events. Activity does not own commands, attempts, or terminal state.
- Added explicit cancel and retry commands for document parsing and deep research. A retry creates a fresh platform Job attempt while preserving the domain resource identity.
- Added continuous Job-to-domain reconciliation so failed or cancelled Job state cannot leave a Document or ResearchRun permanently active.
- Fixed expired-lease recovery for a running Job with an accepted cancellation request: it becomes terminal `cancelled` instead of an unclaimable queued zombie.
- Replaced the Strategy frontend's separate Agent, selected Research, conversation Research, and Document polling loops with one Activity stream plus selective terminal-resource reconciliation.
- Replaced the placeholder background-task panel with real progress, phase, heartbeat/update age, research round, confirmed conclusions, failure reason, cancel/retry, and resource-open actions.
- Kept media-understanding polling unchanged because downstream image/video/audio production is outside this phase's boundary.

## Runtime authority and recovery rules

| Concern | Authority | Activity behavior |
| --- | --- | --- |
| Agent execution | `platform_agent_tasks` | Projects running/terminal state; cancel delegates to the existing Agent command |
| Document parse | KnowledgeDocument + latest matching platform Job | Cancel/retry delegates to Knowledge; terminal Job is reconciled to document status |
| Deep research | ResearchRun + latest matching platform Job | Shows round/conclusions; cancel/retry delegates to Knowledge; terminal Job is reconciled to run status |
| Retry attempt | `platform_jobs` | Always a fresh Job and idempotency key; no terminal Job is reopened |
| SSE recovery | Activity REST read model | Every connection immediately receives a complete content-addressed snapshot; the client de-duplicates by `snapshot_id` |

The frontend stores only the last snapshot ID as a session reconnect optimization. REST and the underlying domain tables remain the source of truth.

## API and contract surface

Activity:

- `GET /api/strategy/v1/projects/{project_id}/activities`
- `GET /api/strategy/v1/projects/{project_id}/activities/events`
- `strategy-task-activity/v1`
- `strategy-task-activity-snapshot/v1`

Knowledge control:

- `POST /platform/v1/projects/{project_id}/knowledge/documents/{document_id}/cancel`
- `POST /platform/v1/projects/{project_id}/knowledge/documents/{document_id}/retry`
- `POST /platform/v1/projects/{project_id}/knowledge/research-runs/{run_id}/cancel`
- `POST /platform/v1/projects/{project_id}/knowledge/research-runs/{run_id}/retry`

A running cancel reports `cancel_requested` until the provider/worker or expired-lease recovery makes the task terminal. The UI does not claim that work stopped synchronously.

## Fault and adversarial verification

| Scenario | Evidence | Result |
| --- | --- | --- |
| Ordinary running lease expires | MySQL integration requeues and permits a later claim | Passed |
| Cancel requested, then Worker disappears | MySQL integration converts the expired running Job to `cancelled`; it is not requeued into an unclaimable state | Passed |
| Document parse cancel and retry | MySQL integration creates a fresh retry Job and preserves the document resource | Passed |
| Research cancel and retry | MySQL integration creates a fresh retry Job and preserves the research run | Passed |
| Retry Job fails after the command was accepted | Reconciler changes active Document/ResearchRun to a bounded failure state | Passed |
| An older terminal Job exists after a newer retry started | Reconciler ignores the stale Job by comparing Job/domain update time and checking for newer Jobs | Passed |
| SSE connection starts with the current `Last-Event-ID` | Server still sends one immediate complete snapshot, avoiding proxy-buffered fake connecting state | Passed in rendered local QA |
| SSE drops after connecting | Client performs REST reconciliation before bounded exponential reconnect | Covered by reducer/source tests; destructive live network injection remains a later E2E hardening case |

## Rendered browser QA

Environment:

- Local Vite application: `http://127.0.0.1:5173`
- Local Go API: `http://127.0.0.1:8080`
- Project: `project_dee069688be97141ad7ba891e97622f0`
- Workspace: `strategyws_4cec9f802ea12cbf2ac8c7b155db2e14`
- Browser viewport: 1280 × 720

Flow verified:

`需求与策略 → 策略任务 → 策略工作区 → 后台任务 → 打开文档解析任务 → 对应资料资源`

| Check | Result |
| --- | --- |
| Page identity and canonical workspace URL | Passed |
| Meaningful content; no blank shell or framework overlay | Passed |
| Prominent Demand/Research/Strategy/Review hub remains visible | Passed |
| Activity reports `状态实时同步` after reload | Passed |
| Existing Agent and Document tasks restore from the server | Passed |
| Completed progress is rendered as real 100% task state | Passed |
| Opening the Document activity selects its exact material resource | Passed |
| Application console errors/warnings | None |
| Screenshot evidence | Captured through the in-app browser and attached to the validation task; not written into the repository |
| Mobile/narrow viewport | Not run: this Browser runtime exposes screenshots but no viewport-resize operation |

Rendered QA initially exposed and fixed an SSE startup defect: when `Last-Event-ID` already matched the current snapshot, a header-only response could be buffered by the local proxy and leave the UI on `正在连接任务状态` until the heartbeat. The protocol now sends an immediate complete, idempotent snapshot on every connection.

## Automated gates

| Gate | Result |
| --- | --- |
| `go test ./...` | Passed |
| MySQL JobRuntime + Knowledge fault integration tests | 3 passed, including cancelled lease recovery and Knowledge cancel/retry/reconcile |
| `npm test` | 160 passed, 0 failed |
| `npm run contract:check` | 7 frontend contract tests plus Go contract/integration packages passed |
| `npm run build` | Passed; 1,826 modules transformed |
| `git diff --check` | Passed |

Build evidence:

- Strategy workspace lazy JS: 144.35 kB, gzip 43.98 kB.
- Strategy workspace lazy CSS: 20.96 kB, gzip 3.95 kB.
- Main JS: 1,115.71 kB, gzip 315.25 kB.
- Global CSS: 441.29 kB, gzip 73.33 kB.
- Vite still reports the existing main-chunk-over-500-kB warning. The Strategy workspace remains lazy-loaded; global bundle splitting is not part of Activity recovery.

## Adversarial review

1. **A9: third task platform.** Rejected in implementation. Activity has no table, queue, worker, write command, or attempt lifecycle. Cancel/retry route back to existing Agent/Knowledge/Job authorities.
2. **A10: SSE as false authority.** Rejected in implementation. SSE contains complete content-addressed snapshots, REST uses the same projection, reconnect reconciles through REST, and terminal transitions trigger a targeted read of the affected resource.
3. **False cancellation.** Running cancellation is explicitly a request, remains visually active as `cancelling`, and becomes terminal only through worker acknowledgement or lease recovery.
4. **Worker crash freeze.** A cancelled expired lease no longer becomes queued while being excluded from Claim. Continuous reconciliation also repairs the user-facing domain state after terminal Job changes.
5. **Retry races.** Retry does not reopen an old Job. The reconciler refuses to apply an older terminal attempt over a newer resource update or newer Job.
6. **Refresh and window close.** Activity state is reconstructible from REST. Session storage contains only a snapshot cursor, not business state.
7. **Polling multiplication.** Four Strategy polling loops were removed. Media-understanding polling remains deliberately isolated at the downstream boundary.
8. **Unbounded payload/UI.** Activity summaries and conclusions are bounded; the schema rejects hidden reasoning and requires explicit progress semantics.
9. **SSE proxy behavior.** A first full data frame is forced even for an unchanged cursor, preventing header buffering from masquerading as a slow backend.
10. **Scope isolation.** Activity queries require project authorization and optionally filter by workspace; resource commands retain their existing organization/project checks.

## Deferred work

- Phase 3 owns project context manifests, compacted Assistant memory, proposal accept/edit/ignore, unread state, and resizable docking.
- Phase 4 owns the bounded multi-round deep-research loop and research-to-Brief/Strategy adoption proof.
- Phase 7/8 own honest document page/checkpoint progress, quality scoring, preview, and visual fallback. This phase only makes the current parse Job observable and recoverable.
- Active SSE disconnect/replay is now covered by the Strategy browser suite, provider timeout/hang behavior has direct bounded fault tests, and a real-MySQL integration starts two separate OS processes to prove Worker A can exit after claiming while Worker B restores the durable payload and finishes after lease recovery. A whole API-host process kill remains a deployment smoke concern rather than an unverified JobRuntime persistence boundary.
