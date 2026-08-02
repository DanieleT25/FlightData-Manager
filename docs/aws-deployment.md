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
| Frontend | nginx pod | nginx pod — the S3 + CloudFront code exists but is disabled, see phase 6 |

The two tracks live side by side on the same branch and never interfere: Gitea only reads `.gitea/workflows/`, GitHub only reads `.github/workflows/`.

## Cost model — read this first

The AWS free plan grants a fixed credit (typically $100, up to $200 after completing the onboarding tasks). Two properties of that credit drive every decision below:

- **Nothing is blocked.** Services outside the always-free tier — EKS control plane, NAT Gateway — bill normally; the charge is silently deducted from the credit instead of the card.
- **When the credit runs out the account is closed**, unless it was upgraded to the paid plan first, in which case billing continues on the card.

Cost is therefore a function of *uptime*, not of the architecture. Approximate hourly rates:

| Resource | ~USD/hour |
|---|---|
| EKS control plane | 0.10 |
| 2 × NAT Gateway, one per zone | 0.09 |
| 2 public IPv4 addresses | 0.01 |
| 3 × `t3.small` worker nodes | 0.072 |
| RDS Multi-AZ (`db.t4g.micro`) | ~0.036 |
| ElastiCache, primary + replica | ~0.038 |
| **Total** | **~0.35** |

Roughly **$8 per day left running**, against about **$1 for a three-hour session** followed by a destroy.

An earlier revision halved that by sharing one NAT Gateway between the zones and running single-instance databases, which fitted the free tier. It was abandoned deliberately: the saving mattered less than an architecture whose two zones are actually independent, given that this environment exists for minutes at a time.

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

It asks the service APIs directly, covers both what OpenTofu manages and what it cannot see, and exits non-zero if anything is still up. What it deliberately ignores is the handful of resources that must survive between sessions: the state bucket, the IAM role and OIDC provider, the budget, and the five hand-written parameters for Neo4j and OpenSky.

## Phases

| Phase | Contents |
|---|---|---|
| **0** | AWS account, OIDC, state bucket, budget alerts |
| 1 | `aws/terraform/` skeleton, `plan`/`apply`/`destroy` workflows |
| 2 | VPC, subnets, Internet Gateway, one NAT Gateway per zone |
| 3 | RDS Postgres, ElastiCache Redis, Neo4j Aura, Parameter Store |
| 4 | ECR, EKS cluster and node group |
| 5 | Application deployment: images, manifests, rollout |
| 6 | S3 + CloudFront static frontend — **written, left disabled** |
| 7 | Observability, documentation, final diagram |

Phase 6 is the one that did not ship. API Gateway and Cognito were dropped first — reaching the cluster from API Gateway needs a VPC Link, which needs a load balancer this plan refuses to create. CloudFront then turned out to need manual account verification, so the code sits behind `var.enable_cdn`, defaulting to `false`. Setting it to `true` on a verified account is all that is missing.

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

### One NAT Gateway per zone

Each zone has its own gateway and its own application route table pointing at it, so traffic leaving a node never crosses into the other zone and losing one zone does not take egress away from the other.

An earlier revision shared a single gateway between both zones to save about `$0.05/hour`. It worked, and it left an asymmetry worth naming: the second zone depended on the first for egress, so an outage of the first stopped everything while an outage of the second was survivable. Redundancy in one direction only.

That trade was reversed once it became clear the environment runs for minutes at a time rather than continuously — the hourly saving is worth less than a topology that is symmetric and explicable. A gateway costs about `$0.045/hour` plus `$0.045/GB` processed, and its public IPv4 address adds about `$0.005/hour`.

### S3 gateway endpoint

Free, and it removes a recurring charge: container image layers pulled from ECR are actually served from S3, and everything crossing a NAT Gateway is billed per GB. Routing S3 traffic through the endpoint keeps image pulls off the gateway entirely.

### Cost

| Resource | ~USD/hour |
|---|---|
| 2 × NAT Gateway | 0.09 |
| 2 × public IPv4 address | 0.01 |
| VPC, subnets, route tables, Internet Gateway, S3 endpoint | free |
| **Total** | **~0.10** |

About `$2.40` per day if left running, `$0.30` for a three-hour session.

### Verify

