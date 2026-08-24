# Connector migrations

Owner: Connector team.

Organization-owned advertising accounts use
`connector_ocean_engine_account_sessions`. This table removes the need for a
business Project before read-only synchronization. Existing Project-scoped
connections remain valid during the compatibility period.
