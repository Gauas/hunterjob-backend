#!/usr/bin/env bash
set -euo pipefail

for migration in /migrations/[0-9]*.js; do
  echo "Running $(basename "$migration")"
  mongosh "$MONGODB_URI" --file "$migration"
done
