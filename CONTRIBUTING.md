# Contributing to cookies

Thanks for helping build cookies. This repository is in its pre-release foundation phase, so the most valuable contributions are scoped, evidence-based, and aligned with the documented product boundaries.

## Before you start

1. Read the [project overview](./docs/00-project-overview.md), [governance model](./GOVERNANCE.md), and [Code of Conduct](./CODE_OF_CONDUCT.md).
2. Search existing issues and discussions before opening a new one.
3. For a material change, open an issue or discussion first and describe the problem, proposed scope, affected documentation/contracts, and acceptance criteria.
4. Do not begin implementation for an unapproved product or architectural direction.

## Good first contributions

- Fix an error, ambiguity, or broken link in documentation.
- Improve an example, diagram, or developer-oriented explanation.
- Add a focused test or reproducible issue report once application code is available.
- Propose a small, independently reviewable improvement with clear acceptance criteria.

## Development setup

Clone with the pinned submodule:

```bash
git clone --recurse-submodules https://github.com/shikanon/cookies.git
cd cookies
```

For an existing clone, run `git submodule update --init --recursive`. The application implementation is not yet published, so there is no root-level build or test command at this time. Do not change `third_party/orag` from this repository unless the change is specifically about updating the pinned dependency; contribute ORAG code upstream instead.

## Pull requests

- Keep each pull request focused on one problem.
- Use a clear, imperative title and explain *why* the change is needed.
- Update the relevant documentation, contracts, and tests in the same pull request.
- Preserve backwards compatibility, or explicitly document migration and deprecation impact.
- Confirm that you have the right to contribute all code, text, and assets.
- Do not include credentials, customer data, personally identifiable information, unlicensed media, or generated outputs without redistribution rights.

Maintainers may request changes, split a large proposal, or close work that conflicts with the published direction. See [GOVERNANCE.md](./GOVERNANCE.md) for decision and merge rules.

## Reporting security issues

Please do not disclose vulnerabilities in public issues. Follow the private reporting process in [SUPPORT.md](./SUPPORT.md).
