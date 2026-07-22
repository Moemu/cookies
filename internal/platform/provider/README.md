# Provider Gateway

Owner: Platform team.

This module exposes capability-based model calls and `contract.Job` resources.
It may know provider adapters, logical model aliases, retry policy, quotas,
usage, and cost. It must not create or update Assets records directly.

When a generation job completes, the caller passes the verified output to the
Assets module's generated-asset intake API. That API is the only route that
returns a project-library `contract.ProjectAssetRef` that points to an
immutable `contract.AssetVersionRef`.

Synchronous text and VLM calls use Provider-owned typed application requests.
VLM accepts only project-scoped `ProjectAssetRef` values; a real adapter gets
media bytes through an injected, Assets-authorized `VisionSourceResolver`, not
through an Assets table, storage URL, or vendor credential leaked across the
module boundary.
