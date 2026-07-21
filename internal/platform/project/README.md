# Project platform module

Owner: Identity, Organization, and Project team.

This module will own projects, memberships, project-level authorization, and
the authoritative ProjectContext projection. The shared contract package only
contains opaque references and must not grow project business fields.
