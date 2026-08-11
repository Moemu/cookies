# Delivery domain context

Delivery owns the controlled transition from an immutable CreativePackage to an auditable advertising-platform change.

- **DeliveryPlan**: the approved intent, budget window, and immutable CreativePackage reference.
- **ChangeSet**: one versioned proposal to change a platform. It is the only object that can be preflighted, approved, executed, or rolled back.
- **Approval**: an immutable mock authority record binding one PlanVersion canonical hash and one ChangeSetVersion/action hash to an approver, 24-hour expiry, `execute_mock` scope, and budget snapshot. `delivery_change_sets.approved_by/approved_at` is only its compatibility projection. Execute and rollback advance lifecycle versions without changing the approved content version; those transitions must not be reported as approval content mismatches.
- **Execution**: one attempt to apply an approved ChangeSet.
- **Evidence**: the durable proof of what an Execution did. MVP evidence explicitly records local simulation and must never imply a real platform write.

Delivery does not own Creative content, Provider jobs, project identity, post-launch analysis, or reusable experience rules.

## Read-only insights consumer port

Delivery consumes read-only object and metric facts through its own
`InsightsConsumer` port. The port returns versioned account → project →
unit (platform promotion) → material mappings plus project-scoped `spend`, `impressions`,
`clicks`, and `conversions` facts. Each fact carries its window, granularity,
unit/currency, schema and definition versions, source (`mock`/`replay` or a
future `connector`), freshness, quality, and evidence references.

Until the Insights Connector publishes its stable contract, the API uses the
versioned deterministic mock fixtures embedded in Delivery. Replay keeps the
same fixture identities and scope. The current simulation adapter normalizes
stored OutcomeSimulation windows into the same metric-fact DTO; `EvaluateAlerts`
then calculates from those facts rather than reading Repository metrics.
An optional `execution_id` on the query pins the simulation adapter to that
execution; it filters before selecting a seed so projects with multiple
executions never mix windows.
Only `usable` data can produce a deterministic alert. `empty`, `stale`, `incomplete`, `schema_mismatch`, and
`unavailable` data return an explicit quality/provenance result with no alert,
and never fabricate a zero metric.
