# Data Connector platform module

Reserved for authorized advertising-data synchronization, source mapping,
metric dictionaries, freshness, and reconciliation. It does not own Delivery
plans or Insight conclusions.

The package owns `connector-oceanengine-point-in-time-v1`. Raw evidence stays
inside the Connector boundary. Delivery and Insights consume only canonical,
permission-trimmed snapshots with a required `prediction_cutoff`.

Valid time states when a platform fact applies. Knowledge time is
`available_at`, which is the first time cookies observed the fact. A platform
`modify_time` never replaces `available_at`.

New runtime routes own accounts at Organization scope. They use local Connector
account IDs and do not require a business Project or Plan. Project routes remain
only for existing connections. The raw platform account ID stays in the
Connector account catalog and never appears in Canonical output.
Metric windows use `quarantine` until the platform definition and attribution
maturity are formally confirmed.

Organization-scoped account sessions use
`connector_ocean_engine_account_sessions`. The table stores encrypted values
only. Canonical facts use an empty legacy `project_id` column and the account
`source_ref`; the system does not create a placeholder Project.

`cookies-delivery-calibration audit` reports anonymous coverage and quality
counts after a read-only sync. `cookies-delivery-calibration export` reads two
knowledge-time snapshots and creates Plan-independent `CalibrationCase` values.
The command accepts only a local `oeacct_` account ID. Its HMAC key comes only
from `COOKIES_CALIBRATION_EXPORT_KEY_BASE64`.

`cookies-delivery-calibration backtest` builds a retrospective-only rolling
origin dataset from anonymous daily metric windows. It excludes current
configuration, platform diagnosis, and immature conversion labels. It uses a
time holdout and never applies a candidate calibration to the simulator.
The hierarchical hurdle challenger models activity and positive magnitude
separately. Relative improvement alone cannot pass its absolute readiness gate.
