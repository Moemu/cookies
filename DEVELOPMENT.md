# Local development

The repository has one Go module at the root and one React application in
`web/`. The vertical systems live under `internal/systems/`; shared platform
capabilities live under `internal/platform/`.

## First-time setup

```powershell
Copy-Item .env.example .env
docker compose up -d
npm ci --prefix web
go run ./cmd/cookies-migrate
```

Start the API and frontend in separate terminals:

```powershell
go run ./cmd/cookies-api
npm run dev --prefix web
```

The API health endpoint is `http://localhost:8080/healthz`; Vite prints the
frontend URL when it starts.

## Verification

```powershell
.\scripts\check.ps1
```

Or, in an environment with `make`:

```bash
make check
```

Both commands verify Go formatting, Go static checks and tests, then lint,
test and build the React shell, and validate the OpenAPI and event schemas.

Migrations are forward-only and are not applied by API startup. Apply them
explicitly with `go run ./cmd/cookies-migrate` after MySQL is healthy.
