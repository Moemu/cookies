# OceanEngine current-page read-only coverage ledger

## Scope

This ledger combines controlled observations from visible OceanEngine pages in
the current test account.

It does not claim full platform coverage.

The checked-in ledgers are:

- `api/fixtures/oceanengine-readonly-coverage-ledger-v1.json` — frozen historical contract.
- `api/fixtures/oceanengine-readonly-coverage-ledger-v2.json` — current contract with separate create/edit targets.

## Coverage inventory

The checklist comes from these version-controlled sources:

- The OceanEngine calibration manifest: 6 page families and 51 fields.
- Four live locator fixtures.
- The Browser RPA safety specification.

All source fixtures disable coordinate fallback. The manifest disables remote write.

The ledger attempts these targets:

| Target | Current status | Drift state |
| --- | --- | --- |
| Account context | `confirmed_shell` | `stable` |
| Project list | `confirmed_shell` | `stable` |
| Project create | `confirmed_fields` | `stable` |
| Project edit | `confirmed_fields` | `stable` |
| Promotion list | `confirmed_shell` | `stable` |
| Promotion create | `confirmed_fields` | `stable` |
| Promotion edit | `confirmed_fields` | `stable` |
| Material overview | `confirmed_shell` | `stable` |
| Report overview | `confirmed_shell` | `stable` |

## Real current-page evidence

The confirmed pages include both lists, both create pages, both edit pages, the
report overview, and the material overview.

The page passed these checks:

- HTTPS and exact allowed host.
- Page type fingerprint.
- Account-context Hash match.
- Required table or form structure.
- Unique and visible semantic locators.

The real locator candidates are controlled keys only:

- `create_project`.
- `project_budget_column`.
- `create_promotion`.
- `promotion_budget_column`.
- `promotion_name`.
- `save_and_close`.
- `project_marketing_purpose`.
- `project_name`.
- `project_deep_optimization`.
- `data_center`.
- `project_report`.
- `promotion_report`.
- `material_report`.
- `material_overview`.

The ledger does not store their visible text.

## No-write proof

Two separate Playwright attachments observed each confirmed page target.

The two list-page summaries contained 10 observable object rows each; the four form, material, and report pages contained 0 rows.

The evidence recorded:

- New object count: 0.
- Observable state change count: 0.
- Draft residual count: 0.
- Draft residual delta: 0.
- Write actions executed: 0.
- Mutation API surface: absent.

The cumulative proof covers 6 pages and 20 observable rows.

The promotion edit page did not render the earlier budget selector.
The current page still had one promotion-name control and one save boundary.
The URL had an edit marker. The ledger does not store its value.

The project-name text appeared three times and failed uniqueness.
The stable project-name attribute matched one visible control.
The tool uses the stable attribute for the confirmed locator.

The report page had no ARIA table and showed four empty markers.
The structure check accepts this explicit empty report state.

Generic button and input counts verify page structure only.
They do not enter the business-state hash.
This prevents page-shell loading from causing a false state change.

The requested material-relation target had no matching product page.
The current product label is `material_overview`.
The tool uses this real label and does not keep the unverified label.

The proof covers current-page observable counts and controlled status markers only.

## Local raw artifacts

The commands write local observations and diagnostics below:

