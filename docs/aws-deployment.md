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

## Tearing down without GitHub

The destroy workflow is convenient, not essential. If Actions is queued, degraded or unreachable, the same teardown runs from a laptop, because none of the three things it needs lives on GitHub: the state is in S3, the credentials come from the local AWS CLI, and OpenTofu is installed locally.

```bash
tofu -chdir=aws/terraform init -reconfigure \
  -backend-config="bucket=flightdata-manager-tofu-state-<account-id>" \
  -backend-config="key=aws/terraform.tfstate" \
  -backend-config="region=eu-south-1"

tofu -chdir=aws/terraform destroy
```

There are therefore three independent ways out, in decreasing order of convenience: the workflow, the commands above, and deleting resources by hand in the console — where the `flightdata-manager-lab` resource group lists everything carrying the project tag, which is what it exists for.

### Confirming the account is actually empty

`tofu destroy` removes what OpenTofu created. It cannot remove what it never knew about — a load balancer or a volume created by Kubernetes itself, or anything left by an interrupted run. And the console resource group is an eventually consistent index that keeps listing deleted resources for hours, so it cannot settle the question either.

```bash
bash pkg/scripts/aws-setup/check-orphans.sh
```

It asks the service APIs directly, covers both what OpenTofu manages and what it cannot see, and exits non-zero if anything is still up. What it deliberately ignores is the handful of resources that must survive between sessions: the state bucket, the IAM role and OIDC provider, the budget, and the three Neo4j parameters.

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

---

## Phase 3 — Data layer

Fills the isolated `data` subnets with managed equivalents of the StatefulSets the local cluster runs:

| Local (Kubernetes) | AWS | Holds |
|---|---|---|
| Redis StatefulSet | ElastiCache, `cache.t3.micro` | idempotency keys of user-manager |
| PostgreSQL StatefulSet | RDS, `db.t4g.micro` | user records of user-manager |
| Neo4j StatefulSet | **Neo4j Aura**, outside AWS | flights, airports, interests |

14 resources: two security groups, the two engines with their subnet groups, a generated password and seven Parameter Store entries.

### Neo4j stays outside

AWS has no managed Neo4j, and running it in the cluster would need more memory than a lab node group can spare. Aura's free instance is enough — with two caveats worth knowing: it is **deleted after 30 days of inactivity**, and the free plan has no IP filtering, so the credentials are the only thing protecting it.

Those credentials are written to Parameter Store **by hand**, once:

```bash
aws ssm put-parameter --name "/flightdata-manager/lab/neo4j/uri"      --type String       --value "neo4j+s://<id>.databases.neo4j.io" --overwrite --region eu-south-1
aws ssm put-parameter --name "/flightdata-manager/lab/neo4j/user"     --type String       --value "<user>"     --overwrite --region eu-south-1
aws ssm put-parameter --name "/flightdata-manager/lab/neo4j/password" --type SecureString --value '<password>' --overwrite --region eu-south-1
```

They never pass through OpenTofu, so they never reach the state file.

### Secrets and the state file

The RDS master password is different: OpenTofu generates it, which means it **is** written to the state in clear text. That is inherent to how Terraform and OpenTofu work, not a flaw in this configuration, and it is why the state bucket is private, encrypted and versioned. The same value is published to Parameter Store as a `SecureString` so that the deployment can read it without anyone ever handling it.

Parameter Store rather than Secrets Manager: both would work, but Secrets Manager costs $0.40 per secret per month while the standard tier of Parameter Store is free, and the only feature lost — automatic rotation — is not used here.

### Free-tier sizing

| Setting | Value | Reason |
|---|---|---|
| RDS class | `db.t4g.micro` | free tier, 750 h/month |
| RDS storage | 20 GB `gp2` | free tier limit |
| Multi-AZ | off | not covered by the free tier |
| Backup retention | 0 days | the environment is destroyed after every session; backups would only add storage cost and slow the teardown |
| Final snapshot | skipped | otherwise every destroy leaves a billable snapshot behind |
| ElastiCache | `cache.t3.micro`, 1 node | free tier |
| Redis AUTH | off | needs a replication group and a larger node; access is restricted by the security group and by the subnet having no route out |

If the free tier does not apply, the two engines together add roughly `$0.04/hour`, taking the total to about `$0.09/hour`. Check `Billing → Cost Explorer` after the first session to see which case applies to this account.

### Divergence from the target architecture

The reference diagram in `schema/` draws RDS and ElastiCache in **both** availability zones. That notation does not mean two independent databases: it depicts a Multi-AZ deployment — one logical database with a standby replica in the second zone, behind a single endpoint — and, for Redis, a replication group with a primary and a replica.

This deployment runs **one instance of each** instead, for the same reason the two zones share one NAT Gateway: the free tier covers neither an RDS standby nor a second cache node, so high availability would cost about `$0.072/hour` where the current setup costs nothing.

What is actually given up: if the first availability zone fails, the database and the cache become unavailable until AWS restores it, whereas a Multi-AZ deployment would fail over automatically in a minute or two. Data itself is not at risk — RDS storage is replicated inside the zone, and the Redis contents are idempotency keys that regenerate on their own.

Restoring the diagram's design is deliberately kept cheap:

- **RDS** — set `multi_az = true`. One line, and AWS builds the standby on the next apply.
- **Redis** — replace `aws_elasticache_cluster` with `aws_elasticache_replication_group`, which is a different resource with a different schema, and add a replica in the second zone.

The zone-redundant part of the network — six subnets across two zones — is already in place, so nothing else would need to change.

### Expect a slow apply

RDS takes five to ten minutes to become available and ElastiCache about five, so this apply is far longer than the previous ones. The destroy is slow for the same reason.

