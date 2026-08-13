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

The 2026-08-12 live walkthrough advanced the definition to
`gate_one_partial_live_calibration`:

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
- promotion/unit live locators were not revalidated because the visible
  existing sample and oldest projects were automatic delivery, while reaching
  a new manual unit required crossing `保存并新建单元`;
- the PR now exposes a fenced takeover-evidence port that atomically records
  Run version, Step, Event, and redacted Evidence for a fixed non-write action
  enum after exact site-policy validation;
- the already completed live actions predated that port and therefore were not
  recorded through a persisted ComputerUseRun chain; a controlled replay is
  still required before the complete release gate can close;
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

The project-form portion passed items 1–7. On 2026-08-13, the promotion-form
page was reidentified in the authorized test account and an allowlisted
existing test project. Before any field was changed, the live page diverged
from the stage B fixture: video/image capacities were `10/10` instead of
`30/50`, the landing-page label was `橙子落地页` instead of `自研落地页`, and
a `单元预算与出价` section was present. The replay therefore stopped with
`PAGE_DRIFT` before form fill. The redacted observation is recorded in
`evidence/oceanengine-gate-one-promotion-drift-2026-08-13.json`.

The promotion-form portion and control-plane evidence recording remain pending,
so gate one is `partial`, not passed, and the real Browser Driver is still
uncalibrated as a complete Skill.

The non-live preparation for a persisted replay is frozen in
`oceanengine-gate-one-replay-runbook.md` and
`fixtures/oceanengine-gate-one-replay-plan-v0.1.json`. Promotion locator
capture uses
`fixtures/oceanengine-promotion-live-locator-capture-v0.1-template.json`.
That file remains an empty `template_not_observed` record: it lists
the stage B fields and selector surfaces to inspect but contains no invented
DOM locator and is not evidence of calibration. The separate 2026-08-13 drift
observation contains only locators confirmed on the live page; ambiguous or
unvisited controls remain explicit gaps.

Agent final-submit behavior must not be added to a `SKILL.md` until the complete
project → promotion flow has been validated. Even after gate one passes, save,
final submit, enable, and all other gate-two actions remain disabled until a
separate explicit gate-two authorization and implementation review.
