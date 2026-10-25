#!/usr/bin/env bash
# Writes the website data from your logs and pushes it, which triggers the GitHub Pages build.
#   ./scripts/publish.sh [YYYY-MM-DD]
set -euo pipefail
if [ "${1:-}" != "" ]; then go run ./cmd/export -date "$1"; else go run ./cmd/export; fi
git add site/public/data/picks.json
if git diff --cached --quiet; then echo "No changes to publish."; exit 0; fi
git commit -m "Update picks $(date '+%Y-%m-%d %H:%M')"
git push
echo "Pushed. The site updates in about a minute (see the Actions tab)."
