# Local development

The repository has one Go module and one React application at the root. The
frontend source lives in `src/`; vertical systems live under
`internal/systems/`; shared platform capabilities live under `internal/platform/`.

## First-time setup

```bash
cp .env.example .env
docker compose up -d
npm ci
go run ./cmd/cookies-seed
```

Start the API and frontend in separate terminals:

```bash
go run ./cmd/cookies-api
npm run dev
```

The API health endpoint is `http://localhost:8080/healthz`; Vite prints the
frontend URL when it starts. The root Vite app proxies `/platform` and `/api` to
the Go `cookies-api` at `http://127.0.0.1:8080`, so the frontend defaults to Go
`/platform/v1` for the project main flow. Set `VITE_API_BASE_URL` only when
pointing the frontend at a different API host.

On macOS you can run the full local loop with one script:

```bash
./scripts/dev.sh
```

To prepare the database and seed without starting long-running servers:

```bash
./scripts/dev.sh --prepare-only
```

The legacy TypeScript MVP server remains available with `npm run server` for
compatibility demos only. It persists ignored `data/mvp-store.json` state and is
not the production-facing authority once an equivalent Go `/platform/v1`
endpoint exists. To run that compatibility demo frontend against the TS server,
start `npm run server` and set `VITE_API_BASE_URL=http://127.0.0.1:8787`
explicitly for Vite.

## Verification

```powershell
.\scripts\check.ps1
```

Or, in an environment with `make`:

```bash
make check
```

Both commands verify Go formatting, Go static checks and tests, then test and
build the root React application, and validate the OpenAPI and event schemas.

Migrations are forward-only. Apply only schema changes with
`go run ./cmd/cookies-migrate`; apply migrations and the canonical Go demo seed
with `go run ./cmd/cookies-seed` after MySQL is healthy.