`%LOCALAPPDATA%\cookies\browser-rpa\calibration\`

Current runs retain their before/after preimages under `runs/<run_id>/coverage-{before,after}.json`, with `active-run.json` pointing to the in-progress run. Finalization removes the pointer. Finalization still reads the legacy root-level `coverage-before.json` and `coverage-after.json` layout for historical runs.

The v2 ledger separates project/unit create and edit targets and distinguishes `confirmed_shell` from the future field-level `confirmed_fields` status. v1 and its fixture remain frozen history.

## Field-level capture

`browser-rpa:fields` runs read-only per-case field observations against the current Edge page. It consumes the 51-field calibration manifest and records, per coverage case, one observation envelope under:

`%LOCALAPPDATA%\cookies\browser-rpa\calibration\field-observations\<case-id>-<timestamp>.json`

Each envelope stores hashes and states only — no raw field values, account identifiers, or URL queries. Field-level evidence that covers every declared field of a page family is what upgrades a v2 target from `confirmed_shell` to `confirmed_fields`.

```powershell
npm run browser-rpa:fields -- init
npm run browser-rpa:fields -- run CASE_ID [PLAN_PATH]
npm run test:browser-rpa:fields
```

For multiple cases, start one persistent CDP session:

```powershell
npm run browser-rpa:fields -- session
```

This command creates the default plan through the same CDP connection. Do not
run `init` first. Wait for the `ready` result. Then enter one case ID per line.
The process keeps one Browser connection for the full stdin session. You can
manually change the visible OceanEngine page before you enter the next case ID.

Use a JSON line when the case needs declared form state:

```json
{"case_id":"OE-PROJECT-E-COMMERCE-MANUAL","declare":["carrier=orange_landing_page"]}
```

Enter `exit` or `quit` to close the session. The command disconnects when its
process exits. It does not close Edge.

Do not copy raw DOM, screenshots, URL queries, or browser data into Git.

## Session reuse verification

The final real attach check succeeded on the current User Profile.

Both attachments returned the same Browser Context and page Target.
Both attachments read the current signed-in material overview page.

Two earlier checks failed before the successful check:

- The first check found no page before the current page became available.
- The second check reached the old 20-second process timeout.

The attach helper now waits for the allowed page across all contexts.
It accepts the default Context when CDP omits its ID.
The outer timeout is 120 seconds.

The field runner now selects the tab that matches the requested page family.
It does not assume that the last OceanEngine tab is the requested tab.

The 2026-08-24 live promotion-edit replay used a self-created unit. The edit
case reused all 19 promotion-create fields and added the edit save boundary.
All 20 fields returned `observed`. The case outcome was `success`. The runner
did not click or save the edit form.

The promotion-create replay used the same self-created parent project. All 19
fields returned `observed`. The case outcome was `success`. The operator then
cancelled the local draft. The runner did not create another unit.

The project-edit replay used the self-created ecommerce project. The case now
contains the complete visible ecommerce-manual edit surface. All 17 fields
returned `observed`. Product, optimization-target, and start-date controls were
read only. The operator cancelled the page and did not save a change.

The ecommerce project-create replay used one managed product reference. The
automatic case returned 12 of 12 observed fields. The manual case returned 6
of 6 observed fields. The operator cancelled the draft. No second project was
created.

The sales-leads project-create replay covered both delivery modes. The smart
case returned 13 of 13 observed fields. The custom case returned 3 of 3
observed fields. The operator cancelled the draft. No sales-leads project was
created.

The product-catalog project-create replay returned 11 of 11 observed fields.
Stable `data-auto-id` locators replaced the repeated product-catalog and
product-targeting titles. The operator cancelled the draft. No product-catalog
project was created.

The short-video content-marketing replay returned 11 of 11 observed fields.
The runner selected visible branch-specific locator alternatives for carrier
and optimization target. The live option remained unavailable. The operator
recorded the live case as blocked, then cancelled the draft. No
content-marketing project was created.

The application replay covered download, launch, and appointment paths. The
iOS and Harmony package-download case returned 11 observed fields and one
condition-blocked bid field. All four launch cases returned 5 of 5 observed
fields. The Android and iOS appointment case returned 4 of 4 observed fields.
Android download remained blocked by its event-asset dependency. Harmony
appointment remained unavailable. The operator cancelled the draft. No
application project was created.

The operator also recorded blocked evidence for ecommerce live delivery and
the unopened other-marketing-purpose branch.

The replay replaced ambiguous visible-text locators with stable `data-e2e` or
`data-auto-id` locators. It also confirmed these branch rules:

- Ecommerce projects expose the marketing-product field.
- Ecommerce manual delivery exposes placement and search-boost fields.
- UBMax exposes AIGC dynamic creative.
- The current ecommerce manual project page does not expose a project bid.
- Sales-leads smart delivery exposes the bid and daily-budget fields.
- Sales-leads custom delivery changes the carrier selector surface.
- Product-catalog delivery exposes catalog, placement, and product targeting.
- Content marketing uses branch-specific carrier and optimization controls.
- The account still shows the live content-marketing branch as unavailable.
- Application download and appointment keep hidden inactive component trees.
- Visible locator alternatives isolate the active application controls.
- Harmony launch is available, but Harmony appointment is unavailable.

The latest-evidence inventory has 20 successful cases and 6 blocked cases.
It has no partial or missing case.

The 2026-08-24 D3 replay used only the self-created project and unit. The
operator enabled and paused the project. The operator paused and enabled the
unit. Each write had a readback. The final project state is paused. The final
unit state is enabled.

The material replay selected a different existing video. It saved and read the
temporary value. It then restored and read the original video. The replay did
not upload, modify, or remove a managed asset.

The schedule stayed on 2026-08-25. The project and unit budgets stayed at CNY
300. The unit bid stayed at CNY 0.01. Account spend stayed at zero.

## Commands

```powershell
npm run browser-rpa:calibrate -- init
npm run browser-rpa:coverage -- observe before
npm run browser-rpa:coverage -- observe after
npm run browser-rpa:coverage -- finalize
npm run test:browser-rpa:coverage
npm run calibration:check
```
