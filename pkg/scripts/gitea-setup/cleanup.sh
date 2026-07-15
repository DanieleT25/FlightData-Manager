#!/usr/bin/env bash
# cleanup.sh
# Stops Gitea and act_runner, then removes all runtime data so you can
# start from scratch with install-gitea.sh.
#
# What gets removed:
#   - Running gitea and act_runner processes
#   - conf/   (app.ini, certificates)
#   - data/   (SQLite DB, repositories, LFS, attachments)
#   - log/    (gitea and runner logs)
#   - .runner  (runner registration)
#   - gitea    (binary)
#   - act_runner (binary)
#
# Usage:
#   cd lab/gitea-setup
#   ./cleanup.sh

set -euo pipefail

cd "$(dirname "$0")"

echo "==> Stopping act_runner..."
pkill -f "act_runner daemon" 2>/dev/null && echo "    act_runner stopped." || echo "    act_runner was not running."

echo "==> Stopping gitea..."
pkill -f "gitea web" 2>/dev/null && echo "    gitea stopped." || echo "    gitea was not running."

# Give processes a moment to exit
sleep 1

echo ""
echo "WARNING: This will permanently delete all Gitea data:"
echo "  conf/  data/  log/  .runner  gitea  act_runner"
echo ""
read -r -p "Are you sure? [y/N] " CONFIRM
if [[ "${CONFIRM,,}" != "y" ]]; then
  echo "Aborted."
  exit 0
fi

echo "==> Removing runtime directories and files..."
rm -rf conf/ data/ log/ .runner gitea act_runner

echo ""
echo "Done. Run ./install-gitea.sh to start fresh."
