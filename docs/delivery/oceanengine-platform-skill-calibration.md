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

The definition status is `realtime_dom_validation_required`:

- stage B business semantics are `observed`/`operator_reviewed` where recorded;
- its observation date is 2026-08-06 and the platform build is unknown;
- selector fixtures describe business semantics, not executable DOM/CSS
  locators;
- PR #50 has no real Browser Driver;
- save, submit, server validation, write idempotency, object-ID readback, and
  post-write status remain `write_validation_pending`;
- gate two remains disabled.

The calibration definition is bound into controlled ChangeSets and formal
Approvals by ID and version. That binding proves which stage B baseline was
approved; it does not turn the baseline into an executable transport.

## Gate one: real-time revalidation

Gate one may begin only after the same turn supplies the exact test account
reference, allowed platform project IDs, the action “fill unsubmitted form”,
and a CNY budget ceiling. It must use a visible, already-authenticated browser
and stop before every remote-write-adjacent action.

Acceptance requires all of the following:

1. Reidentify the current account, page kind, and allowlisted project.
2. Confirm whether the UI has drifted since 2026-08-06.
3. Establish stable live DOM locators without coordinate fallbacks.
4. Fill only the approved unsubmitted project/promotion fields.
5. Read every field back and compare it with the approved configuration.
6. Discard the local form/draft using the observed safe-exit path.
7. Return to a known read-only page and prove that no platform object or status
   change was created.

Passing gate one calibrates the real Browser Driver for this observed UI
version. It still does not authorize save, final submit, enable, or any other
gate-two action.
