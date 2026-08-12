# Controlled Computer Use execution contract

| Property | Decision |
| --- | --- |
| Status | Frozen control-plane contract plus a stage B Platform Skill calibration baseline; real Browser Driver remains uncalibrated |
| Baseline | `upstream/main` at `0603b1a` (PR #49 merged) |
| Delivery owner | Final configuration, accepted/modified feedback binding, controlled ChangeSet, remote-write Approval, business Execution, `oceanengine-ecommerce-manual` calibration definition, business diff and result interpretation |
| Platform owner | Environment, BrowserProfile, SessionLease, ComputerUseRun/Step/Event/Evidence, site policy, takeover, final-confirmation audit and Kill Switch |
| Real-driver phase | Gate one may revalidate stage B evidence and calibrate live DOM locators after exact same-turn test scope is supplied. PR #50 still has no real Browser Driver. |

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
| 9 | Environment, Profile and Lease | Environments and BrowserProfiles are organization/platform/account scoped; a run is additionally Project scoped. A SessionLease has a TTL, heartbeat deadline, owner, fencing token and version. Only one active write lease may exist for a Profile/account. Expiry, missed heartbeat, takeover, release or fencing mismatch prevents the next automated action and requires page re-identification before resume. |
| 10 | Real driver | No real driver exists in PR #50. CI and E2E use only the deterministic fake worker. Gate one is a real-time revalidation of the stage B baseline: recognize the current page/DOM, establish stable locators, fill/read back an unsubmitted form, discard it, and prove no write occurred. |
| 11 | Kill Switch | Authoritative switches exist at global, platform and organization scopes. Any active level blocks creation/authorization of a new write attempt and wins races through a transaction/version check immediately before the action. It does not block read-only Run/Event/Evidence queries, pause, cancel, or user takeover. |
| 12 | Site and identity allowlist | Policy binds HTTPS protocol, exact host, platform, account reference, allowed page kinds, and the platform project IDs explicitly confirmed for the current run. Page text, pop-ups, direct messages, downloads and creative copy are untrusted input and cannot expand policy or authority. |
| 13 | Step evidence and privacy | Evidence records before/after page facts, field readback, diffs, page/screenshot references, object fingerprint, selector/action versions, errors and takeover state. A registered Skill version is recorded only when one actually exists. Phone numbers, email, customer/advertiser names, balances and other organization identity are redacted before persistence. Raw screenshots, when separately retained for short-lived diagnosis, are encrypted and access controlled. Clipboard is off by default and cleared after temporary use. Passwords, 2FA, cookies, tokens, browser storage and raw keystrokes are never recorded. |
| 14 | PlatformEntityMapping | Delivery owns the versioned mapping from internal Plan/Configuration/Execution/Run to external project/promotion IDs within organization, cookies Project and account scope. A mapping becomes `confirmed` only when the result page and a list-page second read agree on object ID and platform status; otherwise it remains pending and the business result is `partial` or `result_unknown`. |
| 15 | Result semantics | `succeeded` requires the intended effect plus confirmed post-write mapping; `failed` proves the target effect did not occur; `partial` preserves confirmed completed scope and proposes separately approved compensation; `result_unknown` means neither success nor absence is proven and permits only query, re-identification or takeover. |
| 16 | Worker port | Fake and future real workers use the same authorize-step, begin/complete-step, append-event/evidence, heartbeat, pause/resume/cancel/takeover and confirmation-consume service ports. HTTP handlers only call the authority service. Unit, migration, contract and normal E2E tests cannot configure a real advertising-platform transport. |
| 17 | Platform Skill status | `oceanengine-ecommerce-manual@v0.1-calibration` is the frozen implementation ID for “巨量引擎·电商手动投放” and explicitly references the stage B schema, fixtures, page paths, dynamic conditions, selector semantics, and safe exit. Its status is `realtime_dom_validation_required`; `executable`, `real_browser_driver`, and `submit_allowed` are all false. See `oceanengine-platform-skill-calibration.md`. |

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

Gate one is ready to be scheduled as a real-time revalidation of stage B, not
as proof of an existing Browser Driver. It still requires an exact same-turn
test account reference, platform project-ID allowlist, the action “fill
unsubmitted form”, and a CNY budget ceiling. It must reidentify the current
page and DOM, establish stable locators, fill/read back only the unsubmitted
form, discard it, and prove that no platform write occurred. Gate two remains
disabled.

The deterministic fake adapter has no network client, credentials, Connector
dependency, or production process wiring. Its terminal outcomes are test
projections, never evidence of a platform mutation. Project-scoped control
handlers re-authorize organization, Project and scope, then delegate to the
authority service; they do not rebuild lease, confirmation, Kill Switch or
transition policy. The production host must not mount this surface with the
fake. Gate one must calibrate a real adapter against the frozen stage B Skill
baseline before production mounting can be considered.