After `apply`, the outputs list the VPC, the three subnet groups and one NAT address per zone. From the console, **VPC → Resource map** draws the whole topology: it should show two zones, six subnets, and each application subnet routed to the gateway in its own zone — four route tables in total, not three.

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

Those credentials are written to Parameter Store **by hand**, once, together with the OpenSky ones — both are credentials for external services that OpenTofu does not create and therefore cannot generate:

```bash
aws ssm put-parameter --name "/flightdata-manager/lab/neo4j/uri"        --type String       --value "neo4j+s://<id>.databases.neo4j.io" --overwrite --region eu-south-1
aws ssm put-parameter --name "/flightdata-manager/lab/neo4j/user"       --type String       --value "<user>"        --overwrite --region eu-south-1
aws ssm put-parameter --name "/flightdata-manager/lab/neo4j/password"   --type SecureString --value '<password>'    --overwrite --region eu-south-1
aws ssm put-parameter --name "/flightdata-manager/lab/opensky/user"     --type String       --value "<client-id>"   --overwrite --region eu-south-1
aws ssm put-parameter --name "/flightdata-manager/lab/opensky/password" --type SecureString --value '<client-secret>' --overwrite --region eu-south-1
```

They never pass through OpenTofu, so they never reach the state file. They also survive `destroy`, unlike the seven parameters OpenTofu generates.

### Where each secret belongs

| Store | Holds | Why |
|---|---|---|
| **GitHub Secrets** | `AWS_ROLE_ARN`, `TF_STATE_BUCKET` | only what is needed to *reach* AWS — without them no pipeline could read anything else |
| **Parameter Store** | every runtime value: endpoints, database credentials, Neo4j and OpenSky credentials | once inside the account, it is the single source of configuration |

The dividing line is not "AWS service or external service" — Neo4j and OpenSky are both external and both live in Parameter Store. It is *bootstrap versus runtime*. A consequence worth having: the deployment pipeline could be replaced tomorrow without moving a single secret.

### Secrets and the state file

The RDS master password is different: OpenTofu generates it, which means it **is** written to the state in clear text. That is inherent to how Terraform and OpenTofu work, not a flaw in this configuration, and it is why the state bucket is private, encrypted and versioned. The same value is published to Parameter Store as a `SecureString` so that the deployment can read it without anyone ever handling it.

Parameter Store rather than Secrets Manager: both would work, but Secrets Manager costs $0.40 per secret per month while the standard tier of Parameter Store is free, and the only feature lost — automatic rotation — is not used here.

### Sizing

| Setting | Value | Reason |
|---|---|---|
| RDS class | `db.t4g.micro` | smallest that runs PostgreSQL 16 |
| RDS storage | 20 GB `gp2` | more than the schema will ever need here |
| **RDS Multi-AZ** | **on** | a synchronous standby in the second zone, with automatic failover |
| Backup retention | 0 days | the environment is destroyed after every session; backups would only add storage cost and slow the teardown |
| Final snapshot | skipped | otherwise every destroy leaves a billable snapshot behind |
| **ElastiCache** | **replication group, 2 nodes** | primary and replica in different zones, automatic failover |
| Redis AUTH | off | access is restricted by the security group and by the subnet having no route out, as in the local deployment |

Both engines are highly available, which the free tier does not cover: together they cost roughly `$0.074/hour`. That is the deliberate trade described in the cost model — this environment runs for minutes, so symmetry between the zones is worth more than fitting inside the free tier.

### What Multi-AZ actually means here

The reference diagram draws RDS and ElastiCache in **both** zones. That notation never meant two independent databases, and the implementation matches it precisely:

- **RDS** keeps one logical database with a synchronous standby in the other zone, behind a **single endpoint**. The application is unaware a failover happened.
- **ElastiCache** needs a different resource for this: `aws_elasticache_replication_group` rather than `aws_elasticache_cluster`, because a standalone cache node cannot span zones. Two nodes, a primary and a replica, with `automatic_failover_enabled`.

One consequence shapes the configuration: the Redis address published to Parameter Store is `primary_endpoint_address`, not a node address. That endpoint follows whichever node currently holds the primary role, so a failover requires no change on the client side — pointing at a node address would break the moment the roles swapped.

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

