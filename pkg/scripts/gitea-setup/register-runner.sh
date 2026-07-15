#!/usr/bin/env bash
# register-runner.sh
# Downloads act_runner, registers it as a self-hosted shell runner on the host,
# and starts it.
#
# The runner runs directly on the host (not in Docker) so it can access
# Multipass, tofu, and ansible — consistent with how those tools are installed.
#
# Prerequisites:
#   - Gitea running at http://localhost:3000 (see install-gitea.sh)
#   - .env file with RUNNER_TOKEN set
#   - tofu, multipass, ansible installed on this host
#
# Usage:
#   cp .env.example .env
#   # edit .env and set RUNNER_TOKEN
#   chmod +x register-runner.sh
#   ./register-runner.sh

set -euo pipefail

# 	act_runner-0.4.1-linux-amd64
ACT_RUNNER_VERSION="0.4.1"
ACT_RUNNER_BIN="./act_runner"
GITEA_URL="http://localhost:3000"


if [[ -z "${GITEA_TOKEN:-}" ]]; then
  echo "ERROR: GITEA_TOKEN is not set in the environment"
  exit 1
fi

# Download act_runner to current directory if not present
if [[ ! -f "$ACT_RUNNER_BIN" ]]; then
  echo "==> Downloading act_runner..."
  curl -sSfL \
    "https://dl.gitea.com/act_runner/${ACT_RUNNER_VERSION}/act_runner-${ACT_RUNNER_VERSION}-darwin-arm64" \
    -o "$ACT_RUNNER_BIN"
  chmod +x "$ACT_RUNNER_BIN"
fi

# Register (writes .runner config file in the current directory)
echo "==> Registering runner with Gitea at ${GITEA_URL}..."
"$ACT_RUNNER_BIN" register \
  --instance "$GITEA_URL" \
  --token    "$GITEA_TOKEN" \
  --name     "host-runner" \
  --labels   "self-hosted,linux,multipass" \
  --no-interactive

echo "==> Starting runner (Ctrl+C to stop)..."
"$ACT_RUNNER_BIN" daemon
