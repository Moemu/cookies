# OceanEngine Platform Skill gap

PR #50 does not define or implement a real OceanEngine Platform Skill. Its
delivered scope is the controlled Computer Use control plane, formal authority
chain, deterministic fake worker, and operations workspace.

The repository PRD currently names `douyin-delivery`, `kuaishou-delivery`, and
`delivery-preflight`. The string previously introduced as
`oceanengine-ecommerce-manual` was not present in that PRD or in a Skill
Registry. Earlier appearances of `ecommerce-manual` in configuration fixture
names and evidence URIs are fixture labels, not evidence of a registered Skill.

The removed `internal/systems/delivery/oceanengineskill` prototype was only a
Go interface exercised by a fake driver. It had no loadable SkillDefinition,
capability matrix, real selector mapping, page branches, supported UI version,
owner, last-verified timestamp, rollback information, or real browser adapter.
It therefore could not support a real-page release gate.

## Current release status

- The control plane, approvals, leases, one-time confirmation, Kill Switch,
  redacted evidence, recovery rules, fake worker, and execution center exist.
- No OceanEngine Platform Skill is registered or loadable.
- No real browser adapter exists.
- Release gate one is not ready for manual validation and no real-page
  walkthrough should be attempted from this PR.
- Release gate two remains disabled.

Before gate one can be proposed again, a separately reviewed change must add a
formal SkillDefinition and registry entry, capability matrix, field-to-selector
mapping, page and error branches, supported UI versions, ownership, last
verification time, rollback information, and a visible-browser adapter. Only
then may an explicitly authorized real-page walkthrough validate it.
