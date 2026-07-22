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
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

case "$(uname -s)" in
  Darwin) GITEA_OS="darwin" ;;
  Linux)  GITEA_OS="linux" ;;
  *)
    echo "ERROR: unsupported operating system: $(uname -s)"
    exit 1
    ;;
esac

case "$(uname -m)" in
  arm64|aarch64) GITEA_ARCH="arm64" ;;
  x86_64|amd64)  GITEA_ARCH="amd64" ;;
  *)
    echo "ERROR: unsupported CPU architecture: $(uname -m)"
    exit 1
    ;;
esac

GITEA_BIN="$SCRIPT_DIR/gitea"
GITEA_DATA="$SCRIPT_DIR/data"
GITEA_CONF="$SCRIPT_DIR/conf"

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
    URL="https://dl.gitea.com/gitea/${GITEA_VERSION}/gitea-${GITEA_VERSION}-${GITEA_OS}-${GITEA_ARCH}"
    echo "==> Downloading Gitea ${GITEA_VERSION} for ${GITEA_OS}/${GITEA_ARCH}..."
    curl -fsSL "${URL}" -o "$GITEA_BIN"
    chmod +x "$GITEA_BIN"
else
  echo "==> Gitea binary already exists at ./${GITEA_BIN}, skipping download."
fi

echo "==> Starting Gitea on http://localhost:3000 ..."
echo "    Open the setup wizard in your browser, then Ctrl+C when done."
"${GITEA_BIN}" web \
  --config "${GITEA_CONF}/app.ini" \
  --port 3000
