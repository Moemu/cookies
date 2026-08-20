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

Runtime routes use local Connector account IDs. The raw platform account ID
stays in the Connector account catalog and never appears in Canonical output.
Metric windows use `quarantine` until the platform definition and attribution
maturity are formally confirmed.
