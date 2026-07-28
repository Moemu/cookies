#!/usr/bin/env bash
set -euo pipefail

prepare_only=0
if [[ "${1:-}" == "--prepare-only" ]]; then
  prepare_only=1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

for command in docker go npm; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "Required command '$command' was not found in PATH." >&2
    exit 1
  fi
done

export COOKIES_ENV="${COOKIES_ENV:-local}"
export COOKIES_HTTP_ADDR="${COOKIES_HTTP_ADDR:-:8080}"
export COOKIES_MYSQL_DSN="${COOKIES_MYSQL_DSN:-cookies:cookies_local_development_only@tcp(127.0.0.1:3306)/cookies?parseTime=true&multiStatements=true}"
export COOKIES_LOCAL_ORGANIZATION_ID="${COOKIES_LOCAL_ORGANIZATION_ID:-org_local}"
export COOKIES_LOCAL_PRINCIPAL_KIND="${COOKIES_LOCAL_PRINCIPAL_KIND:-user}"
export COOKIES_LOCAL_PRINCIPAL_ID="${COOKIES_LOCAL_PRINCIPAL_ID:-user_local}"
export COOKIES_LOCAL_PROJECT_ID="${COOKIES_LOCAL_PROJECT_ID:-project_local}"
export COOKIES_LOCAL_SCOPES="${COOKIES_LOCAL_SCOPES:-project.read,project.write,assets.read,assets.write,provider.job.create,provider.text.generate,provider.vision.understand}"
export COOKIES_BLOB_PROVIDER="${COOKIES_BLOB_PROVIDER:-filesystem}"
export COOKIES_FILESYSTEM_BLOB_ROOT="${COOKIES_FILESYSTEM_BLOB_ROOT:-$repo_root/.data/blobs}"
export COOKIES_SCANNER_MODE="${COOKIES_SCANNER_MODE:-noop}"

echo "Starting MySQL and waiting for it to become healthy..."
docker compose up -d --wait mysql

echo "Applying migrations and seeding the canonical Go demo..."
go run ./cmd/cookies-seed

if [[ ! -d "$repo_root/node_modules" ]]; then
  echo "Installing frontend dependencies (first run only)..."
  npm ci
fi

if [[ "$prepare_only" == "1" ]]; then
  echo "Development dependencies and Go demo seed are ready."
  exit 0
fi

cleanup() {
  if [[ -n "${backend_pid:-}" ]]; then
    kill "$backend_pid" 2>/dev/null || true
  fi
  if [[ -n "${frontend_pid:-}" ]]; then
    kill "$frontend_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

echo "Starting Go cookies-api on http://127.0.0.1:8080..."
go run ./cmd/cookies-api &
backend_pid=$!

echo "Starting Vite frontend on http://127.0.0.1:5173..."
npm run dev -- --host 127.0.0.1 &
frontend_pid=$!

wait "$backend_pid" "$frontend_pid"
