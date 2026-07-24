# Kubernetes Deployment (Gitea + OpenTofu + Multipass + Ansible)

The local Kubernetes deployment uses the following CI/CD chain:

```text
Gitea Actions → OpenTofu → Multipass → Ansible → Kubernetes (kubeadm)
```

OpenTofu creates one control-plane VM and one or more worker VMs. Ansible installs containerd and Kubernetes, bootstraps the cluster with kubeadm and installs Flannel. The application workflow builds the project images on the self-hosted runner and imports them into containerd on every node; therefore no external container registry is required for the local environment.

## Prerequisites

- Docker Desktop running
- Multipass, OpenTofu, Ansible, kubectl and Helm installed on the host that runs Gitea Actions
- Gitea and an `act_runner` registered as `self-hosted` on that same host
- An SSH key pair for the VMs

On macOS with Homebrew:

```bash
brew install opentofu ansible multipass kubectl helm
```

Generate the SSH key once from the repository root:

```bash
ssh-keygen -t ed25519 -f terraform/id_ed25519 -N ""
```

## Set up Gitea and its runner

Run these commands from the repository root. The Gitea installer detects macOS or Linux and downloads the matching binary.

```bash
bash pkg/scripts/gitea-setup/install-gitea.sh

# In a second terminal, after creating a runner token in Gitea:
cp pkg/scripts/gitea-setup/.env.example pkg/scripts/gitea-setup/.env
# Edit .env, then:
set -a; source pkg/scripts/gitea-setup/.env; set +a
bash pkg/scripts/gitea-setup/register-runner.sh
```

Create a Personal Access Token in Gitea (it is distinct from the runner token), then create the repository and all required Actions secrets. `REDIS_PASSWORD` may be empty in this local development setup.

```bash
export GITEA_TOKEN=<personal-access-token>
export NEO4J_PASSWORD=<strong-password>
export REDIS_PASSWORD=''
export POSTGRES_PASSWORD=<strong-local-postgres-password>
export OPENSKY_USER=<opensky-client-id>
export OPENSKY_PASSWORD=<opensky-client-secret>
export GRAFANA_PASSWORD=<strong-local-grafana-password>
bash pkg/scripts/gitea-setup/setup-pipeline.sh
```

The script stores these secrets in Gitea: `SSH_PRIVATE_KEY`, `SSH_PUBLIC_KEY`, `NEO4J_PASSWORD`, `REDIS_PASSWORD`, `POSTGRES_PASSWORD`, `OPENSKY_USER`, `OPENSKY_PASSWORD` and `GRAFANA_PASSWORD`.
They are never stored in Kubernetes manifests or committed to Git.

## Push the repository to Gitea

`setup-pipeline.sh` creates the Gitea repository but does not push any code to it. From the repository root:

```bash
git remote add gitea http://localhost:3000/<gitea-user>/FlightData-Manager.git
git push gitea main
```

Replace `<gitea-user>` with the account printed by `setup-pipeline.sh` (`Authenticated as: ...`). If the repository isn't a git repo yet, run `git init && git branch -M main` first.

## First deployment

Once the push completes, open the repository's **Actions** page.

1. Run **Provision K8s Cluster**. It creates the VMs, configures kubeadm and writes the host kubeconfig to `~/k8s-config`.
2. Run **Deploy to Kubernetes**. It builds and distributes the project images, creates runtime secrets and TLS certificates, applies the manifests and waits for the rollouts.

Future changes under `terraform/` or `ansible/` trigger the provisioning workflow; changes under `k8s/`, `microservices/` or `frontend/` trigger the application deployment workflow.

The infrastructure workflow installs Metrics Server and a resource-limited `kube-prometheus-stack`. Its Prometheus discovers the application services through `ServiceMonitor` resources, so the old standalone Kubernetes Prometheus deployment is no longer used. At the end of a successful deployment, use the control-plane address printed by the workflow:

```bash
curl -k https://<control-plane-ip>:30443/docs/user
open http://<control-plane-ip>:32000
```

Grafana uses `admin` as username and the `GRAFANA_PASSWORD` Gitea secret as its password. Prometheus is available from its Grafana datasource or via port-forward:

```bash
KUBECONFIG=~/k8s-config kubectl -n monitoring port-forward \
  svc/kube-prom-stack-kube-prome-prometheus 9090:9090
```

The root [`prometheus.yml`](../prometheus.yml) and the Prometheus service in `docker-compose.yml` are retained only for the separate Docker Compose development environment; they are not deployed to Kubernetes.

The stateful components (Redis, Postgres, Neo4j, Kafka) use `hostPath` volumes under `/var/lib/flight-data` on the first worker, which Ansible labels for local storage. This is intentional for a local lab cluster: data survives pod restarts on that node, but it is not a portable production storage strategy.

## Destroy the local environment

Run **Destroy K8s Cluster** manually from Gitea Actions. It calls `tofu destroy` and removes the generated host kubeconfig, inventory and join command. It never runs automatically.