The last one should list twelve entries: seven written by OpenTofu, plus the three Neo4j and two OpenSky ones added by hand.

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

---

## Phase 5 — Deploying the application

Builds the five images, publishes them to ECR and rolls the application onto the cluster. A separate workflow from `aws-infra.yml`: changing a microservice must not touch the infrastructure, and changing the infrastructure must not redeploy the application.

```
Actions → AWS — Deploy → Run workflow
```

It is manual only. The cluster does not exist between sessions, so a push trigger would produce a failed run for every commit.

### Only two manifests are AWS-specific

The application manifests are shared with the local deployment, because images and configuration are injected at deploy time by both pipelines. Duplicating them would only create two copies to keep in step. `aws/k8s/` therefore holds just the two that genuinely differ:

| Manifest | Local | AWS |
|---|---|---|
| `kafka.yml` | `hostPath` on the node labelled for local storage | `emptyDir` |
| `nginx.yml` | `NodePort` on the Multipass VM addresses | `LoadBalancer` |

Everything else — `user-manager`, `data-collector`, `alert-system`, `alert-notifier`, `frontend` — is applied straight from `k8s/`. The three StatefulSets of the local stack are never applied at all: RDS, ElastiCache and Aura replace them. `service-monitors.yml` is skipped too, since it needs the Prometheus operator CRDs that phase 7 installs.

**Why `emptyDir` for Kafka.** Managed nodes are disposable, so a `hostPath` would quietly lose its contents whenever the node group replaced an instance. The alternative, an EBS volume through a `PersistentVolumeClaim`, needs the CSI driver and — worse — creates a volume OpenTofu does not know about, which survives `tofu destroy`, keeps being billed and blocks the VPC from being deleted. The cost of `emptyDir` is that undelivered messages are lost if the pod moves, which is acceptable for a pipeline where messages are produced and consumed within seconds.

**Why `ClusterIP` for nginx, and not `LoadBalancer`.** The nodes sit in private subnets with no public address, so a node port would be unreachable and a load balancer is the natural answer — the public subnets were tagged `kubernetes.io/role/elb` in phase 2 for exactly that.

It turns out this account cannot have one. Creating the Service leaves it `<pending>` forever while the controller retries:

```
OperationNotPermitted: This AWS account currently does not support creating load balancers.
```

The free plan refuses the `CreateLoadBalancer` call outright — the same class of restriction that blocks EC2 instance types above `t3.small`, not a tagging or annotation problem. A `ClusterIP` is therefore the honest configuration: it works for everything inside the cluster and avoids leaving a resource that can only fail. Restoring the intended design is a one-word change on an account whose plan allows it.

### Where the configuration comes from

Nothing about the data layer is repeated in the workflow. It reads Parameter Store, where phase 3 published the endpoints and credentials, and turns them into a ConfigMap and a Secret — the same shape the local pipeline builds from Gitea secrets, so the manifests do not care which deployment they are running in.

The only value coming from GitHub is the OpenSky credential pair, which belongs to no AWS resource. The account id is masked with `::add-mask::` before any registry URL is echoed, since the logs are public.

One deliberate difference from the local stack: `POSTGRES_SSLMODE` is `require` rather than `disable`. RDS serves TLS out of the box and the connection crosses subnets.

### Teardown

Because no load balancer can be created, nothing outside OpenTofu's knowledge is ever provisioned, and `tofu destroy` is enough on its own. Deleting the workloads first remains good practice rather than a requirement — it costs nothing and keeps the habit for the day a `LoadBalancer` Service does get created:

```bash
kubectl delete svc --all -n flight-data     # precautionary
# then the AWS — Destroy workflow
bash pkg/scripts/aws-setup/check-orphans.sh # confirms
```

### A known limitation: OpenSky is unreachable from AWS

`opensky-network.org` answers in a quarter of a second from a home connection and times out from inside the cluster, while other hosts answer normally from the same pods — the signature of a cloud-provider IP filter, which public APIs commonly apply to discourage scraping.

It does not prevent the deployment. `data-collector` only calls OpenSky once a user has registered an interest, so the API, user management, the gRPC link between services and the whole Kafka alert chain work exactly as they do locally. Only the retrieval of live flight data fails, and only in the cloud track.

