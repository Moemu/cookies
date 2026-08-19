# Browser RPA migration amendment

| Property | Content |
| --- | --- |
| Status | Supersedes the execution-transport statements of the frozen Computer Use contract; all authority semantics unchanged |
| Applies to | `docs/delivery/controlled-computer-use-contract.md`, `docs/12-browser-rpa-runtime.md`, `docs/04-intelligent-delivery-prd.md` |
| Decision | The intelligent-delivery execution layer migrates from vision-model Computer Use to deterministic Playwright RPA over an externally authenticated Edge session |
| Date | 2026-08-19 |

## 1. Why

Field verification showed Playwright-based RPA with semantic locators is
materially more reliable than vision-model Computer Use for the calibrated
OceanEngine surfaces: locators are deterministic, replayable, and already
frozen in the calibration manifest (`playwright_rpa` records), while Computer
Use never shipped a real unattended driver (contract decisions #10, #16, #18
document that the production mount remained takeover-only).

## 2. The frozen contract stays in force

All 23 decisions of `controlled-computer-use-contract.md` continue to apply
unchanged. They govern authority, approvals, leases, confirmations, kill
switches, evidence semantics and result classification — none of them depend
on which transport performs the page actions. Specifically:

- Decision #1 (run-time write authority), #8 (final confirmation), #9
  (leases), #11 (kill switch), #12 (site allowlist), #15 (result semantics)
  and #16 (worker/takeover ports) bind the new Playwright adapter exactly as
  they bound the deterministic fake worker.
- The adapter may only fill values already bound into the approved ChangeSet
  and may perform at most one final click per authorized attempt, never
  retried.

Only the transport-status statements are superseded:

| Decision | Superseded statement | Replacement |
| --- | --- | --- |
| #10 | “No unattended real driver exists” | A real Playwright RPA driver exists behind `COOKIES_BROWSER_RPA_ENABLED`; it executes only calibrated `playwright_rpa` manifest records and remains disabled by default. |
| #16 | “Fake and future real workers use the same ports” | The real worker is `rparunner.PlaywrightRPAAdapter`; it uses the same service ports unchanged. |
| #17 | `real_browser_driver=false` skill status | The skill's execution driver is `playwright-rpa/edge/v1`; write gates (approval + confirmation + lease + kill switch) are unchanged. |
| #18 | “The production mount has no … unattended page driver” | The production mount gains a config-gated automated worker; the default (flag off) remains takeover-only. |

## 3. Naming migration

Historical payloads are immutable. Row bytes, canonical hashes, and embedded
`schema_version` strings keep their `computer-use-*` values forever; new rows
write `browser-rpa-*` values and validators accept both generations.

| Old | New |
| --- | --- |
| Go package `internal/platform/computeruse` | `internal/platform/browserautomation` |
| API prefix `/api/platform/v1/computer-use/**` | `/api/platform/v1/browser-rpa/**` (old prefix kept one release as a transitional alias) |
| Tables `computer_use_*` (12 tables) | `browser_rpa_*` via metadata-only `RENAME TABLE` (migration `20260819120000_platform_browser_rpa_rename`) |
| Columns `computer_use_run_id` (4 delivery tables) | `browser_rpa_run_id` via `CHANGE COLUMN` (migration `20260819121000_delivery_browser_rpa_run_reference_rename`) |
| `schema_version`: `computer-use-{run,authority,evidence,final-confirmation}/v1` | `browser-rpa-{run,authority,evidence,final-confirmation}/v1` for new rows; legacy values remain valid |
| Redaction version | `browser-rpa-redaction/v1` for new evidence |
| Selector/action provenance | `playwright-rpa-locator/v1`, `playwright-rpa-action/v1` |
| Scope `platform.computer-use.admin` | `platform.browser-rpa.admin` |
| Kill switch IDs `computer-use-kill-*` | `browser-rpa-kill-*` for new rows; lookup is scope-keyed, so legacy rows keep functioning |
| JSON field `computer_use_run_id` | `browser_rpa_run_id` |

Unchanged on purpose: historical migration files, the frozen contract
document, `docs/delivery/evidence/*`, calibration fixtures, and the insights
ingest-mode enum value `computer_use` (a separate frozen concept meaning
“page read-back”).

## 4. Integrity verification

Both rename migrations are metadata-only. Verified against the local
development database (populated with historical rows) on 2026-08-19:

```
CHECKSUM TABLE before rename (computer_use_*) == CHECKSUM TABLE after rename (browser_rpa_*)
  computer_use_runs                        1661480907  ->  browser_rpa_runs                        1661480907
  computer_use_evidence                    1769126427  ->  browser_rpa_evidence                    1769126427
  computer_use_controlled_action_attempts   386337940  ->  browser_rpa_controlled_action_attempts   386337940
  computer_use_final_confirmations         4291378507  ->  browser_rpa_final_confirmations         4291378507
  delivery_controlled_executions           1397992293  ->  delivery_controlled_executions           1397992293
  delivery_platform_entity_mappings        2696923920  ->  delivery_platform_entity_mappings        2696923920
  delivery_platform_entity_mapping_revisions 2636237221 -> delivery_platform_entity_mapping_revisions 2636237221
```

Guard tests enforce this permanently:
`internal/platform/browserautomation/migration_test.go` fails if either
rename migration contains `UPDATE`, `DELETE`, or `INSERT`, or omits any of
the twelve table renames / four column renames.

## 5. Execution architecture

```
Delivery ChangeSet/Approval/Execution ──> BrowserRpaRun (control plane)
                                              │  prepare / submit (config-gated)
                                              ▼
                            rparunner.PlaywrightRPAAdapter
                               │ compile plan (plancompile + frozen manifest)
                               │ spawn `npx tsx scripts/browser-rpa-runner.ts`
                               │ plan JSON on stdin, result JSON on stdout
                               │ lease heartbeat every 30s (TTL 1m)
                               ▼
                          Playwright connectOverCDP ──> operator's logged-in Edge
```

- CDP endpoint source: `browser_rpa_environments.cdp_endpoint`, with a
  non-production-only `COOKIES_BROWSER_RPA_CDP_ENDPOINT` fallback.
- Login, captchas and 2FA remain human-only (contract §4 of the runtime
  doc); the runner attaches to an already authenticated session and never
  records credentials.
- First enabled action: read-only `update_promotion_budget` prepare
  (advances to `awaiting_confirmation` and stops). The real budget click
  (`button:确定修改`) remains a separately authorized, separately calibrated
  turn per contract decision #19/#20.

## 6. Open items

- First authorized real submit for `update_promotion_budget`, pending an
  eligible test object above the 300 CNY calibration ceiling
  (`distinct_authorized_target_exists: false` in the frozen manifest).
- Pause/enable/materials/create actions gain RPA plans only after their own
  calibration records land in the manifest.
- Removal of the legacy `/api/platform/v1/computer-use/**` alias after one
  release.
