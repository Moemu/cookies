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

The static platform build exposed these exact paths:

- `POST /superior/api/v2/project/create`
- `POST /superior/api/v2/project/list`
- `POST /superior/api/v2/promotion/create_promotion`
- `POST /superior/api/ad/promotion/list`

The source file has no available Source Map. Static code did not provide an
accepted request and response fixture. The contract fixture therefore has the
state `static_source_only`.

## Release gates

`COOKIES_OCEAN_ENGINE_WEB_API_WRITE_ENABLED` defaults to `false`.
`COOKIES_OCEAN_ENGINE_WEB_API_WRITE_ACCOUNT_ALLOWLIST` must also contain the
exact external account ID. The current adapter blocks Submit before it consumes
the one-time confirmation token because the live contract is not captured.

Capture one project and one promotion with a dedicated test account. Use a
future schedule and the minimum budget. Replace all identifiers, names, Cookie,
and CSRF values before adding the fixtures.

Do not enable the contract if the request needs an extra signature or device
binding. Do not use Secsdk `DOWNGRADE` values.
