# OceanEngine Runner v3 Submit Runbook

## Purpose

Use Runner v3 for one controlled OceanEngine project or promotion write.

The default plan is `prepare`. It cannot click the final button.

Use an execution authority only for one approved `submit` run.

## Safety limits

- Use an exact numeric `aadvid`.
- Use an exact numeric object ID for edit plans.
- Use an exact numeric parent project ID for promotion plans.
- Set the schedule to the approved date.
- Set every budget and bid within the authority limit.
- Keep `maximum_final_clicks` equal to `1`.
- Do not retry when the result is `result_unknown`.
- Reconcile the exact object name and platform ID.
- Confirm again before deletion.

Runner v3 stores an authority consumption file before the final click.

The same authority cannot run again.

## Prepare flow

1. Compile one form plan.
2. Run the plan in `prepare` mode.
3. Check all field readbacks.
4. Check the screenshot when a locator is uncertain.
5. Stop at `final_click_boundary`.

The prepare result must have:

- `outcome: success`
- `final_click_performed: false`
- final step status `blocked_boundary`

## Issue a short authority

Run:

```text
npm run oceanengine:authority -- issue PREPARE_PLAN.json --account-reference AADVID --maximum-money 300 --schedule-date YYYY-MM-DD
```

Use `-` instead of `PREPARE_PLAN.json` to read the plan from stdin.

The command returns:

- an authorized submit plan;
- one confirmation token.

The authority lifetime is 10 minutes by default.

The maximum lifetime is 15 minutes.

Do not save the raw token in evidence files.

## Submit flow

Pass the authorized plan through stdin.

Run Runner v3 with:

```text
npm run oceanengine:runner-v3 -- CDP_URL --confirm-token TOKEN --authority-state-dir STATE_DIRECTORY
```

For the persistent current-user Edge session, use its session file:

```text
npm run oceanengine:runner-v3 -- --session-file SESSION_JSON --confirm-token TOKEN --authority-state-dir STATE_DIRECTORY
```

The session command reads `DevToolsActivePort` and resolves the direct WebSocket endpoint.

An HTTP 404 from `/json/version` does not prove that a current-user Edge session is stale.

Run this check first:

```text
npm run browser-rpa:edge -- status
npm run browser-rpa:edge -- check
```

Use one persistent authenticated Edge session.

Do not start a new Edge process for each form step.

Do not use desktop Computer Use for this flow.

## Async picker rules

Text-only location is not valid for image cards.

Use a reference selection object:

```json
{
  "selection_kind": "image_card",
  "index": 0,
  "minimum_visible": 1,
  "confirm_button": "确定"
}
```

Runner v3 verifies:

- the card is visible;
- the card contains an `img` element;
- the card has a selected checkbox or selected class;
- the visible card count is stable.

For an asynchronously loaded landing-page row, use:

```json
{
  "selection_kind": "async_row",
  "label": "LANDING_PAGE_NAME",
  "expected_total": 25,
  "confirm_button": "确定"
}
```

When names are duplicated, bind the exact object ID:

```json
{
  "selection_kind": "async_row",
  "label": "PRODUCT_NAME",
  "object_id": "PLATFORM_OBJECT_ID",
  "confirm_button": "确定"
}
```

Runner v3 selects the checkbox inside that exact row.

The expected total is evidence for the current account.

Recalibrate it when the account data changes.

## Result handling

`success` means the platform reported success and reconciliation completed.

`success_with_drift` means the object exists, but a persisted field differs.

For promotion create, Runner v3 reads the landing-page ID from the edit form.

Do not treat `success_with_drift` as an exact field match.

`failed` means no safe write result was accepted.

`result_unknown` means one click occurred, but the result is not stable.

For `result_unknown`:

1. Do not submit again.
2. Reload the list.
3. Search the exact name.
4. Read the platform ID.
5. Record the final state.

## Cleanup

Delete the promotion before the parent project.

Use exact names and IDs.

After deletion, search both exact names.

Both searches must return zero rows.
