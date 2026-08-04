# Creative migrations

- `20260804130000_creative_ai_native_storyboards` adds recoverable storyboard planning, immutable storyboard revisions, and confirmed-storyboard lineage for AI native performance ads.

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

`20260803150000_creative_ai_native_requirement_workspaces.up.sql` stores the
AI-native-ad requirement workspace and every immutable RequirementRevision.
The workspace is linked to one manual CreativeIntake and one video CreativeTask
with `performance_mode=ai_ad_generation`. It owns the aggregate stage and
optimistic `workspace_version`; edits append JSON revisions, confirmation
freezes the current pointer, and reopening supersedes rather than overwrites the
confirmed revision.

`20260804110000_creative_ai_native_scripts.up.sql` extends that aggregate with
durable script generation state and immutable `AdScriptRevision` records. Each
script records its confirmed Requirement revision/hash, channel profile/hash,
model route metadata and regeneration lineage; edits and reopen operations
append revisions instead of overwriting prior content.

`20260804130000_creative_ai_native_storyboards.up.sql` adds recoverable
storyboard planning and immutable storyboard revisions. Product identity keeps
an Assets-owned reference and cannot be replaced by generated media.

`20260804150000_creative_ai_native_production.up.sql` adds the durable AI ad
ProductionPlan, GenerationUnit/Attempt state, active operation, server progress
source and terminal assets-ready/failed/cancelled states. Successful Provider
assets remain referenced across Worker restarts and local Unit retries.

`20260804170000_creative_ai_native_final_render.up.sql` extends Production with
rendering/completed/render-failed states. The frozen Timeline consumes only
Assets-owned video and audio versions; its final H.264/AAC MP4 is ingested with
a stable render identity so a Worker retry cannot create duplicate outputs.