### Verify

The run summary prints the pod list. From a laptop, reach the application through a forwarded port:

```bash
aws eks update-kubeconfig --name flightdata-manager-lab --region eu-south-1
kubectl -n flight-data port-forward svc/nginx 8443:443 &
curl -k https://localhost:8443/docs/user
```

The certificate is self-signed by `gen_certs.sh`, hence `-k`.

The test that actually proves the deployment works is registering a user twice with the same `X-Request-ID`: the first call writes to RDS, the second returns the response cached in ElastiCache. The two are distinguishable in the payload — `registered_at` keeps nanosecond precision when it comes from the cache and is truncated to microseconds when it is read back from PostgreSQL, which is direct evidence that both stores are doing their job.

---

## Phase 6 — Static frontend on S3 and CloudFront

The Svelte application is a Vite build: a few hundred kilobytes of HTML, CSS and JavaScript with no server-side logic. Serving it from a container means paying for compute to hand out files, so it also goes to a bucket behind a CDN — the `Frontend: S3 + CloudFront` row of the target architecture.

Eight resources: a bucket with its public-access block and policy, an Origin Access Control, the distribution, and two Parameter Store entries the deploy workflow reads.

### The bucket is never public

Files are reachable only through the distribution, using an **Origin Access Control**: CloudFront signs its requests to S3, and the bucket policy accepts `s3:GetObject` from the CloudFront service principal *only when the request comes from this distribution's ARN*. No other distribution, in this account or anyone else's, can read it.

That is worth more than convenience. Because the bucket has no public path, the HTTPS redirect and the caching cannot be bypassed by addressing S3 directly — a plain public bucket would leave that door open.

### Single-page routing

A client-routed application has no object at `/login` or `/dashboard`, so S3 answers 403 or 404 and a reload or a shared link breaks. The distribution maps both codes to `/index.html` with a 200, letting the application resolve the route itself.

### Why the frontend also stays in the cluster

Not an oversight. The Svelte code calls its API with **relative paths** — `fetch('/api/interests')` — which resolve against the origin the page was loaded from. Behind nginx, frontend and API share one origin and it works. Loaded from CloudFront, the same call reaches the CDN, which knows only about the bucket.

Fixing that means giving the API a public address to use as a second origin, which needs a load balancer. This account cannot create one — verified twice, from the Kubernetes service controller and again from the console on an unrelated VPC:

```
OperationNotPermittedException: This AWS account currently does not support creating load balancers.
```

So there are two frontends on purpose, each demonstrating a different thing: the one behind nginx is the path that actually works end to end, reachable through a forwarded port; the one on CloudFront shows the CDN tier of the architecture and renders correctly, but its API calls cannot reach the cluster.

### Cost

| Resource | Cost |
|---|---|
| S3 storage | ~160 KB against a 5 GB free tier |
| CloudFront | free tier covers 1 TB egress and 10 M requests per month |
| Distribution itself | no hourly charge — only usage |

Effectively zero. **The teardown, however, gets noticeably slower**: a distribution has to be disabled and the change propagated to the edge locations before it can be deleted, which OpenTofu does automatically but which adds ten to fifteen minutes to every `destroy`. Worth knowing before starting a short session.

### Verify

```bash
tofu -chdir=aws/terraform output frontend_url
```

Opening it should show the interface, served over HTTPS from the nearest edge location. Confirm the bucket is not reachable directly — this must fail:

```bash
BUCKET=$(aws ssm get-parameter --name /flightdata-manager/lab/frontend/bucket --query 'Parameter.Value' --output text)
curl -s -o /dev/null -w '%{http_code}\n' "https://${BUCKET}.s3.eu-south-1.amazonaws.com/index.html"   # expect 403
```

A 403 there is the Origin Access Control doing its job: the files exist, but only CloudFront may fetch them.

Logging in from the CDN page will fail, and that is the documented limitation rather than a defect — the working demonstration is the forwarded port.

---

## Phase 7 — Observability

Installs Metrics Server and `kube-prometheus-stack` into the cluster, then registers the application with Prometheus.

```
Actions → AWS — Monitoring → Run workflow
```

