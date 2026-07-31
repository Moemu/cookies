# Strategy migrations

Owner: Strategy team.

Forward-only migrations create the complete MVP aggregate storage. Every
tenant-scoped lookup is keyed by organization and project, mutable documents
are paired with append-only revisions, and approval writes the immutable
package version and Platform Outbox event in one transaction.

`20260731100000_strategy_task_overlay_v2.up.sql` binds new task plans to an
approved package, formal Handoff, and stable Route, then stores the task
strategy overlay materialized in the same transaction as its v2 strategy
version.
