#!/usr/bin/env bash
# register-runner.sh
# Downloads act_runner, registers it as a host-executed self-hosted runner,
# and starts it.
#
# The runner runs directly on the host (not in Docker) so it can access
# Multipass, tofu, and ansible — consistent with how those tools are installed.
#
# Prerequisites:
#   - Gitea running at http://localhost:3000 (see install-gitea.sh)
#   - .env file with GITEA_RUNNER_TOKEN set
#   - tofu, multipass, ansible installed on this host
#
# Usage:
#   cp .env.example .env
#   # edit .env and set RUNNER_TOKEN
#   chmod +x register-runner.sh
#   ./register-runner.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 	act_runner-0.4.1-linux-amd64
ACT_RUNNER_VERSION="0.4.1"
ACT_RUNNER_BIN="$SCRIPT_DIR/act_runner"
GITEA_URL="http://localhost:3000"

case "$(uname -s)" in
  Darwin) RUNNER_OS="darwin" ;;
  Linux)  RUNNER_OS="linux" ;;
  *)
    echo "ERROR: unsupported operating system: $(uname -s)"
    exit 1
    ;;
esac

case "$(uname -m)" in
  arm64|aarch64) RUNNER_ARCH="arm64" ;;
  x86_64|amd64)  RUNNER_ARCH="amd64" ;;
  *)
    echo "ERROR: unsupported CPU architecture: $(uname -m)"
    exit 1
    ;;
esac


if [[ -z "${GITEA_RUNNER_TOKEN:-}" ]]; then
  echo "ERROR: GITEA_RUNNER_TOKEN is not set in the environment"
  exit 1
fi

# Download act_runner to current directory if not present
if [[ ! -f "$ACT_RUNNER_BIN" ]]; then
  echo "==> Downloading act_runner..."
  curl -sSfL \
    "https://dl.gitea.com/act_runner/${ACT_RUNNER_VERSION}/act_runner-${ACT_RUNNER_VERSION}-${RUNNER_OS}-${RUNNER_ARCH}" \
    -o "$ACT_RUNNER_BIN"
  chmod +x "$ACT_RUNNER_BIN"
fi

# Register (writes .runner config file in the current directory)
echo "==> Registering runner with Gitea at ${GITEA_URL}..."
"$ACT_RUNNER_BIN" register \
  --instance "$GITEA_URL" \
  --token    "$GITEA_RUNNER_TOKEN" \
  --name     "host-runner" \
  --labels   "self-hosted:host,linux:host,multipass:host" \
  --no-interactive

echo "==> Starting runner (Ctrl+C to stop)..."
"$ACT_RUNNER_BIN" daemon
