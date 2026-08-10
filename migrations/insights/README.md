# Insights migrations

Owner: Insights team.

`insight_miyun_*` tables own the project-scoped Miyun connection, controlled
product query, crawl job, material projection, append-only data-card snapshot,
and handoff foundations. They do not store plaintext sessions or media blobs.
