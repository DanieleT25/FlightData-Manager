#!/usr/bin/env bash
# bootstrap-aws.sh
# Creates the one-time AWS resources the GitHub Actions pipeline needs before
# any infrastructure can be provisioned:
#
#   1. S3 bucket for the OpenTofu remote state (versioned, encrypted, private)
#   2. GitHub OIDC identity provider  — lets Actions get temporary credentials
#   3. IAM role assumed by this repository's workflows via OIDC
#   4. Monthly cost budget with e-mail alerts
#
# None of these resources are billable in practice: S3 holds a few KB of state,
# IAM and OIDC are free, and the first two budgets are free. This script does
# NOT create any compute, network or database resource.
#
# The state bucket cannot be managed by OpenTofu itself (it must exist before
# `tofu init` can use it), which is why this is a plain AWS CLI script.
#
# Prerequisites:
#   - AWS CLI v2 already configured with an IAM user that has
#     AdministratorAccess (`aws sts get-caller-identity` must succeed)
#   - jq installed
#
# The script creates no IAM user and no CLI profile: it runs with whatever
# credentials the CLI resolves. Set AWS_PROFILE first to pick a named profile.
#
# Usage:
#   export ALERT_EMAIL=you@example.com   # only if a budget must be created
#   ./bootstrap-aws.sh
#
# The script is idempotent: re-running it updates the existing resources
# instead of failing.

set -euo pipefail

# ── Configuration ────────────────────────────────────────────────────────────

REGION="${AWS_REGION:-eu-south-1}"          # Milan
GITHUB_REPO="${GITHUB_REPO:-DanieleT25/FlightData-Manager}"
ROLE_NAME="${ROLE_NAME:-github-actions-flightdata}"
BUDGET_NAME="flightdata-manager-monthly"
BUDGET_LIMIT="${BUDGET_LIMIT:-200}"         # USD — matches the free-plan credit
ALERT_EMAIL="${ALERT_EMAIL:-}"

# Managed policies attached to the pipeline role. PowerUserAccess covers every
# service this project provisions (VPC, EKS, RDS, ElastiCache, ECR, S3,
# CloudFront); IAMFullAccess is required because EKS creates service-linked and
# node roles. Both are scoped to this repository by the OIDC trust policy below.
ROLE_POLICIES=(
  "arn:aws:iam::aws:policy/PowerUserAccess"
  "arn:aws:iam::aws:policy/IAMFullAccess"
)

BUDGET_REGION="eu-south-1"

OIDC_HOST="token.actions.githubusercontent.com"

# ── Preflight checks ─────────────────────────────────────────────────────────

if ! command -v aws &>/dev/null; then
  echo "ERROR: AWS CLI is required. Install with: brew install awscli"
  exit 1
fi

if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required. Install with: brew install jq"
  exit 1
fi

echo "==> Checking AWS credentials..."
if ! CALLER=$(aws sts get-caller-identity --output json 2>/dev/null); then
  echo "ERROR: Cannot authenticate with AWS."
  echo "       Configure the CLI first: aws configure --profile flightdata"
  echo "       Then: export AWS_PROFILE=flightdata"
  exit 1
fi

ACCOUNT_ID=$(echo "$CALLER" | jq -r '.Account')
CALLER_ARN=$(echo "$CALLER" | jq -r '.Arn')
echo "    Authenticated as: ${CALLER_ARN}"
echo "    Account: ${ACCOUNT_ID}  Region: ${REGION}"

if [[ "$CALLER_ARN" == *":root" ]]; then
  echo ""
  echo "WARNING: You are using root account credentials."
  echo "         Create a dedicated IAM user with AdministratorAccess instead,"
  echo "         enable MFA on root, and delete the root access key."
  echo ""
fi

# Bucket names are globally unique across all of AWS — the account id keeps
# this one collision-free without asking for a name.
BUCKET="flightdata-manager-tofu-state-${ACCOUNT_ID}"

# ── Decide what to do about the budget ───────────────────────────────────────
# An account that already has a cost budget already has the alarm this script
# would add, so a second one is not created unless explicitly requested. Note
# that AWS only provides the first two budgets free of charge.

EXISTING_BUDGETS=$(aws budgets describe-budgets \
  --account-id "$ACCOUNT_ID" \
  --region "$BUDGET_REGION" \
  --query 'Budgets[].BudgetName' \
  --output text 2>/dev/null || true)

if [[ "$EXISTING_BUDGETS" == *"$BUDGET_NAME"* ]]; then
  BUDGET_ACTION="update"
elif [[ -n "$EXISTING_BUDGETS" && -z "${FORCE_BUDGET:-}" ]]; then
  BUDGET_ACTION="skip"
else
  BUDGET_ACTION="create"
fi

