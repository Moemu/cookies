# Platform module rules

`internal/platform` contains reusable, business-agnostic capabilities. The
vertical systems own their own state machines and data:

- Strategy owns conversations, briefs, research, and strategies.
- Creative owns creative tasks, versions, and packages.
- Insights owns asset features, analyses, insights, and experiences.
- Delivery owns delivery plans, change sets, platform entities, and evidence.

Platform code must not introduce tables, routes, events, or state transitions
for those entities. Cross-system collaboration uses stable IDs, authorized APIs,
and versioned event envelopes.

## Integration invariants

1. Every protected operation has a validated organization and user/service
   principal. Project scope is optional only for organization-level operations.
2. A caller never supplies its own authoritative organization through a request
   header or JSON field. An identity adapter resolves it from a trusted source.
3. Platform public routes live under `/platform/v1/*`; vertical routes live
   under `/api/{system}/v1/*`.
4. Event types are versioned and immutable after publication. Event payloads
   carry only routing data and necessary snapshots, never large or sensitive
   content.
5. `projectcontext.Reference` is a versioned, minimal projection. Downstream
   systems retain its stable IDs and version, not mutable brand or product facts.
6. `contract.Job` and `contract.AssetRef` are shared transport contracts.
   Provider returns completed output; Assets alone admits it into a project
   library and returns the durable AssetRef.
7. Every future mutable write requires an idempotency key and expected resource
   version. Immutable versions are never updated in place.
8. New persistent modules must use tenant-aware repositories, forward-compatible
   migrations, audit records, idempotency, and the transactional Outbox pattern.
9. Provider, knowledge, Agent, and Computer Use adapters hide vendor-specific
   SDKs and credentials behind platform-owned seams.

## Bootstrap limits

This initial skeleton deliberately has no database, queue, SSO, Provider, ORAG,
or Computer Use implementation. `StaticResolver` is only enabled by explicit
local environment configuration; all non-local startup paths fail closed for
protected routes until a trusted identity adapter is supplied.