A separate workflow from the application deployment, mirroring how the local track keeps `k8sadmin/` apart from its pipeline. The Helm release changes far less often than the code, and `helm upgrade --wait` takes minutes — running it on every deployment would slow the loop down for nothing.

The two install scripts are **reused unchanged** from the local deployment. They read `KUBECONFIG` and `GRAFANA_PASSWORD` from the environment and know nothing about Multipass, so the same file provisions both clusters — the clearest evidence that the observability layer is genuinely portable.

### The ServiceMonitors finally apply

`k8s/service-monitors.yml` was deliberately skipped in phase 5: it declares `monitoring.coreos.com/v1` objects, whose CRDs arrive with the Helm chart. With the stack installed they apply, and Prometheus starts scraping `/metrics` on `user-manager` and `data-collector` every 15 seconds — the same two instrumented services as locally, through the same manifest.

### Why the node count went from two to three

Memory was never the constraint: the whole stack requests about 640 Mi against more than 2 Gi free. **Pods were.** Each `t3.small` admits 11 — a limit set by network interfaces, not resources — so two nodes give 22, of which 14 were already taken by the application, CoreDNS and the two DaemonSets. The monitoring stack needs 8, which is exactly what was left, with nothing to spare if the scheduler distributed them unevenly.

A third node adds 11 slots and 1.5 Gi for about `$0.024/hour`, bringing the margin to 8 spare slots and 3.1 Gi. It stays inside the 8 vCPU quota even while a rolling update briefly runs a fourth.

This is the one axis of scale the account plan leaves open: `t3.medium` and larger are refused outright, but more `t3.small` are not.

### Grafana's password is generated, not chosen

Like the RDS one, it comes from `random_password` and lands in Parameter Store as a `SecureString`, so no manual parameter has to exist before the first run and nobody handles the value:

```bash
aws ssm get-parameter --name /flightdata-manager/lab/grafana/password \
  --with-decryption --query Parameter.Value --output text
```

### Reaching Grafana

The chart exposes it on a NodePort, which is inert here for the same reason nginx's was: the nodes are private and the plan allows no load balancer. A forwarded port works identically:

```bash
kubectl -n monitoring port-forward svc/kube-prom-stack-grafana 3001:80
# http://localhost:3001 — user admin
```

### Cost

The stack itself is free software on nodes already paid for; the only marginal cost is the third node, about `$0.024/hour`. Total with everything running is roughly **`$0.35/hour`**.

---

## What was built, and what the plan blocked

These are restrictions, not choices. Multi-AZ on RDS and ElastiCache was on this list in an earlier revision as a cost decision; it is now built.

| Component | What AWS answered | Verified |
|---|---|---|
| `t3.medium` and larger nodes | *not eligible under the Free Plan* | console, instance type selector |
| Load balancer | `OperationNotPermitted: this AWS account currently does not support creating load balancers` | Kubernetes service controller, then the console on an unrelated VPC |
| CloudFront | `AccessDenied: your account must be verified before you can add new CloudFront resources` | `tofu apply` |
| OpenSky from AWS | connection times out, while other hosts answer normally from the same pods | `curl` from inside the cluster |

None of them changed the design: the code for CloudFront is written and sits behind `enable_cdn`, the load balancer is one word away in `aws/k8s/nginx.yml`, and the network already carries the `kubernetes.io/role/elb` tags a load balancer would need. What changed is the demonstration — a forwarded tunnel instead of a public address.

### The two tracks, side by side

| | Local | AWS |
|---|---|---|
| Kubernetes | kubeadm on Multipass, built by Ansible | EKS, managed control plane |
| PostgreSQL, Redis | StatefulSets with hostPath | RDS and ElastiCache |
| Neo4j | StatefulSet in cluster | Aura, outside the account |
| Images | imported into containerd with `ctr` | ECR |
| Secrets | Gitea secrets | Parameter Store |
| State | file on the host | S3, versioned and locked |
| Credentials for CI | static SSH keys | OIDC, no stored keys |
| Entry point | NodePort on the VM address | forwarded port |
| Observability | `k8sadmin/` scripts | **the same scripts, unchanged** |

The last row is the point. Everything above it is a substitution — a self-managed component swapped for a service the provider runs. The observability layer needed no substitution at all, because it was already speaking Kubernetes rather than speaking to the infrastructure underneath.
