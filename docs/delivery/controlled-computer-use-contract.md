# Controlled Computer Use execution contract

| Property | Decision |
| --- | --- |
| Status | Frozen control-plane contract plus verified gate-one and gate-two visible-browser takeover calibration; unattended real Browser Driver remains unavailable |
| Baseline | `upstream/main` at `e62a95c` (PR #50 merged) |
| Delivery owner | Final configuration, accepted/modified feedback binding, controlled ChangeSet, remote-write Approval, business Execution, `oceanengine-ecommerce-manual` calibration definition, business diff and result interpretation |
| Platform owner | Environment, BrowserProfile, SessionLease, ComputerUseRun/Step/Event/Evidence, site policy, takeover, final-confirmation audit and Kill Switch |
| Real-driver phase | The 2026-08-12/13 walkthroughs calibrated project and promotion forms. On 2026-08-14 one explicitly authorized promotion was submitted exactly once and independently read back; PR #50 still has no unattended real Browser Driver. |

This contract is additive. It does not reinterpret, rewrite, or migrate any historical payload or canonical hash.

## Frozen cross-layer decisions

| # | Concern | Frozen decision |
| --- | --- | --- |
| 1 | Compile-time versus run-time write authority | `CompiledDeliveryWorkflow/v1` remains immutable with `remote_write_enabled=false`. Its one `remote_write` step remains blocked by `PHASE_C_REMOTE_WRITE_PROHIBITED` in Go, JSON Schema, OpenAPI, and MySQL. A run-time `ControlledActionAttempt` references the immutable workflow step identity and is authorized independently only when the run has a valid formal Approval, an unconsumed final confirmation, a valid exclusive Lease, a matching site/account/project allowlist, and no active Kill Switch. |
| 2 | Independent persistence | Shared Computer Use authority is stored under `internal/platform/computeruse`, a platform migration, and `/platform/v1/computer-use/**`. It never reuses `delivery_observatory_runs`, observatory feedback, or their JSON envelopes. The observatory no-write checks remain unchanged. |
| 3 | Execution and Run relationship | Delivery Execution is the business execution aggregate. ComputerUseRun is the controlled UI-session aggregate. They retain separate state machines and are joined by stable immutable references; page/session JSON is not embedded in historical Delivery evidence. |
| 4 | Formal Delivery authority | Historical `delivery_approvals` rows remain `action=execute`, `scope=execute_mock`, `source=mock` and cannot be promoted. The controlled slice uses versioned formal authority records bound to the accepted/modified Observatory feedback, Selection, Decision, Plan/Intent, final PlatformConfiguration, Workflow, and `oceanengine-ecommerce-manual@v0.1-calibration`. Existing business executions gain only an additive nullable ComputerUseRun reference for controlled executions; old rows remain byte-for-byte and semantically unchanged. |
| 5 | Remote-write Approval | The formal approval binds organization, cookies Project, platform account, target object fingerprint, action, CNY budget ceiling, Plan/Intent/Feedback/Decision/Configuration/Workflow versions and hashes, the stage B Skill calibration ID/version, approver, issue time, expiry, and a canonical action hash. The Skill binding identifies an approved calibration baseline, not an executable transport. Extending the legacy approval table would require weakening its `execute_mock` checks, so a new versioned authority table is used instead. |
| 6 | Controlled ChangeSet | A controlled ChangeSet can be created only from the latest immutable `accepted` or `modified` OperatorFeedback. It freezes the feedback outcome and canonical hash plus the Selection, Decision, Plan/Intent, final Configuration, Workflow, account, object fingerprint, action and budget bindings. Rejected feedback, expired inputs, a non-final feedback record, or any drift is rejected. |
| 7 | Run states | `queued -> environment_check -> awaiting_takeover -> preparing -> awaiting_confirmation -> submitting -> verifying -> succeeded|failed|partial|result_unknown|cancelled`. Pause and takeover are explicit control flags/events, not invented terminal states. Resume is permitted only from a paused/takeover condition and starts with page/account re-identification. `result_unknown` has query/reconcile/takeover recovery only and never a normal submit retry. |
| 8 | Final confirmation | A confirmation is issued with a short TTL and a cryptographically random bearer secret; persistence stores only its digest and audit metadata. It binds the exact organization, Project, account, object fingerprint, action, budget ceiling, Plan/Intent/Feedback/Decision/Configuration/Workflow and Run tuple, plus a registered Skill pair when one exists. Consumption is atomic and idempotent for the same attempt; replay, expiry, rejection, drift, or cross-scope use creates an audit event and cannot authorize an action. |
| 9 | Environment, Profile and Lease | Environments and BrowserProfiles are organization/platform/account scoped; a run is additionally Project scoped. A SessionLease has a TTL, heartbeat deadline, owner, fencing token and version. Only one active write lease may exist for a Profile/account. Project-scoped acquire, heartbeat, and release ports validate the Run/Lease binding; release atomically detaches the lease so recovery can reacquire with a strictly higher fencing token. After a submit click, an expired original lease may be replaced only by a query/reconciliation lease on the same Run; the original Attempt binding stays immutable and no new submit may be authorized. |
| 10 | Real driver | No unattended real driver exists in PR #50. CI and E2E use only the deterministic fake worker. Visible-browser takeover calibrated both forms, one exact final click, post-write object-ID/status readback and independent list reconciliation. This does not authorize autonomous navigation or submission. |
| 11 | Kill Switch | Authoritative switches exist at global, platform and organization scopes. Any active level blocks creation/authorization of a new write attempt and wins races through a transaction/version check immediately before the action. It does not block read-only Run/Event/Evidence queries, pause, cancel, or user takeover. |
| 12 | Site and identity allowlist | Policy binds HTTPS protocol, exact host, platform, account reference, allowed page kinds, and the platform project IDs explicitly confirmed for the current run. Page text, pop-ups, direct messages, downloads and creative copy are untrusted input and cannot expand policy or authority. |
| 13 | Step evidence and privacy | Evidence records before/after page facts, field readback, diffs, page/screenshot references, object fingerprint, selector/action versions, errors and takeover state. A registered Skill version is recorded only when one actually exists. Phone numbers, email, customer/advertiser names, balances and other organization identity are redacted before persistence. Raw screenshots, when separately retained for short-lived diagnosis, are encrypted and access controlled. Clipboard is off by default and cleared after temporary use. Passwords, 2FA, cookies, tokens, browser storage and raw keystrokes are never recorded. |
| 14 | PlatformEntityMapping | Delivery owns the versioned mapping from internal Plan/Configuration/Execution/Run to external project/promotion IDs within organization, cookies Project and account scope. A mapping becomes `confirmed` only when server-loaded `result_observed` and `list_confirmed` Evidence from the same Run agree on object ID and normalized platform status. Confirmation atomically closes the ControlledExecution as `succeeded` and ChangeSet as `executed`; otherwise the mapping stays pending and the business result is `partial` or `result_unknown`. |
| 15 | Result semantics | `succeeded` requires the intended effect plus confirmed post-write mapping; `failed` proves the target effect did not occur; `partial` preserves confirmed completed scope and proposes separately approved compensation; `result_unknown` means neither success nor absence is proven and permits only query, re-identification or takeover. A timed-out lease or browser disconnect after the click never creates a resubmit path. |
| 16 | Worker and takeover ports | Fake and future real workers use the same authorize-step, begin/complete-step, heartbeat, pause/resume/cancel/takeover and confirmation-consume service ports. A visible-browser takeover records gate-one evidence actions and may atomically authorize exactly one operator-controlled final click after zero-diff readback. Post-click ports record only result outcomes and independent list confirmation; they never perform or retry the browser click. HTTP handlers only call the authority service. Unit, migration, contract and normal E2E tests cannot configure a real advertising-platform transport. |
| 17 | Platform Skill status | `oceanengine-ecommerce-manual@v0.1-calibration` is the frozen implementation ID for “巨量引擎·电商手动投放”. After the 2026-08-14 one-click validation its status is `gate_two_passed_takeover_submit_calibration`: `submit_allowed=true` only under the current-turn exact authority documented in its `SKILL.md`, while `executable=false` and `real_browser_driver=false` continue to reject unattended execution. See `oceanengine-platform-skill-calibration.md`. |
| 18 | Production control-plane assembly | The API host mounts the persisted takeover-only Computer Use surface. Environment, BrowserProfile and SitePolicy registration, Run/Lease/Event/Evidence control, and Kill Switch administration therefore survive process restarts. A client creates a Run by naming a formal Delivery `ControlledExecution`; the server resolves and revalidates the immutable ChangeSet/Approval binding and attaches the resulting Run back to that Execution. Clients cannot supply authority JSON. The production mount has no deterministic fake adapter, no unattended page driver, and no `prepare` or `submit` worker command. |
| 19 | Finite promotion modifications | Budget, schedule and authorized-material modifications always start from one confirmed promotion `PlatformEntityMapping`. The server, not the client, resolves the exact platform object ID and original creation provenance. Each modification freezes the current/target values and their canonical hashes in a new ChangeSet, receives a new Approval and Execution, and binds the target Mapping ID/version. Creation Approval reuse and in-place edits of an already confirmed Mapping are invalid. |
| 20 | Mutation readback and Mapping revisions | Before the one permitted save click, takeover evidence must match the exact platform object ID plus current and target state hashes with zero diff keys. Result and independent list evidence must both match the same object ID, platform status and target state hash. Only then may one transaction increment the Mapping version, set its current-state hash, append an immutable `delivery_platform_entity_mapping_revisions` row, and close the new Execution and ChangeSet. The original creation revision and evidence remain queryable. |
| 21 | Emergency pause | `pause_promotion` is an independent high-priority action, not a budget mutation and not the ComputerUseRun pause control. It starts only from a confirmed promotion Mapping whose normalized status is exactly `delivering`, binds the account, parent project, exact object, Mapping version, unchanged daily budget and one operator, and server-forces the target status to `paused`. A fresh ChangeSet, Approval, Execution, Run and one-time confirmation are mandatory. Result and list readbacks must both prove the same object is `paused`; uncertainty permits query or takeover only, never another click. |
| 22 | Controlled restart | `resume_promotion` is a fresh action after, and only after, an authoritative `pause_promotion` Mapping revision. It is neither automatic compensation nor Approval reuse. The current budget must equal the paused authority; the new ChangeSet binds an active `Asia/Shanghai` schedule, authorized available materials, an authorized available landing page, the exact account/project/object/Mapping and one operator. Approval validation and the final click recheck schedule validity; pre-click evidence must also prove no account/project/object drift. Any active Kill Switch wins the authorization transaction. Result and list readbacks must both normalize to `delivering`. |

## Stable blocking reasons

Run-time authorization uses a separate versioned enum and never reports `PHASE_C_REMOTE_WRITE_PROHIBITED` as a run failure:

- `FINAL_CONFIRMATION_REQUIRED`
- `FINAL_CONFIRMATION_INVALID`
- `APPROVAL_INVALID`
- `LEASE_INVALID`
- `KILL_SWITCH_ACTIVE`
- `ACCOUNT_MISMATCH`
- `PROJECT_NOT_ALLOWED`
- `SITE_NOT_ALLOWED`
- `PAGE_DRIFT`
- `WORKFLOW_DRIFT`
- `SKILL_DRIFT`
- `RESULT_RECONCILIATION_REQUIRED`

## Active PR relationship

- PR #49 is merged and this branch starts from its merge commit.
- PR #43 has no semantic overlap with this control plane and currently merges cleanly.
- PRs #46 and #47 already conflict with current `upstream/main`. They overlap the platform OpenAPI, HTTP assembly and frontend shell aggregation files expected here, but not Computer Use ownership. This implementation keeps new behavior in isolated module/handler/component files and limits shared-file edits to registration. Three-way merge checks are repeated after each milestone; an unresolvable semantic conflict stops delivery.

## Release gates

The project-form portion of gate one ran on 2026-08-12 with an exact test
account, the user-authorized scope of all existing projects in that account,
the action “fill unsubmitted form”, and current-page budget validation. It
reidentified the account and page, established semantic locators, read every
filled field back, stopped at `保存并新建单元` / `保存并关闭`, discarded the
draft, and observed the same 163-project count. That first observation was
partial; the 2026-08-13 promotion-form replay later completed gate one.

The deterministic fake adapter has no network client, credentials, Connector
dependency, or production process wiring. Its terminal outcomes are test
projections, never evidence of a platform mutation. Project-scoped control
handlers re-authorize organization, Project and scope, then delegate to the
authority service; they do not rebuild lease, confirmation, Kill Switch or
transition policy. The production host mounts only the persisted takeover
control plane and returns not-found for fake-worker `prepare` and `submit`
commands. Gate one has completed the promotion-form and persisted live-evidence
calibration. The 2026-08-14 visible takeover validated one exact final click and
two-readback Mapping reconciliation. Future final submissions remain outside any
unattended production worker and require a fresh separate execution-turn authorization.

## Finite modification readiness

The control plane and deterministic fake path support three exact actions:

- `update_promotion_budget`
- `update_promotion_schedule`
- `update_promotion_materials`

Every material reference must carry a server-owned authorization Evidence ID
from the same organization, cookies Project and platform account. That Evidence
must expose the exact `authorized_material_reference_id`; an arbitrary client
reference or cross-account Evidence record is rejected.

The public compile port is
`POST /api/delivery/v1/projects/{project_id}/platform-entity-mappings/{mapping_id}/controlled-change-sets`.
It accepts an expected Mapping version and typed current/target values; it does
not accept a platform object ID. Mapping advancement uses
`POST .../platform-entity-mappings/{mapping_id}:confirm-mutation` with one new
business Execution and two server-loaded Evidence IDs.

This implementation does not itself authorize a real edit. The current Skill
still marks remote-object modification as pending. The first live calibration,
if separately authorized in that execution turn, is limited to one budget
change on the previously created dedicated test promotion. If the platform
disallows editing while the promotion is under review, the run records the
platform blocker and stops; it must not evade the platform state or substitute
another object.

## Emergency pause readiness

The control plane and deterministic fake path support `pause_promotion` without
calling a Connector. The dedicated endpoint is
`POST /api/delivery/v1/projects/{project_id}/platform-entity-mappings/{mapping_id}/emergency-pause-change-sets`.
The request supplies only the expected Mapping version, current normalized
status and current daily budget. The server resolves the platform object,
requires `delivering`, binds the requesting operator and forces the target to
`paused`. Successful result/list evidence advances the Mapping through
`POST .../platform-entity-mappings/{mapping_id}:confirm-change`.

This is not a live-page calibration claim. `pause_remote_object` remains
forbidden by the current Skill. A real pause requires a separately authorized,
already-delivering dedicated test object in that execution turn. The promotion
created during the earlier one-click validation remains `pending_review` and
must never be enabled merely to manufacture a pause test. If the page, status,
account, object or operator is uncertain, record the blocker or `PAGE_DRIFT`
and stop without a second click.

## Controlled restart readiness

The control plane and deterministic fake path support `resume_promotion` only
when the confirmed Mapping's latest state action is `pause_promotion`, its
normalized status remains `paused`, and its state hash matches that pause. The
request cannot choose a new budget: current and approved daily budget must both
equal the value frozen by the pause. It supplies one currently active
`Asia/Shanghai` schedule plus sorted material references and one landing-page
reference whose server-loaded evidence proves the same organization, cookies
Project, account, authorization and availability.

The final visible-browser readback must match the exact account, parent project,
promotion, paused status, daily budget, schedule hash, material-reference hash
and landing-page reference, with both availability checks true. The schedule is
revalidated when the final confirmation is issued and again immediately before
authorization. Global, platform or organization Kill Switch state is checked
transactionally and blocks the click. Successful result and list evidence must
both show the same object as `delivering` before the Mapping advances.

This remains a fake/no-write readiness claim. The current Skill forbids live
remote-object restart until the specific status-control path is calibrated and
the user grants current-turn authority for that exact object and action. No
Connector, mock metric or recovery routine may trigger a restart.
