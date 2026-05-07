#!/usr/bin/env bash
set -euo pipefail

if [[ -f ".env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

DB_USER="${POSTGRES_USER:-ruilin_user}"
DB_NAME="${POSTGRES_DB:-ruilin_dictionary}"

for file in sql/schema/*.sql; do
  echo "Applying ${file}..."
  docker compose exec -T db psql -U "$DB_USER" -d "$DB_NAME" < "$file"
done