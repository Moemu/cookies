# Local infrastructure

`docker compose up -d` starts the local MySQL 8.4 dependency. Redis is not a
runtime dependency in the current MVP and is intentionally omitted.
processes remain host-run during the skeleton phase so Go and React developers
can iterate independently.

Use the root `compose.yaml`; it includes `docker-compose.yml` in this directory.
