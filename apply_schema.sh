#!/usr/bin/env bash
set -euo pipefail

docker compose up -d db
docker compose run --rm migrate
