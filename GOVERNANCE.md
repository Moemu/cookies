# Governance

## Purpose

cookies is maintained as an open project with a clear technical and product direction. This document explains how decisions are made while the project is pre-release.

## Roles

| Role | Responsibilities |
| --- | --- |
| **Maintainer** | Triage issues, review contributions, protect releases, enforce community standards, and make routine decisions. |
| **Project lead** | Own the product direction, architecture boundaries, release readiness, and maintainer appointments. |
| **Contributor** | Proposes, discusses, documents, tests, or implements improvements. |

Current maintainers and role holders are listed in [MAINTAINERS.md](./MAINTAINERS.md).

## Decision making

- Routine fixes and documentation changes are decided by the reviewing maintainer.
- Changes to product scope, public APIs, domain ownership, security controls, dependencies, licenses, or release policy require project-lead approval.
- Significant proposals should begin in a GitHub Discussion or issue, include alternatives and compatibility impact, and remain open for community feedback before implementation.
- The published specifications are the current source of truth until replaced by an approved decision record or pull request.

Maintainers seek consensus. When consensus is not reached in a reasonable time, the project lead makes the final decision and records the rationale in the relevant issue, pull request, or ADR.

## Merge and release authority

- Only maintainers may merge pull requests, change protected-branch settings, publish releases, or modify project secrets.
- A maintainer must review every external contribution. Changes owned by a maintainer should be reviewed by another maintainer when one is available.
- Changes affecting security, compatibility, or real advertising-account actions require explicit project-lead approval.
- Releases require passing required checks, updated release notes, and confirmation that relevant security and compatibility gates have been met.

## Becoming a maintainer

Maintainers are appointed by the project lead based on sustained, constructive contributions; sound judgment; respectful collaboration; and demonstrated understanding of the project's product, security, and architecture boundaries. Maintainers may step down at any time. The project lead may suspend or remove maintainer access when necessary to protect the project or community, documenting the decision where appropriate.

## Changes to this document

Governance changes require a public pull request and project-lead approval.
