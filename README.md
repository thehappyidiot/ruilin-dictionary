# Ruilin Dictionary

Self-hosted Go + Postgres web app for managing a personal dictionary.

## Deployment model

- `docker compose` runs two services: `app` and `db`.
- Cloudflare Tunnel stays on the homeserver host and points to `http://localhost:18080` (or your `HOST_PORT`).
- No CI/CD is required; deploys happen directly from the repo checkout.

## Prerequisites

- Docker Engine with Compose plugin (`docker compose version`)
- Cloudflare Tunnel already installed and managed on host

## First-time homeserver setup

1. Clone this repo on the homeserver.
2. Create environment file:

   ```bash
   cp .env.example .env
   ```

3. Edit `.env` and set:
   - `POSTGRES_PASSWORD` to a strong value
   - `ADMIN_PASSWORD_HASH` to your bcrypt password hash
   - `SESSION_SECRET` to a random secret (at least 32 chars)
4. Build and start the stack:

   ```bash
   docker compose up -d --build
   ```

5. Apply SQL schema (first deployment only):

   ```bash
   ./apply_schema.sh
   ```

   Run this from the repo root and do not use `sudo`.
   This runs Goose in a one-off Docker container and applies only `-- +goose Up` sections.

6. Verify health:

   ```bash
   curl -fsS http://localhost:18080/api/health
   ```

## Updating deployment from new code

From the repo on the homeserver:

```bash
git pull
docker compose up -d --build
```

This rebuilds/restarts the app container while preserving Postgres data in the named volume.

## Cloudflare Tunnel notes

- Keep tunnel config on host (not in Compose).
- Route your public hostname to `http://localhost:18080` (or your `HOST_PORT`).
- Do not publish Postgres to public interfaces.

## Migration notes

- Do not pipe migration files directly into `psql` if they contain Goose markers.
- `psql` treats `-- +goose Up/Down` as comments and executes all SQL, including down migrations.
- `./apply_schema.sh` runs `docker compose run --rm migrate`, so no host `go`/`goose` install is needed.
- Goose records applied versions in its migration table, so reruns are safe.
