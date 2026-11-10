#!/usr/bin/env bash
# Publish authoritative saved state. Source/data pushes trigger the site workflow.
set -euo pipefail
if [ "${1:-}" != "" ]; then go run ./cmd/export -date "$1"; else go run ./cmd/export; fi
branch=$(git branch --show-current)
[ -n "$branch" ] || { echo "Check out your branch before publishing."; exit 1; }
python3 scripts/sync_state.py --branch "$branch" --message "Publish saved records"
echo "Saved records synchronized. New commits trigger the site workflow on the default branch."
