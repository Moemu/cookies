# Insights migrations

Owner: Insights team.

`insight_miyun_*` tables own the project-scoped Miyun connection, controlled
product query, crawl job, material projection, append-only data-card snapshot,
and handoff foundations. They do not store plaintext sessions or media blobs.

`20260810102000_insight_miyun_product_analysis_intake.up.sql` adds deterministic
product-profile input lineage and field sources. It also permits manual material
intake without a crawl job, keyed idempotently per Project, while retaining the
existing AssetVersion and Insight Asset foreign keys and append-only snapshots.

`20260810103000_insight_miyun_crawl_authorized_import.up.sql` adds crawl
idempotency/runtime lineage, encrypted resource locators, operator decisions,
import errors, and replay-safe page snapshots.
