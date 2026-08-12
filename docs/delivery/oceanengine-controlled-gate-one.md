# OceanEngine controlled browser gate one

Gate one validates the first visible-browser path without causing a remote
side effect. It is not a test of final submission and never authorizes gate
two.

Before opening the browser, the operator must show and obtain current-turn
confirmation for all of the following:

- the exact OceanEngine account reference;
- the complete platform project-ID allowlist for this run;
- the CNY budget ceiling;
- the versioned `oceanengine-ecommerce-manual` Skill;
- the forbidden actions: save, create, submit, enable, and modify.

The driver may navigate to the exact allowlisted HTTPS host, recognize the
account/page/project, fill only the unsubmitted local form, and read every
field back. Any host, page, account, project, selector, or readback drift stops
the run and discards local form state. Coordinates are not a fallback.

The gate succeeds only after the approved/readback diff is empty, the driver
has stopped before `submit_platform_configuration`, and local draft state has
been discarded. Evidence must be redacted before persistence. Passwords, 2FA,
cookies, tokens, browser storage, raw keystrokes, clipboard contents, and
unredacted screenshots are outside the contract.

## Current PR status

The versioned gate-one Skill and its no-submit browser port are implemented and
covered by deterministic tests. No real browser walk-through has been run in
this Goal because no current-turn confirmation naming an account, project-ID
allowlist, and budget ceiling has been provided. Therefore this PR must report
“implementation ready; real-page gate one not validated” and must not claim a
real platform result.

Gate two remains disabled. A later general “continue” message, code approval,
or historical authorization cannot substitute for a separate current-turn
authorization naming the account, test object, single final action, and budget
ceiling.
