# `oceanengine-ecommerce-manual` calibration baseline

`oceanengine-ecommerce-manual` is the implementation ID for the
“巨量引擎·电商手动投放” capability. Version `v0.1-calibration` promotes the
stage B read-only walkthrough into a versioned business and page-semantics
baseline. It does not claim an executable Browser Driver.

The canonical machine-readable definition is
`internal/systems/delivery/platformskills/definitions/oceanengine-ecommerce-manual-v0.1.json`.
Its `calibration_manifest` binding identifies the frozen current-account
project-create observation source at
`docs/delivery/fixtures/oceanengine-calibration-manifest-v1.json`; that
Manifest is evidence only and never grants an action or write authorization.
It binds:

- the frozen `oceanengine-bidding-schema/v0.1` business schema;
- the 2026-08-06 account → project → promotion → data center → report path;
- project/promotion field ownership and conditional display rules;
- the ecommerce manual, UBMax, leads, Android download, promotion create/edit,
  and selector fixtures;
- selector business semantics for identity, material, copy, category, brand,
  native anchor, and landing-page selection;
- filling and reading back an unsubmitted local form;
- discarding the draft and returning to a known read-only page;
- the default invariant that write actions remain forbidden unless a current-turn,
  exact gate-two authority permits one final click.

## Readiness

The 2026-08-12 and 2026-08-13 live walkthroughs advanced the definition to
`gate_one_passed_takeover_calibration`:

- stage B business semantics are `observed`/`operator_reviewed` where recorded;
- its observation date is 2026-08-06 and the platform build is unknown;
- the project-create form now has a checked-in Playwright-semantic locator
  baseline in `fixtures/oceanengine-ecommerce-manual-live-locators-v0.1.json`;
- account identity, project-list identity, product selection, optimization
  target, deep-optimization mode, manual delivery, daily budget, project name,
  write boundaries, field readback, cancel, and no-write verification were
  exercised in a visible authenticated browser;
- selecting `app内下单` reset delivery mode to automatic, so the frozen
  dynamic order reselects manual delivery after the optimization target;
- the project draft was discarded and the project count remained 163;
- promotion/unit live locators were revalidated on an existing allowlisted
  manual-delivery project without crossing a save boundary;
- the PR now exposes a fenced takeover-evidence port that atomically records
  Run version, Step, Event, and redacted Evidence for a fixed non-write action
  enum after exact site-policy validation;
- the live observations were replayed through the persisted Observatory,
  ChangeSet, Approval, ControlledExecution, ComputerUseRun, fenced Lease,
  Step, Event, and redacted Evidence chain;
- PR #50 has no unattended real Browser Driver;
- gate two was still disabled at the end of these gate-one observations.

The calibration definition is bound into controlled ChangeSets and formal
Approvals by ID and version. That binding proves which stage B baseline was
approved; it does not turn the baseline into an executable transport.

## Gate one: real-time revalidation

The user supplied a test account, authorized all existing projects in that
account for test use, authorized only “fill unsubmitted form”, and constrained
the budget to current-page validation. The persisted evidence deliberately
stores only a SHA-256 account reference and no advertiser/customer or product
name. No screenshot was persisted.

Acceptance requires all of the following:

1. Reidentify the current account, page kind, and allowlisted project.
2. Confirm whether the UI has drifted since 2026-08-06.
3. Establish stable live DOM locators without coordinate fallbacks.
4. Fill only the approved unsubmitted project/promotion fields.
5. Read every field back and compare it with the approved configuration.
6. Discard the local form/draft using the observed safe-exit path.
7. Return to a known read-only page and prove that no platform object or status
   change was created.

The project-form portion passed items 1–7. On 2026-08-13, the first promotion
observation stopped conservatively because an Orange landing-page branch and a
`10/10/10` material limit did not match the stage B sample. Operator review
then established that the landing-page label comes from the parent project's
delivery-carrier branch and that material capacity is conditional rather than
a frozen constant. The original stop remains as a corrected audit record in
`evidence/oceanengine-gate-one-promotion-drift-2026-08-13.json`.

A new allowlisted test project exposed the intended self-hosted landing-page,
`app内下单`, no-deep-optimization, manual-delivery branch. Nine approved local
fields were filled and read back with semantic DOM locators. Material, copy,
landing-page, category, and brand selectors were opened only where safe and
cancelled without applying real references. The form was discarded through
its observed `createad_save_cancel` event boundary. An exact temporary-name
search on the read-only promotion list returned zero platform objects. The
redacted result is in
`evidence/oceanengine-gate-one-promotion-form-2026-08-13.json`, and the locator
baseline is in `fixtures/oceanengine-promotion-live-locators-v0.1.json`.

A second read-only replay on the Orange landing-page/button-jump branch
confirmed that selecting `账户信息` marks the stable
`createad_nativetype_0` control as checked and hides the Douyin-account
configuration region. It also opened the category cascader through its stable
field container, observed search and top-level options, cancelled with Escape,
and discarded the form back to the read-only project list. This is recorded as
cross-branch evidence only; it does not replace the self-hosted/app-order
branch or supply an approved category reference.

The operator subsequently authorized all existing assets in the test account.
After a transient zero-result response recovered, the live form selected and
read back one existing video, one existing self-hosted landing page, one
category, and one existing brand. Together with the previously calibrated
identity, copy, product, source, budget, bid, and name fields, this completed
the observed unsubmitted promotion form. The exact temporary-name search again
returned zero objects after discard. The promotion-form portion is therefore
`passed`.