if [[ "$BUDGET_ACTION" != "skip" && -z "$ALERT_EMAIL" ]]; then
  echo "ERROR: ALERT_EMAIL is not set."
  echo "       Budget alerts are the only warning before the credit runs out."
  echo "       Run: export ALERT_EMAIL=you@example.com"
  exit 1
fi

echo ""
echo "This will create:"
echo "  S3 bucket    ${BUCKET}"
echo "  OIDC provider ${OIDC_HOST}"
echo "  IAM role     ${ROLE_NAME}  (trusted by repo ${GITHUB_REPO})"
case "$BUDGET_ACTION" in
  create) echo "  Budget       ${BUDGET_NAME}  \$${BUDGET_LIMIT}/month → ${ALERT_EMAIL}" ;;
  update) echo "  Budget       ${BUDGET_NAME}  (already exists — limit will be updated)" ;;
  skip)   echo "  Budget       none — account already has: ${EXISTING_BUDGETS}"
          echo "               (set FORCE_BUDGET=1 to add a project-specific one)" ;;
esac
echo ""
read -r -p "Continue? [y/N] " REPLY
if [[ ! "$REPLY" =~ ^[Yy]$ ]]; then
  echo "Aborted."
  exit 0
fi

# ── 1. Create the OpenTofu state bucket ──────────────────────────────────────

echo ""
echo "==> Creating S3 bucket '${BUCKET}'..."
if aws s3api head-bucket --bucket "$BUCKET" 2>/dev/null; then
  echo "    Bucket already exists — skipping creation."
else
  # us-east-1 rejects LocationConstraint; every other region requires it.
  if [[ "$REGION" == "us-east-1" ]]; then
    aws s3api create-bucket --bucket "$BUCKET" --region "$REGION" > /dev/null
  else
    aws s3api create-bucket \
      --bucket "$BUCKET" \
      --region "$REGION" \
      --create-bucket-configuration "LocationConstraint=${REGION}" > /dev/null
  fi
  echo "    Created."
fi

# Versioning turns every `tofu apply` into a restorable state revision.
echo "==> Enabling bucket versioning..."
aws s3api put-bucket-versioning \
  --bucket "$BUCKET" \
  --versioning-configuration Status=Enabled
echo "    Done."

# State files contain resource attributes in clear text — encrypt at rest.
echo "==> Enabling default encryption..."
aws s3api put-bucket-encryption \
  --bucket "$BUCKET" \
  --server-side-encryption-configuration \
    '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'
echo "    Done."

echo "==> Blocking all public access..."
aws s3api put-public-access-block \
  --bucket "$BUCKET" \
  --public-access-block-configuration \
    "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true"
echo "    Done."

# NOTE: no DynamoDB lock table is created. OpenTofu >= 1.10 locks state natively
# in S3 via `use_lockfile = true` in the backend block; the `dynamodb_table`
# option is deprecated. If your OpenTofu is older, create the table with:
#   aws dynamodb create-table --table-name tofu-state-lock \
#     --attribute-definitions AttributeName=LockID,AttributeType=S \
#     --key-schema AttributeName=LockID,KeyType=HASH \
#     --billing-mode PAY_PER_REQUEST

# ── 2. Register GitHub as an OIDC identity provider ──────────────────────────

OIDC_ARN="arn:aws:iam::${ACCOUNT_ID}:oidc-provider/${OIDC_HOST}"

echo "==> Registering OIDC provider '${OIDC_HOST}'..."
if aws iam get-open-id-connect-provider --open-id-connect-provider-arn "$OIDC_ARN" &>/dev/null; then
  echo "    Provider already exists — skipping."
else
  # AWS no longer validates the thumbprint for this well-known provider, but
  # the API still expects the parameter.
  aws iam create-open-id-connect-provider \
    --url "https://${OIDC_HOST}" \
    --client-id-list "sts.amazonaws.com" \
    --thumbprint-list "6938fd4d98bab03faadb97b34396831e3780aea1" > /dev/null
  echo "    Created."
fi

# ── 3. Create the IAM role assumed by GitHub Actions ─────────────────────────

# `sub` is the OIDC claim identifying the workflow's origin. The wildcard allows
# any branch or pull request of this repository; tighten it to
# "repo:${GITHUB_REPO}:ref:refs/heads/main" to restrict deployments to main.
TRUST_POLICY=$(cat <<EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": { "Federated": "${OIDC_ARN}" },
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Condition": {
      "StringEquals": { "${OIDC_HOST}:aud": "sts.amazonaws.com" },
      "StringLike":   { "${OIDC_HOST}:sub": "repo:${GITHUB_REPO}:*" }
    }
  }]
}
EOF
)

echo "==> Creating IAM role '${ROLE_NAME}'..."
if aws iam get-role --role-name "$ROLE_NAME" &>/dev/null; then
  echo "    Role already exists — updating its trust policy."
  aws iam update-assume-role-policy \
    --role-name "$ROLE_NAME" \
    --policy-document "$TRUST_POLICY"
