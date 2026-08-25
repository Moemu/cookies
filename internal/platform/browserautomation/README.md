# Browser RPA control plane

The control plane uses OceanEngine Runner v3 by default.

Set these values to enable the automated worker:

```text
COOKIES_BROWSER_RPA_ENABLED=true
COOKIES_BROWSER_RPA_RUNNER_PROTOCOL=v3
COOKIES_BROWSER_RPA_EDGE_SESSION_FILE=C:\Users\USER\AppData\Local\cookies\browser-rpa\session.json
```

The worker uses the persistent Edge session file. Runner v3 resolves the
current DevTools WebSocket before each process starts.

The control plane executes two phases:

1. `prepare` fills the form and stops before the final click.
2. `submit` consumes one final confirmation and permits one final click.

The raw confirmation token stays in process memory. It is not stored in run
evidence. Runner v3 writes one authority-consumption record below
`COOKIES_BROWSER_RPA_AUTHORITY_STATE_ROOT`.

The v3 control-plane converter supports one promotion budget edit and one
promotion create in a bound existing project. Each run binds exact numeric
account and platform object IDs. The site policy must allow the exact form.

## Delivery configuration planning

`plancompile.CompileConfigurationV3` converts one immutable OceanEngine
`PlatformConfiguration` into an ordered form-plan set. The set contains one
project form and one form for each promotion.

- A missing confirmed platform binding produces a create plan.
- A confirmed numeric platform binding produces an edit plan.
- A promotion under a new project depends on the created project platform ID.
- Each form includes the planned field differences shown before confirmation.
- Budget, bid, Shanghai schedule, and next-day limits are checked first.
- Material, image, landing-page, product, brand, and category references must be resolved.
- Intent-owned references must also exist in the bound DeliveryIntent.
- Only the calibrated ecommerce short-video and image-text account path is accepted.

`V3BindingsFromMappings` accepts confirmed `PlatformEntityMapping` records only.
It keeps Cookies draft IDs separate from OceanEngine object IDs.

One Browser RPA run executes one form. A project plus its promotions must use
staged controlled runs. The compiler rejects a compound remote write before
Edge receives a plan.

The following actions stop before browser execution:

- project and promotion compound creation;
- promotion material replacement;
- promotion pause or resume.

These actions need a one-form Runner v3 contract before they can run.

The old v2 runner remains an explicit rollback path. Set:

```text
COOKIES_BROWSER_RPA_RUNNER_PROTOCOL=legacy
```

This selects `scripts/browser-rpa-runner.ts`. The control plane does not
automatically fall back from v3 to legacy. This rule prevents one approved
run from changing protocols without a new process configuration.
