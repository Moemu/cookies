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

Both captured writes included the `_signature` query parameter. They also
included `x-sessionid`, `x-csrftoken`, and the Secsdk CSRF header. This is an
additional request binding. The migration policy requires a stop. Do not
generate, replay, or bypass these values.

The HAR included write request bodies. It did not include response bodies.
Only field names and JSON types are stored in Git. The contract fixture has the
state `blocked_additional_request_binding`.

## Release gates

`COOKIES_OCEAN_ENGINE_WEB_API_WRITE_ENABLED` defaults to `false`.
`COOKIES_OCEAN_ENGINE_WEB_API_WRITE_ACCOUNT_ALLOWLIST` must also contain the
exact external account ID. The current adapter blocks Submit before it consumes
the one-time confirmation token because the request binding is unsupported.

The 2026-08-31 probe used a dedicated test account, future dates, and the
minimum controlled budget. The operator paused both objects. The captured
signature and session binding make production Web API writes a no-go. Keep
Prepare and read-only research available. Do not use Secsdk `DOWNGRADE` values.
