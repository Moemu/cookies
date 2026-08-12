# Strategy Workspace populated migration rehearsal

## Purpose

This evidence qualifies the local rehearsal harness for the 2026-08-10/11 Strategy, Knowledge and Provider migrations. It is not production admission evidence and makes no prediction about the real production distribution.

## Safety boundary

- The command reads only `COOKIES_REHEARSAL_MYSQL_DSN`.
- It accepts loopback TCP only, requires a schema named `cookies_rehearsal_*`, requires `multiStatements=true`, and refuses a non-empty schema.
- It takes a schema-scoped MySQL advisory lock and never creates, drops, truncates or cleans up a schema.
- `-production-like` requires a non-sensitive baseline label; the qualification below did not use that assertion.

## Qualification result

Date: 2026-08-11. Database: local MySQL 8.4.10. Data: synthetic.

| Measurement | Result |
| --- | ---: |
| Populated affected tables | 7 |
| Rows per affected table | 25,000 |
| Staged migrations | 13 |
| Read errors during migration | 0 |
| Slowest migration | 2,347 ms |
| Maximum concurrent read latency | 132 ms |
| Final row-count mismatches | 0 |
| Report result | `passed=true` |
| Production-like assertion | `production_like=false` |

The slowest observed operations were research v2, document parse v2 and Strategy perspective. The provider connection CHECK replacement took 417 ms with a 28 ms maximum concurrent read; the forward compatibility repair for durable document-vision intents took 14 ms with a 1 ms maximum concurrent read in this synthetic run.

## Release interpretation

The harness, migration ordering, populated transforms, row preservation and read probe work locally. B4 remains open until operations supply approved table-specific row-count baselines, run the same command against a production-like clone, inspect the Provider CHECK lock separately, and preserve a report with `production_like=true` and all thresholds passing.

The local JSON report is stored outside the repository at `C:\Users\Admin\AppData\Local\Temp\cookies_rehearsal_strategy_1786405450.json`. Its schema is intentionally left in place because cleanup is not an implicit part of the rehearsal command.