The formal control-plane replay recorded all five non-write takeover actions:
`observe_page`, `begin_form_fill`, `field_readback`, `discard_draft`, and
`verify_no_write`. The fenced lease was released and the run was cancelled.
Gate one is therefore `passed`, and the visible-browser takeover path is
calibrated. No automated Browser Driver is mounted in the production API, so
the Skill remains `executable=false` and `real_browser_driver=false`.
Submission capabilities remained disabled until a separate gate-two authorization.

The persisted replay contract is frozen in
`oceanengine-gate-one-replay-runbook.md` and
`fixtures/oceanengine-gate-one-replay-plan-v0.1.json`. Promotion locator
capture uses
`fixtures/oceanengine-promotion-live-locator-capture-v0.1-template.json`.
That file remains an empty `template_not_observed` record: it lists
the stage B fields and selector surfaces to inspect but contains no invented
DOM locator and is not evidence of calibration. The separate 2026-08-13 live
locator file contains only controls confirmed on the visible page; selected
reference values remain redacted and unvisited branches remain explicit gaps.

## Gate two: one-click validation

On 2026-08-14 the operator explicitly authorized one promotion under the persisted
account reference SHA-256 `a8c499f7e22dc70d392d8de9b7bd093a4e7371cb17ba3e53c4bf8e0eea15667c`,
the current parent project, and a CNY 300 daily-budget ceiling. Public evidence
retains only irreversible SHA-256 references for platform objects and internal
names. The previous day's parent had been deleted and was excluded from the
rebuilt ChangeSet, Approval, Execution and Run.

The visible form was read back with an empty diff, one final confirmation was
consumed, and `保存并关闭` was clicked exactly once. The promotion list then exposed
the single hashed promotion reference with budget CNY 300, bid CNY 0.01 and normalized
status `pending_review`; delivery was not enabled. A second independent reload and
exact-name query returned the same ID and status. Server-loaded Evidence confirmed
the Mapping, and the same transaction closed the ControlledExecution as `succeeded`
and ChangeSet as `executed`.

The original submission lease heartbeat expired during the post-write readback.
No submit retry occurred. A higher-fencing recovery lease was used only for result
reconciliation, preserving the original Attempt binding. This recovery rule is now
covered by a unit test. The complete audit record is
`evidence/oceanengine-gate-two-promotion-submit-2026-08-14.json`.

The Skill now has a `SKILL.md` with final-submit instructions. At rest no execution
is authorized: each write still requires a fresh exact current-turn authorization,
fresh control-plane objects, zero-diff readback, one short-lived confirmation and a
single click. `submit_allowed=true` expresses only this controlled takeover path;
`executable=false` and `real_browser_driver=false` remain unchanged, and enable,
unattended submit, resubmit, remote modification, delete and upload remain forbidden.

## Existing-object edit inventory

After gate two, a separate 2026-08-14 visible-browser walkthrough inspected the
edit surfaces for the exact hashed parent project and promotion without changing
any field. The project edit surface owns targeting, schedule/dayparting, project
budget mode, search settings, tracking links and project name. The promotion edit
surface exposes materials, copy, landing/direct-link data, product additions,
creative settings, category/brand, daily budget, bid and promotion name. Its
schedule is inherited and read-only.

This field ownership corrects the control-plane boundary: a confirmed promotion
Mapping may support only `update_promotion_budget` and
`update_promotion_materials`. `update_promotion_schedule` is invalid because it
would write a parent-project field with promotion authority. Project schedule
changes remain capability-pending until a confirmed project Mapping and separate
project-mutation contract exist.

The walkthrough also found locator drift on the promotion brand field. The old
container selector now matches two inputs; the exact visible placeholder
`选择或手动输入品牌` is the unique replacement. The Mapping retained its
creation-time `pending_review` snapshot; a later live read showed independent
not-delivering, paused, and review-completed dimensions. No eligible mapped
object was already delivering, so no pause or enable control was calibrated.
No field was filled and no save, pause or enable control was clicked. The
redacted evidence and locator baseline are respectively
`evidence/oceanengine-existing-object-edit-readonly-2026-08-14.json` and
`fixtures/oceanengine-existing-object-live-locators-v0.1.json`.

## Existing-promotion controlled-action batch

On 2026-08-14 one continuous visible-browser session calibrated four paths from
the same server-resolved confirmed Mapping revision 2. The budget draft changed
CNY 300 to CNY 310 and only the daily-budget field differed. A second draft
selected one authorized existing alternative material from the same test
account and only the material field differed. Both drafts were read back,
discarded, and reopened at the same 18-field shared baseline; the remote budget
remained CNY 300 and the original material remained selected.

Pause and enable were recorded as `blocked_by_eligible_test_object`. The live
object was not delivering, and enable additionally lacked a successful
authoritative pause revision. No object was enabled or otherwise changed to
manufacture those preconditions.

The user authorized an ordered real batch with independent authority for every
action. Budget was first, followed by materials; pause and enable remained
ineligible. Before final confirmation for the budget action, the fenced Lease
expired. The stop policy therefore cancelled the Run, invalidated the abandoned
zero-attempt ChangeSet, and stopped every dependent action without reacquiring
the Lease or retrying. No final confirmation, `ControlledActionAttempt`, save
click, status click, or remote write occurred. The immutable Approval and
control-plane Evidence history were preserved.

The single machine-readable capability matrix, form diffs, authority outcome,
and redacted Evidence references are in
`evidence/oceanengine-controlled-actions-batch-2026-08-14.json`. Real existing-
promotion modification remains forbidden until a fresh current-turn authority
successfully passes every pre-click check.
