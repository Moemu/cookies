# Strategy migrations

Owner: Strategy team.

Forward-only migrations create the complete MVP aggregate storage. Every
tenant-scoped lookup is keyed by organization and project, mutable documents
are paired with append-only revisions, and approval writes the immutable
package version and Platform Outbox event in one transaction.
