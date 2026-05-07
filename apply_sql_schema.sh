#!/usr/bin/env bash
set -euo pipefail

echo "This script is deprecated. Use ./apply_schema.sh (Goose-managed migrations)." >&2
./apply_schema.sh