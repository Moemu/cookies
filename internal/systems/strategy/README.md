# Strategy system

Owner: Strategy team.

This package owns the Strategy MVP business facts and state machines:
Workspace, Conversation, Brief Draft/Version, Strategy Draft/Revision,
Review, and Strategy Package Version. It uses Platform Agent, Job Runtime,
Provider, and Event Outbox through their public seams; it never creates a
Creative task.

The authenticated HTTP API is mounted under `/api/strategy/v1/`. The immutable
cross-module reader is:

`GET /api/strategy/v1/projects/{project_id}/strategy-packages/{package_id}/versions/{version}`
