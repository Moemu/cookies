# Provider Gateway

Owner: Platform team.

This module exposes capability-based model calls and `contract.Job` resources.
It may know provider adapters, logical model aliases, retry policy, quotas,
usage, and cost. It must not create or update Assets records directly.

When a generation job completes, the caller passes the verified output to the
Assets module's generated-asset intake API. That API is the only route that
returns a project-library `contract.AssetRef`.
