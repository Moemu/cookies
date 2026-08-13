# `oceanengine-ecommerce-manual` calibration baseline

`oceanengine-ecommerce-manual` is the implementation ID for the
“巨量引擎·电商手动投放” capability. Version `v0.1-calibration` promotes the
stage B read-only walkthrough into a versioned business and page-semantics
baseline. It does not claim an executable Browser Driver.

The canonical machine-readable definition is
`internal/systems/delivery/platformskills/definitions/oceanengine-ecommerce-manual-v0.1.json`.
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
- the invariant that save, create, submit, enable, remote modification, upload,
  and authorization remain forbidden.

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
- PR #50 has no real Browser Driver;
- save, submit, server validation, write idempotency, object-ID readback, and
  post-write status remain `write_validation_pending`;
- gate two remains disabled.

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
Submission capabilities remain disabled.

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

Agent final-submit behavior must not be added to a `SKILL.md` until the complete
project → promotion flow has been validated. Even after gate one passes, save,
final submit, enable, and all other gate-two actions remain disabled until a
separate explicit gate-two authorization and implementation review.

The non-write gate-two preparation baseline is frozen in
`oceanengine-gate-two-preparation.md` and
`fixtures/oceanengine-gate-two-preflight-v0.1.json`. It records the reusable
authority and recovery primitives, the fresh objects required at execution
time, the one-click limit, double-readback Mapping rule, and the deferred
post-validation Skill documentation. Its status is `ports_ready_authorization_required`;
it does not issue a confirmation or enable submit.
