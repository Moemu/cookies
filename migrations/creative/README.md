# Creative migrations

Owner: Creative team.

`20260723150000_creative_image_text_m1.up.sql` creates Creative-owned Intakes,
Tasks, image-and-text drafts, and production-lineage records. It deliberately
does not add foreign keys to Strategy or Provider tables; those are cross-module
references resolved through their APIs.

`20260723180000_creative_versions_m1_5.up.sql` adds immutable
`CreativeVersion` snapshots. A version stores Creative's own frozen draft
payload and hash; it does not own Asset or Provider records.

`20260723181000_creative_strategy_package_idempotency.up.sql` makes one
approved Strategy package version map to one Creative Intake in a Project.
Repeated handoff delivery returns that existing Intake instead of duplicating
the Creative work queue.

`20260723181100_creative_task_directions.up.sql` removes the one-task-per-
Intake restriction. One Strategy-backed Intake may create several explicitly
named Creative directions, such as lifestyle, ingredient explanation or usage
scenario.

`20260723181200_creative_task_archive.up.sql` adds the `archived` task state.
Archiving removes a task from the active Creative queue; it is not a hard
delete, because drafts, versions, Provider jobs and Assets remain auditable.

`20260731101000_creative_strategy_intake_v3.up.sql` changes Strategy-backed
Intake identity from package-only to package + Handoff + Route + optional task
overlay. `20260731102000_creative_direction_planning.up.sql` stores immutable
LLM candidate batches and the explicitly confirmed CreativeDirection.
`20260731103000_creative_intake_identity_index.up.sql` makes the canonical
`input_identity_hash` the concurrency-safe deduplication key.
