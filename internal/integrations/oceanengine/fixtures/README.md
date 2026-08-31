# Ocean Engine replay fixtures

These fixtures are synthetic, redacted payloads for deterministic adapter and normalization tests. They contain no Cookie, CSRF token, advertiser ID, project name, promotion name, signed URL, or unredacted platform response.

`web-api-contract-v1.json` records the observed Secsdk and endpoint contract.
Its `request_captured_response_pending` state keeps production writes blocked.
The captured browser writes included `_signature` and `x-sessionid`. Their
presence does not prove that the server requires them. Existing Connector reads
omit both fields. Test omission before adding either field to the write Client.
`web-api-request-shapes-v1.json` stores names and types only. The raw HAR and
all scalar request values stay outside Git.

Fixture rules:

- Platform identifiers are intentionally opaque strings, including values larger than JavaScript's safe integer range.
- Empty reports prove schema-only synchronization and must not become all-zero metric facts.
- `quality` is explicit; unknown mappings are represented as `mapping_incomplete`.
- Fixtures are not evidence from a real account and must never be presented as live platform data.
