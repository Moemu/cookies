# Platform migrations

Reserved for platform-owned persistence such as workflow Jobs, audit, and the
transactional Outbox. No vertical-system tables belong here.

`20260723157000_strategy_generation_metadata.up.sql` records the model route,
response mode, Prompt/Skill versions, latency, validation attempts, and quality
report used by Strategy generation.

`20260723158000_strategy_generation_hashes.up.sql` records immutable Strategy
Skill content hashes, the Generation Context hash, and the generated output hash.
