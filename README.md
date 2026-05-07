# Ruilin Dictionary

Self-hosted Go + Postgres web app for managing a personal dictionary.

## Deployment model

- `docker compose` runs two services: `app` and `db`.
- Database migrations are embedded in the Go binary and run automatically on startup.
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

   The app applies any pending migrations on startup.

5. Verify health:

   ```bash
   curl -fsS http://localhost:18080/api/health
   ```

## Updating deployment from new code

From the repo on the homeserver:

```bash
git pull
docker compose up -d --build
```

This rebuilds/restarts the app container while preserving Postgres data in the named volume. New migrations bundled with the new build are applied automatically on next start.

## Cloudflare Tunnel notes

- Keep tunnel config on host (not in Compose).
- Route your public hostname to `http://localhost:18080` (or your `HOST_PORT`).
- Do not publish Postgres to public interfaces.

## Migrations

- Source files live in `internal/migrations/schema/` and use Goose's `-- +goose Up` / `-- +goose Down` format.
- They are embedded into the binary via `go:embed` and applied with `goose.Up` at server startup.
- To create a new migration locally:

  ```bash
  goose -dir ./internal/migrations/schema create your_migration_name sql
  ```

  After committing it, the next `docker compose up -d --build` applies it automatically.
