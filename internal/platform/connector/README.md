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
The lifecycle hurdle challenger adds cutoff-safe age, activity recency, streak,
and recent-trend segments. Sparse segments fall back to anonymous Project and
account priors. The cohort continuation challenger uses the latest cutoff-safe
value and a robust next-day ratio. It selects its settings with training-only
nested time validation. It evaluates overall, cold-start, and warm-start
holdouts separately. A Promotion does not need 90 to 180 days of history.
A passing report still requires a separate simulator change.
Backtest diagnostics compare training and holdout coverage, activity, positive
magnitude, and unseen Promotion share. Unseen share describes the cold-start
workload. It is not a drift failure. The prediction path cannot consume these
diagnostics.

`cookies-delivery-calibration backtest-xlsx` validates four offline platform
exports without a running API or database. Account, Project, and Promotion
daily atomic totals must reconcile exactly. Only Promotion daily metrics enter
the retrospective model. Material aggregate metrics stay excluded because they
lack a daily Promotion binding and can count one delivery more than once.
After reconciliation, an absent Promotion row becomes zero only on a date that
exists in the account export and after the Promotion first appears. This rule
does not require a platform status field.

`cookies-delivery-calibration import-xlsx` writes the validated bundle to the
immutable ledger when MySQL is available. It verifies the source account
against the registered local account and encrypts the original workbooks.
Canonical facts never contain names, material text, URLs, or access parameters.
Retrospective backtests retain metric-only Promotions after they leave the
current inventory. Missing Project lineage stays empty. The backtest does not
create or infer a Project, and these cases use only account-level priors.
Each hurdle model requires at least ten positive training cases per eligible
metric. A larger all-zero training set cannot satisfy this readiness boundary.

`cookies-delivery-calibration backtest-launch-batches` joins two complementary
custom reports to the read-only object index. It aggregates each report at
day, Project, and Promotion grain before the join. It uses the platform create
time. It does not use the first metric day as a launch date.

The launch-batch model treats fixed optimization and charging settings as an
account/product prior. It groups Promotions by the platform Project and the
Project launch day. It predicts a typical scenario and a heavy-tail breakout
scenario. It does not claim that it can identify the winning Promotion before
platform learning. `ready_for_probabilistic_shadow` permits shadow simulation
only. Point estimates remain diagnostic and cannot drive an optimization.
Use `--persist` only after the result reaches that status. Persistence stores a
compact prior and source hashes. It never stores raw report rows or external
platform IDs. Delivery reads the latest prior through a narrow read-only port.