else
  aws iam create-role \
    --role-name "$ROLE_NAME" \
    --description "Assumed by GitHub Actions in ${GITHUB_REPO} via OIDC" \
    --assume-role-policy-document "$TRUST_POLICY" > /dev/null
  echo "    Created."
fi

for POLICY_ARN in "${ROLE_POLICIES[@]}"; do
  echo "==> Attaching $(basename "$POLICY_ARN")..."
  aws iam attach-role-policy --role-name "$ROLE_NAME" --policy-arn "$POLICY_ARN"
  echo "    Done."
done

ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${ROLE_NAME}"

# ── 4. Create the cost budget and its alerts ─────────────────────────────────

# IncludeCredit=false makes the budget measure gross spend *before* promotional
# credits are applied. With the default (true) the reported cost stays near zero
# while the credit is being consumed, which is precisely what must be tracked.
BUDGET_JSON=$(cat <<EOF
{
  "BudgetName": "${BUDGET_NAME}",
  "BudgetLimit": { "Amount": "${BUDGET_LIMIT}", "Unit": "USD" },
  "TimeUnit": "MONTHLY",
  "BudgetType": "COST",
  "CostTypes": {
    "IncludeCredit": false,
    "IncludeRefund": false,
    "IncludeDiscount": true,
    "IncludeTax": true,
    "IncludeSubscription": true,
    "IncludeSupport": true,
    "IncludeOtherSubscription": true,
    "IncludeUpfront": true,
    "IncludeRecurring": true,
    "UseBlended": false,
    "UseAmortized": false
  }
}
EOF
)

# Three actual-spend alerts plus one forecast alert, which fires as soon as the
# projected month-end cost crosses the limit — usually days before it happens.
NOTIFICATIONS_JSON=$(cat <<EOF
[
  { "Notification": { "NotificationType": "ACTUAL", "ComparisonOperator": "GREATER_THAN", "Threshold": 25, "ThresholdType": "PERCENTAGE" },
    "Subscribers": [{ "SubscriptionType": "EMAIL", "Address": "${ALERT_EMAIL}" }] },
  { "Notification": { "NotificationType": "ACTUAL", "ComparisonOperator": "GREATER_THAN", "Threshold": 50, "ThresholdType": "PERCENTAGE" },
    "Subscribers": [{ "SubscriptionType": "EMAIL", "Address": "${ALERT_EMAIL}" }] },
  { "Notification": { "NotificationType": "ACTUAL", "ComparisonOperator": "GREATER_THAN", "Threshold": 80, "ThresholdType": "PERCENTAGE" },
    "Subscribers": [{ "SubscriptionType": "EMAIL", "Address": "${ALERT_EMAIL}" }] },
  { "Notification": { "NotificationType": "FORECASTED", "ComparisonOperator": "GREATER_THAN", "Threshold": 100, "ThresholdType": "PERCENTAGE" },
    "Subscribers": [{ "SubscriptionType": "EMAIL", "Address": "${ALERT_EMAIL}" }] }
]
EOF
)

case "$BUDGET_ACTION" in
  skip)
    echo "==> Skipping budget creation — this account already has: ${EXISTING_BUDGETS}"
    echo "    Verify that it actually notifies someone:"
    echo "      aws budgets describe-notifications-for-budget \\"
    echo "        --account-id ${ACCOUNT_ID} --budget-name <name> --region ${BUDGET_REGION}"
    echo "    A budget with no subscribers is silent and warns nobody."
    ;;
  update)
    echo "==> Updating budget '${BUDGET_NAME}' (\$${BUDGET_LIMIT}/month)..."
    aws budgets update-budget \
      --account-id "$ACCOUNT_ID" \
      --new-budget "$BUDGET_JSON" \
      --region "$BUDGET_REGION"
    echo "    Done."
    ;;
  create)
    echo "==> Creating budget '${BUDGET_NAME}' (\$${BUDGET_LIMIT}/month)..."
    aws budgets create-budget \
      --account-id "$ACCOUNT_ID" \
      --budget "$BUDGET_JSON" \
      --notifications-with-subscribers "$NOTIFICATIONS_JSON" \
      --region "$BUDGET_REGION"
    echo "    Created — confirm the subscription in the e-mail AWS just sent."
    ;;
esac

# ── 5. Print next steps ──────────────────────────────────────────────────────

cat <<EOF

==> Bootstrap complete.

Add this secret to the GitHub repository:

  Settings → Secrets and variables → Actions → New repository secret

    AWS_ROLE_ARN = ${ROLE_ARN}

Values needed by the OpenTofu backend in the next phase:

    bucket = "${BUCKET}"
    region = "${REGION}"
EOF
