# AWS Deployment (GitHub Actions + OpenTofu + AWS)

The cloud deployment mirrors the [local Kubernetes deployment](kubernetes-deployment.md), replacing the self-hosted pieces with managed services:

| Concern | Local | AWS |
|---|---|---|
| Git host & CI | Gitea + `act_runner` on the laptop | GitHub + hosted runners |
| Credentials | Gitea secrets (static) | OIDC — temporary, no stored keys |
| State | `~/k8s-terraform.tfstate` on disk | S3 bucket, versioned and locked |
| Infrastructure | Multipass VMs + kubeadm | VPC + EKS |
| Postgres / Redis | StatefulSets in-cluster | RDS / ElastiCache |
| Neo4j | StatefulSet in-cluster | Neo4j Aura (external) |
| Frontend | nginx pod | S3 + CloudFront |

The two tracks live side by side on the same branch and never interfere: Gitea only reads `.gitea/workflows/`, GitHub only reads `.github/workflows/`.

## Cost model — read this first

The AWS free plan grants a fixed credit (typically $100, up to $200 after completing the onboarding tasks). Two properties of that credit drive every decision below:

- **Nothing is blocked.** Services outside the always-free tier — EKS control plane, NAT Gateway — bill normally; the charge is silently deducted from the credit instead of the card.
- **When the credit runs out the account is closed**, unless it was upgraded to the paid plan first, in which case billing continues on the card.

Cost is therefore a function of *uptime*, not of the architecture. Approximate hourly rates, single-AZ:

| Resource | ~USD/hour |
|---|---|
| EKS control plane | 0.10 |
| NAT Gateway | 0.045 |
| 2 × `t3.small` worker nodes | 0.042 |
| RDS + ElastiCache (`micro`, free tier) | ~0 |
| **Total** | **~0.19** |

That is roughly **$4.50 per day left running**, against **~$0.60 for a three-hour working session** followed by a destroy. The credit covers hundreds of hours of deliberate use, and about six weeks of forgetting.

Two consequences shape the phases below: the budget alarm is created *before* any billable resource, and the destroy workflow is written *before* the deploy workflow.

## Phases

| Phase | Contents | Billable |
|---|---|---|
| **0** | AWS account, OIDC, state bucket, budget alerts | no |
| 1 | `aws/terraform/` skeleton, `plan`/`apply`/`destroy` workflows | no |
| 2 | VPC, subnets, Internet Gateway, single NAT Gateway | NAT |
| 3 | RDS Postgres, ElastiCache Redis, Neo4j Aura, Secrets Manager | free tier |
| 4 | ECR, EKS + node group, adapted `k8s/` manifests | EKS + nodes |
| 5 | S3 + CloudFront frontend, API Gateway, Cognito | low |
| 6 | Observability, documentation, final diagram | no |

---

## Phase 0 — Prerequisites

Everything in this phase is free: it creates an S3 bucket holding a few KB of state, IAM objects, and a budget. No compute, network or database resource is provisioned.

### 1. Secure the account

In the AWS console:

1. **Billing and Cost Management → Free tier** — confirm the plan and the remaining credit.
2. **IAM → Enable MFA** on the root account.
3. **IAM → Users → Create user** — name it e.g. `flightdata-admin`, attach `AdministratorAccess`, then **Security credentials → Create access key → CLI use case**.

Root credentials must never be used for day-to-day work or in a pipeline.

### 2. Configure the AWS CLI

Skip this step if the CLI is already configured — `aws configure list-profiles` shows the existing profiles, and the scripts use whatever credentials the CLI resolves by default.

Otherwise:

```bash
aws configure
```

Answer with the access key from the previous step, `eu-south-1` as region, and `json` as output format. Verify:

```bash
aws sts get-caller-identity
```

This must succeed before anything else — every `aws` and `tofu` command depends on it. To keep these credentials in a named profile instead of the default one, use `aws configure --profile <name>` and `export AWS_PROFILE=<name>` before running anything below.

> [!note]
> `eu-south-1` (Milan) is the default region used throughout this project. In that region the free-tier EC2 instance type is `t3.micro`; `t2.micro` is not available.

### 3. Run the bootstrap script

```bash
export ALERT_EMAIL=<your-email>   # only required if a budget has to be created
bash pkg/scripts/aws-setup/bootstrap-aws.sh
```

It prints a summary and asks for confirmation before creating anything. It is idempotent — re-running updates the existing resources. It creates:

