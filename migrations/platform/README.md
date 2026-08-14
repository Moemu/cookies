# Platform migrations

Reserved for platform-owned persistence such as workflow Jobs, audit, and the
transactional Outbox. No vertical-system tables belong here.

`20260723157000_strategy_generation_metadata.up.sql` records the model route,
response mode, Prompt/Skill versions, latency, validation attempts, and quality
report used by Strategy generation.

`20260723158000_strategy_generation_hashes.up.sql` records immutable Strategy
Skill content hashes, the Generation Context hash, and the generated output hash.

## Controlled Computer Use

`20260812130000_platform_computer_use_control_plane.up.sql` adds the Platform-
owned browser execution control plane. It persists tenant-scoped environments,
profiles, allowlists, exclusive session leases with fencing tokens, run state,
append-only events and evidence, kill switches, short-lived one-time final
confirmations, and controlled action attempts. Confirmation consumption and
attempt creation occur in one transaction; token material is never stored,
only its SHA-256 digest. The migration does not enable a remote write worker or
change an existing Delivery contract.