### Verify

Both engines are unreachable from outside the VPC by design, so there is nothing to connect to from a laptop — that is the expected result, not a failure. What can be checked:

```bash
aws rds describe-db-instances --region eu-south-1 \
  --query 'DBInstances[].{Id:DBInstanceIdentifier,Status:DBInstanceStatus,Public:PubliclyAccessible,MultiAZ:MultiAZ}' --output table

aws elasticache describe-cache-clusters --region eu-south-1 --show-cache-node-info \
  --query 'CacheClusters[].{Id:CacheClusterId,Status:CacheClusterStatus,Node:CacheNodeType}' --output table

aws ssm get-parameters-by-path --path "/flightdata-manager/lab" --recursive --region eu-south-1 \
  --query 'Parameters[].Name' --output table
```

The last one should list ten entries: seven written by OpenTofu and the three Neo4j ones added by hand.

---

## Phase 4 — Registry and Kubernetes

Replaces the kubeadm cluster that Ansible builds on the Multipass VMs with a managed one. 18 resources: five ECR repositories with their lifecycle policies, two IAM roles, the cluster and a node group.

### Why EKS rather than kubeadm on EC2

The course material provisions EC2 instances and runs the same three Ansible playbooks used locally. That path is cheaper and reuses work already done, but it makes the cloud track a copy of the local one on rented hardware. Choosing EKS keeps the contrast that justifies having two tracks at all — one where every layer is operated by hand, one where the platform is delegated — and stays consistent with the managed RDS and ElastiCache already adopted in phase 3.

The Ansible playbooks are not wasted: they remain the heart of the local deployment.

### Node sizing

Two constraints decide this, and neither is the workload.

**The account's plan restricts which EC2 types may be launched at all.** Under the AWS free plan, `t3.medium` and larger are refused outright — the console says *"not eligible under the Free Plan"* — and a managed node group is nothing but EC2 instances, so asking for one would fail. What remains is `t3.micro`, `t3.small` and `c7i-flex.large`.

**The pod ceiling follows the network interfaces, not the memory**, because the VPC CNI gives every pod a real VPC address: `(interfaces × (addresses each − 1)) + 2`.

| Type | vCPU | RAM | Pod ceiling | Available in |
|---|---|---|---|---|
| `t3.micro` | 2 | 1 GB | 4 — two already go to DaemonSets | a, b |
| **`t3.small`** | 2 | 2 GB | **11** | **a, b** |
| `c7i-flex.large` | 2 | 4 GB | 29 | **b, c only** |

`c7i-flex.large` has the most room but does not exist in `eu-south-1a`, so adopting it would mean either moving the whole network to zones b and c or running both nodes in one zone — giving up the only place where this deployment has real redundancy. It also costs about `$0.10/hour` against `$0.024`.

Two `t3.small` fit with room to spare. Summing the resource requests of the existing manifests, minus the three components that no longer run in the cluster (Neo4j moved to Aura, PostgreSQL to RDS, Redis to ElastiCache):

| | Needed | Available on 2 × `t3.small` |
|---|---|---|
| Pods | 10 (8 application + CoreDNS) | 18 free of 22, after DaemonSets |
| Memory | ~960 Mi | 3144 Mi allocatable |
| CPU | ~1.1 vCPU | ~3.8 vCPU |

EKS reserves roughly 475 Mi per node for the kubelet and the eviction threshold, which is already subtracted above.

### The version is a cost decision

An EKS control plane costs `$0.10/hour` while its Kubernetes version is under standard support, and **`$0.60/hour` once it is not** — six times as much for an identical cluster. Versions move to extended support silently, so check before pinning:

```bash
aws eks describe-cluster-versions --region eu-south-1 \
  --query 'clusterVersions[?status==`STANDARD_SUPPORT`].clusterVersion' --output text
```

### Cost

| Resource | ~USD/hour |
|---|---|
| EKS control plane | 0.10 |
| 2 × `t3.small` | 0.05 |
| NAT Gateway + public IPv4 | 0.05 |
| RDS + ElastiCache | 0 to 0.04 |
| **Total** | **~0.20–0.24** |

About `$0.65` for a three-hour session, but `$5` for a forgotten day. From here the destroy discipline stops being a formality.

### A teardown trap worth knowing

OpenTofu destroys only what it created. A Kubernetes `Service` of type `LoadBalancer` makes **Kubernetes** create a load balancer that OpenTofu never sees — and that load balancer holds network interfaces in the subnets, so `tofu destroy` fails on the VPC with a `DependencyViolation` and leaves a billable resource behind.

The rule that avoids it: **delete the workloads before destroying the infrastructure.**

```bash
kubectl delete svc --all --all-namespaces
```

It does not apply yet, since no application is deployed, but it will from the next phase.

### Verify

```bash
aws eks describe-cluster --name flightdata-manager-lab --region eu-south-1 \
  --query 'cluster.{Stato:status,Versione:version,Endpoint:endpoint}' --output table

aws eks update-kubeconfig --name flightdata-manager-lab --region eu-south-1
kubectl get nodes -o wide
```

`update-kubeconfig` only works for a principal with an access entry on the cluster. The GitHub Actions role gets one automatically as its creator; to use `kubectl` from a laptop, grant one to the local user:

```bash
ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
aws eks create-access-entry --cluster-name flightdata-manager-lab --region eu-south-1 \
  --principal-arn "arn:aws:iam::${ACCOUNT}:user/dev-admin"
aws eks associate-access-policy --cluster-name flightdata-manager-lab --region eu-south-1 \
  --principal-arn "arn:aws:iam::${ACCOUNT}:user/dev-admin" \
  --policy-arn arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy \
  --access-scope type=cluster
```
