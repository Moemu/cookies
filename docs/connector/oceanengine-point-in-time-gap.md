# Ocean Engine point-in-time ledger P0 gap table

`data/deep-research-report.md` is design input. It is not formal evidence.

| Target fact | Current source endpoint | Collection grain | Time semantics | Idempotency key | Quality rule | Consumer |
| --- | --- | --- | --- | --- | --- | --- |
| Account | `GET /ad/api/account/info` | account per sync | valid time from platform when present; knowledge time is collection time | account + payload hash + collected run | reject missing lineage | Connector verification |
| Project and Promotion | `POST /ad/api/promotion/ads/list` | object per page | platform create/modify time is valid time; collection time is knowledge time | kind + opaque object ref + valid from + payload hash | quarantine incomplete page | Delivery, Insights |
| Promotion configuration | `GET /ad/api/promotion/ads/get_promotion_detail` | Promotion per observation | configuration effective time is valid time; collection time is knowledge time | object ref + valid from + payload hash | quarantine unresolved change window | Delivery, Insights |
| Material binding | `GET /superior/api/ad/promotion/detail` | material to Promotion interval | binding interval is valid time; first observation is knowledge time | material ref + Promotion ref + valid from | quarantine missing parent | Insights |
| Platform diagnosis | `POST /ad/api/promotion/ads/attribute/list` | Promotion per observation | diagnosis observation time only | object ref + observed time + payload hash | prelaunch eligibility must be false | Insights weak label only |
| Metric window | read-only `statQuery` POST endpoints | object and metric window | window is valid time; collection time is knowledge time | object + window + attribution + definition + payload hash | reject invalid window, unit, or definition | Delivery, Insights |
| Metric revision | repeated `statQuery` for the same window | revision per changed payload | source window remains fixed; revision collection time advances | metric identity + payload hash; `revision_of` is required | quarantine immature attribution | Insights training export |
| Status event | list, detail, and attribute payload state | object state transition | observed platform state is valid time; collection time is knowledge time | object + state + observed time | warning for unexpected regression | Delivery, Insights |

## Current read-only boundary

The protocol client permits only allow-listed GET requests and proven read-only POST queries.
It supports account verification, Promotion listing, configuration, material, diagnosis, and metrics.
Unknown paths and write methods return `ErrForbiddenEndpoint`.

## Runtime status and remaining evidence gaps

- The shared ledger has memory and MySQL implementations.
- The authenticated runtime exposes account registration, verification, sync, sync status, and point-in-time reads.
- Synchronization uses only allow-listed read-only GET and query POST operations.
- An authorized local read-only audit completed on 2026-08-21. Credentials and raw account identity are not in this repository.
- `statQuery` leaf rows can include `Rows: null`. The normalizer treats these objects as metric leaves when `Metrics` exists.
- `ad_material_data.material_id` does not share one proven namespace with Promotion detail material IDs. The Connector keeps these metric rows as quarantined facts with `material_binding_unresolved`; it does not guess a binding.
- Platform metric definitions and attribution maturity require formal confirmation. Until then, collected metric windows stay quarantined.
