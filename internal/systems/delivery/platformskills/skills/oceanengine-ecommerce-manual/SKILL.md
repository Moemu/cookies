---
name: oceanengine-ecommerce-manual
description: Prepare, calibrate, or perform a controlled Ocean Engine ecommerce manual promotion workflow in a visible authenticated browser. Use for 巨量引擎电商手动投放 account/project identification, promotion form fill and readback, safe draft discard, or one explicitly authorized final promotion submission with fenced ComputerUseRun evidence and Mapping reconciliation.
---

# Ocean Engine ecommerce manual promotion

Treat the machine-readable [SkillDefinition](../../definitions/oceanengine-ecommerce-manual-v0.1.json) as the capability boundary. Read the relevant evidence and locator references it names before interacting with a page. Never infer a selector or permission from business semantics alone.

The frozen `OceanEngineCalibrationManifest` is the observation source for project-create/edit and promotion-create/edit path dimensions, field ownership, units, dynamic conditions and semantic locators. It is never a ChangeSet, Approval, Confirmation or execution authorization. Page observation and unsubmitted form calibration never authorize a save; every remote-write boundary still requires the Stage D authority chain and action-time confirmation.

For promotion forms, scope every action to its section before resolving the target. Prefer placeholders, role/name pairs, and stable platform attributes such as `data-auto-id=oc-create-product-img-add-button` or `data-e2e=createad_yuntuCategory`; never persist coordinates or DOM order. Reference pickers select the first currently available semantic item only when policy permits. Product selling points are committed with Enter and must contain 6–9 Chinese characters per item, at most 10 items. A product-image picker returning zero items is `blocked_by_missing_material`, not page drift.

The observed 展示量/CPM promotion path accepts bids from CNY 4 through CNY 100. If an intent or calibration authority caps the bid below CNY 4 (including CNY 0.01), stop before save and record `blocked_by_input_constraint`; never silently raise the bid. Promotion edit exposes identity, materials, copy, native anchor, landing page/deep-link, product information, creative components, settings, and name, while the save button remains the final write boundary.

## Fill a project-create field from the Manifest

For a no-write project-create calibration, use only a field carrying a
`computer_use` specification in the frozen Manifest. Reidentify the matching
page fingerprint first, then apply this sequence:

1. Resolve the `scope`, then resolve the `target` inside that scope; require the
   declared `expected_target_count` before interacting.
2. Use the declared operation only (`choose_exact_visible_option`,
   `open_reference_picker`, `fill_text`, `fill_money`, `toggle`, or
   `configure_object`). Do not substitute a coordinate, DOM index, or a
   same-labelled control from elsewhere on the page.
3. For a choice, accept only an `observed_options` value. For money, convert
   only according to `input_constraints` (for example CNY yuan in the page to
   CNY fen in the domain model), and never type a business value outside the
   current no-write calibration case.
4. Read the value back through the declared `readback`. Do not use a fallback
   selector if a locator is missing, non-unique, disabled unexpectedly, or the
   readback differs. Record an explicit current-page restriction as its stable
   blocked reason (for example `blocked_by_platform_capability` for a page that
   states an application platform is unsupported). Use `page_drift` only when
   the current page contradicts the frozen Manifest record; otherwise stop the
   case as incomplete calibration.

A field without `computer_use` remains observation-only. It must not be
presented as an executable form action until a later calibration adds a unique
scope, target and readback. This rule applies equally to fields marked
`evidence_only`: the Manifest may document them without granting a modelled
platform write.

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

## Calibrate finite actions as one ordered batch

Use one visible-browser session, one shared promotion-form fixture, and one
machine-readable batch evidence record for `update_promotion_budget`,
`update_promotion_materials`, `pause_promotion`, and `resume_promotion`. Do not
split these paths into separate calibration narratives. Resolve the exact object
only from a server-loaded confirmed Mapping; page search results and operator
text cannot substitute it.

During no-write calibration, reidentify the account, parent project, promotion,
Mapping revision, live composite status, and current values. The only permitted
form differences are the approved budget field or the approved material
reference. Material selection is restricted to a server-authorized existing
asset in the same test account. Read every field back, stop before the unique
save or status boundary, discard the draft, and reopen or return to the list to
prove the remote values are unchanged. Reject `update_promotion_schedule` for a
promotion Mapping because schedule remains parent-project owned.

`PlatformEntityMapping.platform_status` is the last accepted result/list
snapshot, not a live status feed. Ocean Engine can expose delivery, pause, and
review dimensions simultaneously. Compare live dimensions explicitly without
rewriting the Mapping merely because asynchronous review completed. If no
authorized mapped object is already `delivering`, mark pause and enable
`blocked_by_eligible_test_object`; never enable an object to manufacture a test.

Each real action still requires its own ChangeSet, Approval,
ControlledExecution, ComputerUseRun, one-time confirmation, Attempt, and exact
Mapping revision. Execute budget before materials so the material authority can
bind the post-budget revision. Execute enable only after a successful pause
revision. Immediately before each remote-write click, require an unexpired
fenced Lease and zero unexpected field differences. Click at most once. Any
page drift, lease expiry, or `result_unknown` stops all dependent actions without
retry.

The 2026-08-14 batch passed no-write calibration for the CNY 300 to CNY 310
budget path and one same-account existing-material replacement. Both drafts were
discarded and the fresh form returned to the shared baseline. Pause and enable
were blocked because no eligible delivering mapped test object existed. The
authorized real batch then stopped when its Lease expired before final
confirmation: no confirmation, Attempt, save click, or remote write occurred,
and the abandoned calibration authority was invalidated while its Approval
history remained immutable. Read the batch evidence before claiming any live
modification capability.

## Read the verified baseline

- Read the [control contract](../../../../../../docs/delivery/controlled-computer-use-contract.md) for state and authority invariants.
- Read the [gate-one runbook](../../../../../../docs/delivery/oceanengine-gate-one-replay-runbook.md) for no-write calibration.
- Read the [gate-two baseline](../../../../../../docs/delivery/oceanengine-gate-two-preparation.md) for the one-click and recovery contract.
- Read the [2026-08-14 gate-two evidence](../../../../../../docs/delivery/evidence/oceanengine-gate-two-promotion-submit-2026-08-14.json) before claiming real-submit calibration.
- Read the [2026-08-14 controlled-action batch evidence](../../../../../../docs/delivery/evidence/oceanengine-controlled-actions-batch-2026-08-14.json) before calibrating or authorizing any existing-promotion change.
