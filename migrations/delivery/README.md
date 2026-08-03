# Delivery migrations

Owner: Delivery team.

`20260731120000_delivery_approval_content_hashes.up.sql` intentionally adds
`delivery_plan_versions.canonical_hash` as nullable for the SQL phase. The
`cookies-migrate` command immediately runs
`delivery.BackfillPlanCanonicalHashes`, which recalculates every existing
version with the same RFC 8785 JCS + SHA-256 Go canonicalizer used by new
writes, fills only missing hashes, rejects any non-empty mismatched hash
instead of blessing changed immutable content, verifies that no hashes are
missing, and then makes the column `NOT NULL`.

The command then runs `delivery.BackfillLegacyApprovals`. Any pre-A03
ChangeSet with the compatibility `approved_by`/`approved_at` projection is
converted once into an immutable `delivery_approvals` authority record,
including the original approval time, the fixed 24-hour expiry, the inferred
approval-time ChangeSetVersion, content/action hashes, `execute_mock` scope,
budget snapshot, and mock provenance.

Apply migrations through `go run ./cmd/cookies-migrate`; do not fill plan
hashes with MySQL JSON/hash functions or a second canonicalization
implementation.
