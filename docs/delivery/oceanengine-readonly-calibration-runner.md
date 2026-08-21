# OceanEngine read-only calibration Runner

## Purpose

This Runner attaches to the reusable local Edge session. It reads page facts for OceanEngine calibration.

The Runner has no submit authority. It does not use or extend the production Browser RPA Plan.

## Contracts

The Runner uses two independent versioned contracts:

- `api/contracts/oceanengine-readonly-calibration-plan-v1.schema.json`.
- `api/contracts/oceanengine-readonly-calibration-result-v1.schema.json`.

Plan v1 permits these step kinds only:

- `identify_page`.
- `scope_check`.
- `locator_unique`.
- `presence_check`.
- `readback`.

The host, page kind, account query key, locator kinds, locator keys, roles, and value sources use closed enums.

Unknown properties and unknown steps are invalid. A Plan with `remoteWrite` is invalid.

## Safety boundary

The implementation has no method for these actions:

- Click, fill, press, select, check, or drag.
- Upload, download, or clipboard access.
- Arbitrary script evaluation.
- Final confirmation or remote write.
- Browser close.

Each CLI run uses a separate Playwright process. Process exit disconnects CDP. It does not close Edge.

Edge can omit `browserContextId` for its default Context. It can also contain an unrelated blocked page.

The version-locked post-install patch handles both cases. It permits the missing default Context ID. It does not wait for unrelated pages.

The Runner then waits only for the allowed OceanEngine page. The patch rejects an unknown Playwright version.

The Runner reads the account value only from the fixed `aadvid` query key. It stores only SHA-256.

The Result stores only these facts:

- Controlled host and page kind.
- Hashes for path, account context, session context, accessible name, and value.
- Element counts, visibility, value type, and controlled states.
- Page structure counts.
- Controlled locator keys and error codes.

The Result does not store URL queries, IDs, names, balances, browser data, or free text.

## Local artifacts

All raw Runner artifacts stay below:

```text
%LOCALAPPDATA%\cookies\browser-rpa\calibration\
```

The directory contains:

- `live-plan.json`: local Plan with the account-context hash.
- `results\*.json`: raw versioned Results.
- `diagnostics.jsonl`: controlled command outcomes and error codes.

Do not copy these files to Git, `data\`, or `.env`.

Only sanitized contract fixtures can enter Git.

## Commands

First, start and verify the Goal 1 Edge session.

```powershell
npm run browser-rpa:edge -- status
```

Create the local Plan from the current signed-in promotion-list page:

```powershell
npm run browser-rpa:calibrate -- init
```

Run the read-only Plan:

```powershell
npm run browser-rpa:calibrate -- run
```

Run the Runner tests:

```powershell
npm run test:browser-rpa:calibrate
```

## Result rules

Report success only when a real Result has all these values:

- `outcome=success`.
- `error_code=ok`.
- `account_context_state=matched`.
- The required locator checks passed.
- A readback contains count, visibility, accessible-name state, and value state.
- The Edge process remains running after Runner exit.

Do not use a fixture or fake page as real calibration evidence.
