# Local infrastructure

`docker compose up -d` starts the local MySQL 8.4 dependency. Redis is not a
runtime dependency in the current MVP and is intentionally omitted.
processes remain host-run during the skeleton phase so Go and React developers
can iterate independently.

Use the root `compose.yaml`; it includes `docker-compose.yml` in this directory.

## Local MySQL resource boundaries

The local container defaults to a 640 MiB memory limit, a 192 MiB InnoDB
buffer pool, and 40 server connections. Override
`COOKIES_MYSQL_MEMORY_LIMIT`, `COOKIES_MYSQL_INNODB_BUFFER_POOL_SIZE`, or
`COOKIES_MYSQL_MAX_SERVER_CONNECTIONS` only when the host has enough memory.

Platform Playwright runs reset a port-scoped `cookies_e2e_<api-port>` database
before starting. Set `COOKIES_E2E_MYSQL_DATABASE` to choose another dedicated
test database. `COOKIES_E2E_SKIP_MYSQL_BOOTSTRAP=true` and
`COOKIES_E2E_REUSE_SERVERS=true` preserve caller-owned data and therefore also
make the caller responsible for cleanup.

Routine `cookies-migrate` runs only canonicalize DeliveryPlan snapshots that
still lack a hash. Run the intentionally expensive full integrity audit only
when required:

```bash
go run ./cmd/cookies-migrate --verify-delivery-hashes
```

If WSL starts swapping heavily, stop API/test clients first and then stop the
MySQL container without deleting its volume:

```bash
docker stop -t 30 cookies-mysql-1
```

Never use `docker compose down -v` as a resource-recovery command because it
deletes the local database volume.
