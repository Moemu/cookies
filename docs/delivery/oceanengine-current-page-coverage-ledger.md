# OceanEngine current-page read-only coverage ledger

## Scope

This ledger calibrates the current visible OceanEngine page only.

It does not claim full platform coverage. It does not navigate to another page.

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
| Project create | `not_accessible` | `verification_pending` |
| Project edit | `confirmed_shell` | `stable` |
| Promotion list | `confirmed_shell` | `stable` |
| Promotion create | `not_accessible` | `verification_pending` |
| Promotion edit | `confirmed_shell` | `stable` |
| Material overview | `confirmed_shell` | `stable` |
| Report overview | `confirmed_shell` | `stable` |

`not_accessible` means the target was not the current page. It does not mean the platform lacks the page.

## Real current-page evidence

The confirmed pages are both lists, both edit pages, the report overview, and the material overview.

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

## Commands

```powershell
npm run browser-rpa:calibrate -- init
npm run browser-rpa:coverage -- observe before
npm run browser-rpa:coverage -- observe after
npm run browser-rpa:coverage -- finalize
npm run test:browser-rpa:coverage
npm run calibration:check
```
