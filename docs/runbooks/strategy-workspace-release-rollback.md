# Strategy workspace release and rollback checklist

This runbook applies to the Cookies repository only. It contains no credentials and must not be replaced by an external service's deployment notes.

## Security stop

- Never copy credentials, private keys, RDS DSNs or TOS secrets into Git, CI logs, shell history or a chat response.
- The separately supplied `deploy-push-runbook.md` describes another repository and contains plaintext credentials. It is not a Cookies deployment source of truth. Rotate every credential exposed there before relying on that environment again.
- Do not run a production migration, push, deploy, or paid LAS smoke test without explicit approval for that exact environment and action.

## Release shape

This change is a coordinated frontend, API and database release. Database migrations are forward-only during the release window. Application rollback is supported because:

- new columns and tables are additive;
- document vision is default-off behind `COOKIES_DOCUMENT_VISION_ENABLED=false`;
- PPTX-to-PDF conversion is independently default-off behind `COOKIES_DOCUMENT_CONVERTER_ENABLED=false`;
- the old text parse remains the usable fallback;
- the new Strategy routes replace legacy UI writes but retain deterministic redirects;
- the Creative downstream production state machine is unchanged.

The provider connection-type CHECK migration is the only constraint replacement. Rehearse it against a schema clone and record lock duration before production.

## Pre-release gate

1. Freeze one exact commit and record its parent commit as the application rollback target.
2. Confirm the working tree contains only reviewed scope; do not stage unrelated user changes.
3. Run:

   ```powershell
   git diff --check
   # This command must print no paths.
   gofmt -l cmd internal
   Get-ChildItem api/events,api/contracts,api/fixtures -Filter *.json |
     ForEach-Object { [System.IO.File]::ReadAllText($_.FullName) | ConvertFrom-Json | Out-Null }
   go test ./...
   go vet ./...
   go build ./cmd/cookies-api
   go build ./cmd/cookies-migrate
   npm.cmd run check:server
   npm.cmd test
   npm.cmd run test:server
   npm.cmd run build
   npm.cmd run contract:check
   npx.cmd playwright test e2e/strategy-workspace-rearchitecture.spec.ts --config=playwright.platform.config.ts
   ```

   Also run `go test -race ./...` in the same Linux/CGO-capable environment used by CI. A Windows host with `CGO_ENABLED=0` does not execute the race detector; a plain `go test` result must not be reported as an equivalent substitute. After the exact candidate is pushed, the required GitHub Actions checks remain authoritative even when every local command passes.

4. Run MySQL integration tests against a disposable schema, including Knowledge, Provider and product-event UX metrics.
5. Take a database backup and verify restore access. Record row counts for Strategy, Knowledge and Provider route tables.
6. Run migrations against a production-like clone. Confirm every migration is current and inspect the provider CHECK constraint. Use the repository rehearsal command below to populate an empty, disposable loopback schema with the approved table-specific row counts and capture migration plus concurrent-read latency. A clean-schema run proves ordering and shape only; the 25,000-row synthetic run proves the harness and bounded local behavior, but neither replaces the production-volume run.
7. Keep these flags off for the first application rollout:

   ```dotenv
   COOKIES_DOCUMENT_VISION_ENABLED=false
   COOKIES_DOCUMENT_VISION_AUTO_FALLBACK=false
   ```

8. Confirm the one shared private TOS bucket is configured through scoped prefixes. CORS and lifecycle remain an explicit operations debt and are not changed by this release.

Before any paid canary, run the repository's read-only readiness command:

```powershell
go run ./cmd/cookies-check-document-vision-readiness
```

It loads the same process/`.env` configuration as the API, performs only a read-only Provider capability query, and emits `document-vision-readiness/v1`. The output contains boolean state and fixed blocker codes only; it never contains credentials, bucket/object names, DSNs, endpoints, or route URLs. `ready=true`, `database_checked=true`, `route_available=true`, and `credential_configured=true` are all required. A non-ready result exits nonzero; `go run` may render that program exit as `exit status 2`.

For configuration-only diagnosis without a database query, use `-config-only`. This does not prove route or credential readiness and is not sufficient for a canary. The interactive `scripts/configure-las-document-vision.ps1` now validates all required `COOKIES_TOS_*` names before reading the LAS key and runs the full readiness check after saving the encrypted route. Do not use unscoped `TOS_*` aliases.

## Populated migration rehearsal

The rehearsal command refuses non-loopback MySQL, a schema whose name does not begin with `cookies_rehearsal_`, a schema containing any table, or a DSN without `multiStatements=true`. It never creates, drops or empties the schema. Create the disposable empty schema separately, run the command, preserve the JSON report, and clean up only through an explicitly reviewed operation.

```powershell
$env:COOKIES_REHEARSAL_MYSQL_DSN = '<local-rehearsal-dsn-with-multiStatements=true>'
go run ./cmd/cookies-rehearse-strategy-migrations `
  -research-rows <approved-count> `
  -document-rows <approved-count> `
  -memory-rows <approved-count> `
  -provider-rows <approved-count> `
  -proposal-rows <approved-count> `
  -product-event-rows <approved-count> `
  -analysis-rows <approved-count> `
  -max-migration 30s `
  -max-read-block 2s `
  -production-like `
  -baseline-label '<non-sensitive-baseline-id>' `
  -output '<approved-report-path>'
Remove-Item Env:COOKIES_REHEARSAL_MYSQL_DSN
```

