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
