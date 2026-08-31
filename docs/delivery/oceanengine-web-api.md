# OceanEngine Web API driver

The driver ID is `oceanengine-web-api/session/v1`.

The driver uses the organization Connector session. It does not require Edge,
CDP, screenshots, or page selectors. Historical runs use
`playwright-rpa/edge/v3` when they have no stored driver.

## Frozen observations

- The inspected Secsdk version is `1.2.22`.
- `TOKEN_PATH` is undefined.
- The CSRF HEAD uses the protected POST path.
- HEAD sends `x-secsdk-csrf-request: 1`.
- HEAD sends `x-secsdk-csrf-version: 1.2.22`.
- The server returns `x-ware-csrf-token`.
- POST sends `x-secsdk-csrf-token`.

The static platform build and the sanitized HAR exposed these exact paths:

- `POST /superior/api/v2/project/create`
- `POST /superior/api/v2/promotion/create_promotion`
- `GET /superior/api/v2/project/detail`
- `GET /superior/api/ad/promotion/detail`

Both captured browser writes included the `_signature` query parameter. They
also included `x-sessionid`, `x-csrftoken`, and the Secsdk CSRF header. This
proves presence only. It does not prove that the server requires every field.

The public platform bundle shows the field sources. Its request interceptor
sets `X-SessionId` from `window.sessionId`. Its URL transformer calls an
obfuscated `sign({url, body})` module and adds `_signature`. The transformer
handles same-origin GET requests and supported POST content types. This is
browser-client enrichment. It is not evidence of a server requirement.

The existing Connector omits `_signature` and `x-sessionid` for multiple live
read-only GET and POST routes. The write Client must omit both fields first.
Add a field only after an isolated experiment proves that the server requires
it. Do not infer a write requirement from the browser request alone.

The HAR included write request bodies. It did not include response bodies.
Only field names and JSON types are stored in Git. The contract fixture has the
state `request_captured_response_pending`.

## Release gates

`COOKIES_OCEAN_ENGINE_WEB_API_WRITE_ENABLED` defaults to `false`.
`COOKIES_OCEAN_ENGINE_WEB_API_WRITE_ACCOUNT_ALLOWLIST` must also contain the
exact external account ID. The current adapter blocks Submit before it consumes
the one-time confirmation token because response and reconciliation contracts
are not captured.

The 2026-08-31 probe used a dedicated test account, future dates, and the
minimum controlled budget. The operator deleted both objects after capture.
Continue controlled research on the two optional browser fields. Keep Prepare
and read-only research available. Do not use Secsdk `DOWNGRADE` values.
