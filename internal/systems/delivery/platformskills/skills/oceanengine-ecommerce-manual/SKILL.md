---
name: oceanengine-ecommerce-manual
description: Prepare, calibrate, or perform a controlled Ocean Engine ecommerce manual promotion workflow in a visible authenticated browser. Use for 巨量引擎电商手动投放 account/project identification, promotion form fill and readback, safe draft discard, or one explicitly authorized final promotion submission with fenced ComputerUseRun evidence and Mapping reconciliation.
---

# Ocean Engine ecommerce manual promotion

Treat the machine-readable [SkillDefinition](../../definitions/oceanengine-ecommerce-manual-v0.1.json) as the capability boundary. Read the relevant evidence and locator references it names before interacting with a page. Never infer a selector or permission from business semantics alone.

## Choose the gate

- Use gate one for page identification, form fill, field readback, and draft discard. Do not save or create an object.
- Use gate two only for one final submission explicitly authorized in the current user turn. A generic continuation, an earlier approval, a zero account balance, or this Skill never grants write authority.
- Keep `executable=false` and `real_browser_driver=false`: operate only through a visible authenticated browser takeover and the persisted control-plane APIs. Do not claim an unattended Browser Driver.

## Require exact gate-two authority

Before issuing a final confirmation, require all of the following:

1. The current-turn user authorization names the advertiser account, current parent project, action, object count, and CNY budget ceiling.
2. A fresh ChangeSet, unexpired Approval, ControlledExecution, independent ComputerUseRun, SitePolicy, and fenced Lease bind those exact values and the current Skill version.
3. The Kill Switch is inactive and the responsible user remains available.
4. Reidentify the visible account, parent project, page kind, action, budget, and every form field. Require an empty diff.
5. Use only account assets, landing pages, category, and brand references included in the frozen authority.

Stop on any mismatch, ambiguous page identity, stale lease, expired approval, permission error, or locator drift.

## Submit exactly once

1. Issue and atomically consume one short-lived final confirmation while authorizing one `ControlledActionAttempt`.
2. Recheck the unique enabled `保存并关闭` control and the zero-diff readback.
3. Click it exactly once. Never retry the click, even after a timeout, disconnect, or unclear navigation.
4. Do not enable delivery. Preserve the created test object unless the user separately authorizes another action.

## Reconcile without resubmitting

After the click:

1. Record `result_observed` only when an authoritative page exposes the platform object ID and normalized status.
2. Independently reload or query the promotion list and record `list_confirmed` only when object ID and status match.
3. Confirm `PlatformEntityMapping` using only those two server-loaded Evidence IDs. Let the same transaction close the ControlledExecution as `succeeded` and the ChangeSet as `executed`.
4. Release the Lease after the terminal state.

If the submission result is unclear, record `result_unknown`; permit only query, reidentification, or human reconciliation. If the original submission Lease expires after the click, release it and acquire a new fenced recovery Lease for readback. Keep the original Attempt binding immutable and never authorize or perform another submit.

## Calibrate finite modifications separately

The shared control plane can now freeze and simulate exact promotion budget,
schedule and authorized-material modifications. That implementation does not
expand this Skill's live write capability: `modify_remote_object` remains
forbidden until the specific edit path is calibrated in a visible browser.

For a no-write calibration, require a confirmed promotion Mapping and a new
ChangeSet/Approval. Reidentify the exact parent project and promotion, read the
current field values, prepare only the approved target values, read every
changed field back, and exit without saving. Record `PAGE_DRIFT` or the
platform's own edit restriction instead of selecting a different object.

If the user separately authorizes the first real modification in that same
execution turn, limit it to one budget change on the dedicated test promotion.
Require an exact pre-click object/current-state/target-state readback, consume a
new one-time confirmation, click the unique save boundary once, and reconcile
the same object and target-state hash from the result and promotion list. Never
reuse the creation Approval or the creation final confirmation.

## Keep emergency pause distinct

The shared control plane can freeze and simulate `pause_promotion`, but this
Skill still forbids `pause_remote_object` until the live status-control surface
is calibrated. Do not confuse it with pausing a ComputerUseRun.

For no-write calibration, start from one confirmed promotion Mapping whose
normalized server status is exactly `delivering`. Reidentify the account,
parent project and exact promotion, read the unchanged daily budget, verify the
unique pause control and stop before activating it. The ChangeSet must bind one
operator and server-force the transition from `delivering` to `paused`.

A real pause requires current-turn authorization naming that already-delivering
dedicated test promotion and action. Consume a fresh confirmation and click the
unique pause boundary once. Never enable a pending or inactive promotion merely
to create a pause test. After the click, require both the result and an
independent list readback to show the same object as `paused`; on uncertainty,
query or take over without clicking again.

## Read the verified baseline

- Read the [control contract](../../../../../../docs/delivery/controlled-computer-use-contract.md) for state and authority invariants.
- Read the [gate-one runbook](../../../../../../docs/delivery/oceanengine-gate-one-replay-runbook.md) for no-write calibration.
- Read the [gate-two baseline](../../../../../../docs/delivery/oceanengine-gate-two-preparation.md) for the one-click and recovery contract.
- Read the [2026-08-14 gate-two evidence](../../../../../../docs/delivery/evidence/oceanengine-gate-two-promotion-submit-2026-08-14.json) before claiming real-submit calibration.