| Resource | Purpose |
|---|---|
| S3 bucket `flightdata-manager-tofu-state-<account-id>` | OpenTofu remote state — versioned, encrypted, private |
| OIDC provider `token.actions.githubusercontent.com` | Lets GitHub Actions request temporary AWS credentials |
| IAM role `github-actions-flightdata` | Assumed by this repository's workflows, `PowerUserAccess` + `IAMFullAccess` |
| Budget `flightdata-manager-monthly` | Alerts at 25%, 50%, 80% of spend and on a forecast overrun |

AWS sends a subscription confirmation e-mail for the budget alerts — **confirm it**, otherwise the alerts never arrive.

**If the account already has a budget**, the script detects it and creates no second one (AWS only provides the first two budgets free of charge). In that case, check that the existing budget actually notifies someone — a budget without subscribers is silent:

```bash
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
aws budgets describe-notifications-for-budget \
  --account-id "$ACCOUNT_ID" --budget-name <name> --region eu-south-1
```

Set `FORCE_BUDGET=1` to add the project-specific budget anyway.

> [!important]
> A cost budget must exclude credits to be useful here, otherwise the reported spend stays near zero while the credit drains — precisely the information that needs to be visible. The script sets `IncludeCredit: false`; an existing budget does the same if its filter excludes the `Credit` and `Refund` record types.

The state bucket is created with the CLI rather than OpenTofu because it must exist before `tofu init` can use it.

### 4. Store the role ARN in GitHub

The script prints the role ARN. Add it to the repository:

```
Settings → Secrets and variables → Actions → New repository secret

  AWS_ROLE_ARN = arn:aws:iam::<account-id>:role/github-actions-flightdata
```

This is the only AWS secret the pipeline needs — no access key is ever stored on GitHub. The workflow exchanges its OIDC token for credentials that expire after about an hour and are scoped to this repository by the role's trust policy.

### 5. Verify

```bash
# the state bucket exists and lives in the project region
aws s3 ls | grep tofu-state
aws s3api get-bucket-location --bucket "flightdata-manager-tofu-state-$(aws sts get-caller-identity --query Account --output text)"

# GitHub is registered as an identity provider
aws iam list-open-id-connect-providers

# the role exists and only this repository can assume it
aws iam get-role --role-name github-actions-flightdata --query 'Role.Arn' --output text
aws iam get-role --role-name github-actions-flightdata --query 'Role.AssumeRolePolicyDocument.Statement[0].Condition' --output json
```

The last command must show `repo:DanieleT25/FlightData-Manager:*` — that condition is what prevents any other repository from assuming the role.

> [!note]
> All project resources live in `eu-south-1`. IAM and Budgets are global services: they have no regional endpoint, so the `--region` flag is irrelevant for them and any value works.

### About state locking

The backend configured in phase 1 uses OpenTofu's native S3 locking (`use_lockfile = true`, available since OpenTofu 1.10) instead of a DynamoDB lock table. The `dynamodb_table` backend option is deprecated. If an older OpenTofu is required, create the table as described in the comment inside `bootstrap-aws.sh` and swap the backend option.

---

## Phase 1 — Pipeline skeleton

Validates the whole chain — OIDC, remote state, locking, apply and destroy — while it is still free to get wrong. The only resource created is a **Resource Group**, which costs nothing and afterwards shows every resource of the project as one group in the console.

### Layout

| Path | Role |
|---|---|
| `aws/terraform/providers.tf` | Provider, default tags, S3 backend |
| `aws/terraform/variables.tf` | Region, project name, environment |
| `aws/terraform/main.tf` | Tags, data sources, resource group |
| `aws/terraform/outputs.tf` | Region, availability zones, group name |
| `.github/workflows/aws-infra.yml` | `plan` on pull requests, `plan`/`apply` on demand |
| `.github/workflows/aws-destroy.yml` | Manual teardown |

The local track under `terraform/` is untouched; the two never share state or workflows.

### Second secret

Add it next to `AWS_ROLE_ARN`:

```
TF_STATE_BUCKET = flightdata-manager-tofu-state-<account-id>
```

The backend block in `providers.tf` is empty by design: the bucket name embeds the AWS account id and this repository is public, so it is injected at `init` time with `-backend-config` rather than committed. This mirrors what `infra.yml` already does locally with `-backend-config="path=..."`.

### Running it

```
Actions → AWS — Infrastructure → Run workflow → action: plan
```

`plan` is read-only and can be run freely. When the output looks right, run it again with `action: apply`. Teardown is a separate workflow that requires typing `destroy` as confirmation:

