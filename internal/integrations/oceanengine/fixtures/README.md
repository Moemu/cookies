# Ocean Engine replay fixtures

These fixtures are synthetic, redacted payloads for deterministic adapter and normalization tests. They contain no Cookie, CSRF token, advertiser ID, project name, promotion name, signed URL, or unredacted platform response.

`web-api-contract-v1.json` records the observed Secsdk and endpoint contract.
Its `static_source_only` state keeps production writes blocked. Live sanitized
request and response fixtures must replace all listed blockers before release.

Fixture rules:

- Platform identifiers are intentionally opaque strings, including values larger than JavaScript's safe integer range.
- Empty reports prove schema-only synchronization and must not become all-zero metric facts.
- `quality` is explicit; unknown mappings are represented as `mapping_incomplete`.
- Fixtures are not evidence from a real account and must never be presented as live platform data.
