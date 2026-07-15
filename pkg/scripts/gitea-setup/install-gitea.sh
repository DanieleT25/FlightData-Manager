#!/usr/bin/env bash
# install-gitea.sh
# Downloads the Gitea binary, creates a dedicated system user,
# and starts Gitea on port 3000.
#
# Run once on the host — no Docker required.
# After running, complete the setup wizard at http://localhost:3000
#
# Usage:
#   chmod +x install-gitea.sh
#   ./install-gitea.sh   # will prompt for sudo when needed

set -euo pipefail

GITEA_VERSION="1.26.0"
GITEA_BIN=$PWD/gitea
GITEA_DATA=$PWD/data
GITEA_CONF=$PWD/conf

if [[ ! -d "$GITEA_DATA" ]]; then
  echo "==> Creating Gitea data directory at ./${GITEA_DATA} ..."
  mkdir -p "$GITEA_DATA"
fi

if [[ ! -d "$GITEA_CONF" ]]; then
  echo "==> Creating Gitea config directory at ./${GITEA_CONF} ..."
  mkdir -p "$GITEA_CONF"
fi

if [[ ! -f "$GITEA_BIN" ]]; then
  echo "==> Gitea binary not found at ./${GITEA_BIN}"
  # https://dl.gitea.com/gitea/1.26.0/gitea-1.26.0-linux-amd64
    URL="https://dl.gitea.com/gitea/${GITEA_VERSION}/gitea-${GITEA_VERSION}-linux-amd64"
    echo "==> Downloading Gitea ${GITEA_VERSION}..."
    curl "${URL}" -o gitea
    chmod +x gitea
else
  echo "==> Gitea binary already exists at ./${GITEA_BIN}, skipping download."
fi

echo "==> Starting Gitea on http://localhost:3000 ..."
echo "    Open the setup wizard in your browser, then Ctrl+C when done."
"${GITEA_BIN}" web \
  --config "${GITEA_CONF}/app.ini" \
  --port 3000
