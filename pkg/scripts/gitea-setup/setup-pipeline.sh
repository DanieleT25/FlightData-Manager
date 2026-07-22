#!/usr/bin/env bash
# setup-pipeline.sh
# Creates the Gitea repository and uploads the SSH key pair as Actions secrets.
#
# Prerequisites:
#   - Gitea running at http://localhost:3000 (see install-gitea.sh)
#   - act_runner registered and running (see register-runner.sh)
#   - SSH key pair already generated:
#       ssh-keygen -t ed25519 -f terraform/id_ed25519 -N ""
#
# Usage:
#   GITEA_TOKEN=<personal-access-token> ./setup-pipeline.sh
#
# Get a Personal Access Token (NOT the runner token) from:
#   Gitea UI → top-right avatar → Settings → Applications
#   → Token Name: anything
#   → Permissions:
#       repository → Read and Write
#       user       → Read and Write
#   → Generate Token
#
# The runner token (from Site Administration → Actions → Runners)
# is only used by register-runner.sh — it cannot authenticate API calls.

set -euo pipefail

GITEA_URL="http://localhost:3000"
REPO_NAME="FlightData-Manager"
SSH_PRIVATE_KEY_FILE="./terraform/id_ed25519"
SSH_PUBLIC_KEY_FILE="./terraform/id_ed25519.pub"

set_secret() {
  local name="$1"
  local value="${!name:-}"

  if [[ -z "$value" && "$name" != "REDIS_PASSWORD" ]]; then
    echo "ERROR: ${name} is not set in the environment."
    exit 1
  fi

  echo "==> Setting secret ${name}..."
  curl -sSf \
    "${AUTH[@]}" \
    -X PUT "${API}/repos/${GITEA_USER}/${REPO_NAME}/actions/secrets/${name}" \
    -d "{\"data\":$(printf %s "$value" | jq -Rs .)}" \
    > /dev/null
  echo "    Done."
}

if [[ -z "${GITEA_TOKEN:-}" ]]; then
  echo "ERROR: GITEA_TOKEN is not set."
  echo "       This must be a Personal Access Token (NOT the runner registration token)."
  echo "       Create one at: ${GITEA_URL}/user/settings/applications"
  exit 1
fi

if [[ ! -f "$SSH_PRIVATE_KEY_FILE" || ! -f "$SSH_PUBLIC_KEY_FILE" ]]; then
  echo "ERROR: SSH key pair not found."
  echo "       Run: ssh-keygen -t ed25519 -f ./terraform/id_ed25519 -N \"\""
  exit 1
fi

if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required. Install with: sudo apt install jq"
  exit 1
fi

API="${GITEA_URL}/api/v1"
AUTH=(-H "Authorization: token ${GITEA_TOKEN}" -H "Content-Type: application/json")

# ── 1. Check Gitea is reachable and token is valid ───────────────────────────
echo "==> Checking Gitea API..."
HTTP=$(curl -s -o /dev/null -w "%{http_code}" "${AUTH[@]}" "${API}/user")
if [[ "$HTTP" != "200" ]]; then
  echo "ERROR: Cannot reach Gitea API (HTTP ${HTTP})."
  echo "       Is Gitea running at ${GITEA_URL}?"
  echo "       GITEA_TOKEN must be a Personal Access Token with these scopes:"
  echo "         issue (Read/Write), repository (Read/Write), user (Read)"
  echo "       Create one at: ${GITEA_URL}/user/settings/applications"
  exit 1
fi
echo "    OK — Gitea is up and token is valid."
GITEA_USER=$(curl -s "${AUTH[@]}" "${API}/user" | jq -r '.login')
echo "    Authenticated as: ${GITEA_USER}"
echo ""
echo "NOTE: Ensure Actions are enabled before pushing code:"
echo "      Site Administration → Configuration → Actions → Enable"
echo ""

# ── 2. Create repository ──────────────────────────────────────────────────────
echo "==> Creating repository '${REPO_NAME}'..."
HTTP=$(curl -s -o /dev/null -w "%{http_code}" \
  "${AUTH[@]}" \
  -X POST "${API}/user/repos" \
  -d "{\"name\":\"${REPO_NAME}\",\"private\":false,\"auto_init\":false}")

if [[ "$HTTP" == "201" ]]; then
  echo "    Repository created: ${GITEA_URL}/${GITEA_USER}/${REPO_NAME}"
elif [[ "$HTTP" == "409" ]]; then
  echo "    Repository already exists — skipping."
else
  echo "ERROR: Unexpected HTTP ${HTTP} when creating repository."
  echo "       Ensure your PAT has: repository → Read/Write AND user → Read/Write"
  echo "       Recreate it at: ${GITEA_URL}/user/settings/applications"
  exit 1
fi

# ── 3. Enable Actions on the repository via API ─────────────────────────────
echo "==> Enabling Actions on repository '${REPO_NAME}'..."
curl -sSf \
  "${AUTH[@]}" \
  -X PATCH "${API}/repos/${GITEA_USER}/${REPO_NAME}" \
  -d '{"has_actions":true}' \
  > /dev/null
echo "    Done."

# ── 4. Set SSH_PRIVATE_KEY secret ───────────────────────────────────────────
echo "==> Setting secret SSH_PRIVATE_KEY..."
PRIVATE_KEY=$(cat "$SSH_PRIVATE_KEY_FILE")
curl -sSf \
  "${AUTH[@]}" \
  -X PUT "${API}/repos/${GITEA_USER}/${REPO_NAME}/actions/secrets/SSH_PRIVATE_KEY" \
  -d "{\"data\":$(echo "$PRIVATE_KEY" | jq -Rs .)}" \
  > /dev/null
echo "    Done."

# ── 5. Set SSH_PUBLIC_KEY secret ────────────────────────────────────────────
echo "==> Setting secret SSH_PUBLIC_KEY..."
PUBLIC_KEY=$(cat "$SSH_PUBLIC_KEY_FILE")
curl -sSf \
  "${AUTH[@]}" \
  -X PUT "${API}/repos/${GITEA_USER}/${REPO_NAME}/actions/secrets/SSH_PUBLIC_KEY" \
  -d "{\"data\":$(echo "$PUBLIC_KEY" | jq -Rs .)}" \
  > /dev/null
echo "    Done."

# ── 6. Set application runtime secrets ──────────────────────────────────────
# REDIS_PASSWORD may intentionally be empty for the local development cluster.
set_secret NEO4J_PASSWORD
set_secret REDIS_PASSWORD
set_secret OPENSKY_USER
set_secret OPENSKY_PASSWORD
set_secret GRAFANA_PASSWORD

# ── 7. Print next steps ─────────────────────────────────────────────────────
cat <<EOF

==> Setup complete!

Next steps (run from the repository root):
  git init
  git branch -M main
  git remote add gitea ${GITEA_URL}/${GITEA_USER}/${REPO_NAME}.git
  git add .
  git commit -m "initial commit"
  git push gitea main

Then watch the pipeline at:
  ${GITEA_URL}/${GITEA_USER}/${REPO_NAME}/actions

For the first deployment, run the "Provision K8s Cluster" workflow and then
the "Deploy to Kubernetes" workflow from the Actions page.
EOF