Do not use `-production-like` for invented or rounded counts. The JSON report must have `passed=true`, `production_like=true`, zero concurrent read errors, preserved final row counts, and every measurement within the approved thresholds. Review the provider connection CHECK replacement separately even when the aggregate report passes.

The 2026-08-11 harness qualification used 25,000 synthetic rows in each of seven affected tables on local MySQL 8.4.10. All 13 staged migrations passed; the slowest migration took 2,347 ms and the largest concurrent read latency was 132 ms. This is synthetic evidence (`production_like=false`) and does not close the release gate.

## Deployment order

1. Put worker intake into drain mode; do not kill running research, parse or Agent tasks.
2. Apply database migrations once from the approved migration artifact.
   The migrator records SHA-256 for every applied file and must stop if an applied migration's content changes. Never bypass that failure; add a new forward compatibility migration instead.
3. Start the new API and worker version with document vision disabled.
4. Start the frontend only after API health and migration status pass.
5. Re-enable worker intake.
6. Run authenticated smoke checks:

   - Strategy task list and stable five-stage workspace route;
   - Assistant command acknowledgment and Activity status;
   - non-blocking research start, status and cancellation;
   - document upload, milestone progress, partial preview and retry;
   - self-confirmation versus formal review policy;
   - frozen Strategy package to Creative handoff.

7. Query `/api/strategy/v1/projects/{project_id}/workspace-ux-metrics?days=1`. Zero samples are valid for a new environment; schema errors or cross-project data are not.

## Document vision canary

Document vision is a separate, explicitly approved canary after the base release is stable.

1. Configure an encrypted fixed LAS route and the shared TOS bucket. Never place the LAS key in `.env` plaintext or SQL.
2. Enable `COOKIES_DOCUMENT_VISION_ENABLED=true` for one canary environment only. Keep automatic fallback off.
3. Use one low-page-count, non-sensitive PDF and manually confirm the exact pages.
4. Verify TOS access, returned page numbering, Markdown/locator bounds, route lineage, task checkpoint, latency and billable pages.
5. For `DOCUMENT_VISION_SUBMISSION_UNKNOWN`, stop. Reconcile against LAS before any retry.
6. Keep `COOKIES_DOCUMENT_CONVERTER_ENABLED=false` until the internal Gotenberg service is healthy and a canary PPTX confirms fonts, slide count, derived-PDF lineage, and LAS output. Enabling it also requires `COOKIES_DOCUMENT_CONVERTER_ALLOW_INSECURE_HTTP=true` for an explicitly trusted internal HTTP endpoint, or an HTTPS endpoint.

Automatic fallback is a later admission gate, not part of this canary. Before requesting it, collect a real deidentified dataset under the [Phase 11 blinded evaluation protocol](../testing/2026-08-11-strategy-workspace-phase-11-document-vision-evaluation-protocol.md), validate it against `document-vision-evaluation-dataset/v1`, and archive the generated report plus its `dataset_sha256`. A passing report still requires product-owner approval; missing evidence, any report blocker, unreconciled billable pages, or a changed parser/model/route/prompt/converter/cost-policy version keeps automatic fallback closed.

Cookies now exposes the project-scoped administrator reconciliation APIs described in [Document vision submission reconciliation](document-vision-submission-reconciliation.md). They preserve the pre-call intent, require two different administrators, bind an accepted external task without resubmitting, and retain an immutable resolution audit. Cookies still cannot independently prove that LAS did not accept an unknown submission: a `not_accepted` decision requires unambiguous provider-console or support evidence retained outside the application. Do not mutate the task row or clear the error to make the UI retryable. Production automatic fallback remains blocked until the real-account reconciliation boundary and paid canaries are validated, the blinded evaluation passes, and the product owner separately approves enablement.

## Rollback decision

Rollback the application immediately when any of these occur:

- cross-organization/project data exposure;
- repeated navigation/focus changes caused by background completion;
- growing queued/running tasks without heartbeat or recovery;
- Strategy writes bypassing revision/hash checks;
- Creative handoff changing downstream production state unexpectedly;
- duplicate LAS submission or unexplained billable pages.

## Rollback procedure

1. Disable document vision first. This stops new paid submissions without deleting text results or checkpoints.
2. If only presentation conversion is unhealthy, disable `COOKIES_DOCUMENT_CONVERTER_ENABLED` first; PDF visual fallback can remain available.
3. Drain new worker claims and allow known in-flight tasks to checkpoint. Do not blindly retry `submitting` or `unknown` external tasks.
4. Roll the frontend and application containers back to the recorded parent image/commit.
5. Leave additive database migrations in place during the incident rollback. Older application code ignores the new nullable/defaulted fields and tables.
6. Do not run destructive down migrations while any new-version task or event exists. A schema rollback requires a separate approved maintenance window, a backup and explicit orphan-data handling.
7. Re-enable the old application workers and verify text parsing, research, review and handoff.
8. Preserve failed job rows, external task IDs, route revisions and product events for incident analysis. Do not delete them to make dashboards look healthy.

## Post-release observation

For the first 24 hours, compare 15-minute and 24-hour windows for:

- assistant missing acknowledgment/meaningful-update counts and p50/p95 latency;
- stalled, retried and terminal failure counts;
- research completed/partial/failed distribution and proposal adoption;
- document ready/partial/failed distribution and terminal latency;
- visual attempts, terminal outcomes and billable pages;
- review-mode distribution and handoff creation.

The metrics are observed workflow evidence, not a causal claim. The offline evaluator can quantify blinded correction-session time for matched pages, but product-level “time saved” remains unmeasured until the 2—4 week workflow baseline and a comparable post-release cohort exist.