```
Actions → AWS — Destroy → Run workflow → confirm: destroy
```

Opening a pull request that touches `aws/terraform/**` also runs `plan` and posts the diff as a comment, so no infrastructure change reaches `main` unreviewed.

### Why `apply` is never automatic

The local pipeline applies on every push because a wrong Multipass VM costs nothing. On AWS the same mistake bills by the second, so `apply` only runs from an explicit dispatch, and its job targets the `production` environment — adding *required reviewers* to that environment in the repository settings makes every apply wait for a human approval.

### Choices worth knowing

- **The plan is not uploaded as an artifact.** Artifacts of a public repository can be downloaded by anyone, and a plan file contains resource attributes in clear text — harmless now, not from phase 3 with database credentials. The `apply` job re-plans internally instead.
- **Pull requests from forks get no secrets.** The credentials step fails and the run stops before reaching AWS. That is the intended behaviour, and the reason the workflow uses `pull_request` and never `pull_request_target`, which does expose secrets to fork code.
- **`.terraform.lock.hcl` is committed**, with checksums for both `linux_amd64` (the runner) and `darwin_arm64` (the laptop). A lock file recorded for one platform only makes `tofu init` fail on the other. Refresh it with:

  ```bash
  tofu -chdir=aws/terraform providers lock -platform=linux_amd64 -platform=darwin_arm64
  ```

- **Everything is tagged** through `default_tags`, which drives both the resource group and per-project cost breakdown in Cost Explorer.

### Verify

After `apply`, the run summary shows the outputs. Then confirm the state landed in S3:

```bash
aws s3 ls "s3://flightdata-manager-tofu-state-$(aws sts get-caller-identity --query Account --output text)/aws/"
```

Run **AWS — Destroy** and check the summary reports an empty state list. At that point the pipeline is proven and phase 2 can add real infrastructure.

---

## Phase 2 — Network

First phase that costs money. Everything here exists only while the infrastructure is up, so the working rule from now on is: **run the destroy workflow at the end of every session**.

### Layout

Two availability zones, three subnet tiers in each:

| Tier | CIDR (zone a / zone b) | Route to the internet | Will host |
|---|---|---|---|
| public | `10.0.0.0/24` / `10.0.1.0/24` | Internet Gateway | NAT Gateway, public load balancer |
| private app | `10.0.16.0/20` / `10.0.32.0/20` | NAT Gateway | EKS nodes |
| private data | `10.0.48.0/24` / `10.0.49.0/24` | **none** | RDS, ElastiCache |

The application tier is a `/20` because the EKS VPC CNI gives every *pod* a real VPC address, not just every node — a `/24` runs out quickly. The other two tiers hold a handful of interfaces each.

The data tier has no default route at all: those databases are reachable from inside the VPC and can reach nothing outside it. It is one less path to worry about, and it keeps database traffic off the NAT Gateway.

### The shared NAT Gateway

A NAT Gateway costs about `$0.045/hour` plus `$0.045/GB` processed, and since February 2024 its public IPv4 address adds about `$0.005/hour`. One per zone — the textbook layout — would double that standing cost for the whole life of the project.

Both zones therefore share a single gateway, placed in the first zone. The consequence is explicit: **an outage of the first zone also cuts outbound traffic for the second one**. Zone redundancy still protects the compute and database tiers, but not egress. In production each zone gets its own gateway; here the saving is worth more than the redundancy.

### S3 gateway endpoint

Free, and it removes a recurring charge: container image layers pulled from ECR are actually served from S3, and everything crossing a NAT Gateway is billed per GB. Routing S3 traffic through the endpoint keeps image pulls off the gateway entirely.

### Cost

| Resource | ~USD/hour |
|---|---|
| NAT Gateway | 0.045 |
| Public IPv4 of the NAT Gateway | 0.005 |
| VPC, subnets, route tables, Internet Gateway, S3 endpoint | free |
| **Total** | **~0.05** |

About `$1.20` per day if left running, `$0.15` for a three-hour session.

### Verify

After `apply`, the outputs list the VPC, the three subnet groups and the NAT address. From the console, **VPC → Resource map** draws the whole topology: it should show two zones, six subnets, and every private subnet routed to the same gateway.

```bash
aws ec2 describe-subnets --region eu-south-1 \
  --filters "Name=tag:Project,Values=flightdata-manager" \
  --query 'Subnets[].{Name:Tags[?Key==`Name`]|[0].Value,CIDR:CidrBlock,AZ:AvailabilityZone}' \
  --output table
```
