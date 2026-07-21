# Local infrastructure

`docker compose up -d` starts only local PostgreSQL and Redis. Application
processes remain host-run during the skeleton phase so Go and React developers
can iterate independently.

Use the root `compose.yaml`; it includes `docker-compose.yml` in this directory.
