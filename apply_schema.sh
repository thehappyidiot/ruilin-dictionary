#!/usr/bin/env bash
set -euo pipefail

if [[ -f ".env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

GOOSE_DRIVER="${GOOSE_DRIVER:-postgres}"
GOOSE_MIGRATION_DIR="${GOOSE_MIGRATION_DIR:-./sql/schema}"

if [[ -z "${GOOSE_DBSTRING:-}" ]]; then
  echo "GOOSE_DBSTRING is not set. Configure it in .env (see .env.example)." >&2
  exit 1
fi

if command -v goose >/dev/null 2>&1; then
  goose -dir "$GOOSE_MIGRATION_DIR" "$GOOSE_DRIVER" "$GOOSE_DBSTRING" up
elif command -v go >/dev/null 2>&1; then
  go run github.com/pressly/goose/v3/cmd/goose@latest -dir "$GOOSE_MIGRATION_DIR" "$GOOSE_DRIVER" "$GOOSE_DBSTRING" up
else
  echo "Neither goose nor go is installed. Install goose or Go first." >&2
  exit 1
fi
