# Flight Data Manager - Distributed Microservices System


_Project for "Distributed Systems and Big Data" course of Computer Engineering_

_[Daniele Tambone](https://www.linkedin.com/in/daniele-tambone-b5733616a/) @ Dept of Math and Computer Science, University of Catania_

## Overview

**Flight Data Manager** is a robust distributed system designed to manage user subscriptions and monitor real-time flight data from OpenSky Network. The project demonstrates a **Microservices Architecture** built with **Go**, adhering to **Hexagonal Architecture (Ports & Adapters)** principles to ensure decoupling, testability, and scalability.

The system is composed of two main services:
1.  **User Manager:** Handles user identity, secure registration, and idempotency.
2.  **Data Collector:** Manages flight data collection, user interests, and statistical analysis.


## Architecture & Design Choices

The project implements advanced distributed systems patterns:

* **Hexagonal Architecture:** Both services separate Domain Logic (Core) from Infrastructure (Adapters) via Interfaces (Ports).
* **Inter-Service Communication:**
    * **gRPC:** Used for internal, high-performance synchronous communication (e.g., verifying user existence).
    * **REST API (Huma):** Used for external communication with clients.
* **Resiliency:**
    * **At-Most-Once Delivery:** Implemented in User Manager using a custom **Idempotency Key mechanism** (IP Hash + Request ID) backed by Redis.
    * **Fault Tolerance:** The Data Collector uses **Retry** and **Circuit Breaker** patterns when calling the User Manager.
* **Data Storage:**
    * **Redis:** Key-Value store for User profiles and Idempotency locks (Speed).
    * **Neo4j:** Graph database for modeling relationships (Users -> Interests -> Flights).
* **Security:**
    * **mTLS Ready:** Infrastructure code supports Mutual TLS for gRPC.
    * **Bcrypt:** Passwords are hashed before storage.
    * **Access Control:** Data access is guarded by cross-service credential verification.


## Getting Started

### 1. Prerequisites

  * **Docker** & **Docker Compose**
  * **Go 1.25+** (Optional, only for local development)
  * **Restish** (CLI for testing APIs):

**macOS:**
```bash
# Homebrew
brew tap danielgtaylor/restish
brew install restish

# Go (requires Go 1.18+)
go install [github.com/danielgtaylor/restish@latest](https://github.com/danielgtaylor/restish@latest)
```

**Linux:**
```bash
# Go (requires Go 1.18+)
go install [github.com/danielgtaylor/restish@latest](https://github.com/danielgtaylor/restish@latest)

# Homebrew for Linux
brew tap danielgtaylor/restish
brew install restish
```

**Windows:**
```bash
# Go (requires Go 1.18+)
go install [github.com/danielgtaylor/restish@latest](https://github.com/danielgtaylor/restish@latest)
```

### 2. Configuration (.env)

Create a `.env` file in the root directory. You can copy the example below:

```ini
# --- OPENSKY NETWORK (Required for Data Collector) ---
# Use your API Client credentials (recommended) or website login
OPENSKY_USER=your-client-id
OPENSKY_PASSWORD=your-client-secret

# --- NEO4J ---
NEO4J_USER=neo4j
NEO4J_PASSWORD=secret_password

# --- REDIS ---
REDIS_PASSWORD=
REDIS_DB=0
```

### 3. Generate Certificates (mTLS)

Before running, generate the TLS certificates for internal gRPC communication and https:

```bash
chmod +x pkg/scripts/gen_certs.sh
./pkg/scripts/gen_certs.sh
```

### 4. Run the System

Build and start all containers using Docker Compose:

```bash
docker-compose up --build
```

The system will be available via the **API Gateway (Nginx)** at `https://localhost:3443`.


## Testing the API

  * **User Manager Docs:** https://localhost:3443/docs/user
  * **Data Collector Docs:** https://localhost:3443/docs/data

> [!note]
> You can see a quick scenario in `docs/demo.md`.

## Development & Debugging

  * **Redis CLI:** `docker exec -it redis redis-cli`
  * **Neo4j Cypher Shell:** `docker exec -it neo4j cypher-shell -u neo4j -p <password>`
  * **Forcing Data Collection:** The worker runs every 12h by default. To force an immediate run:
    `docker-compose restart data-collector`

> [!tip]
>
> If you prefer use Neo4j Browser, in neo4j service in docker-compose:
>
>  ```yaml
>  neo4j:
>  	...
>    #ports:
>    #  - "7474:7474" # Browser UI
>    #  - "7687:7687"
>    ...
>  ```
>
> Remove the comment.
>
> Now you can use Neo4j Browser in http://localhost:7474/browser/

## Tests

Run the full automated suite from the repository root:

```bash
go test ./...
```

Each microservice has unit tests for its core logic and component-integration tests for its public adapter boundary: HTTP/Huma for User Manager and Data Collector, Kafka payload-to-service processing for Alert System and Alert Notifier. The deploy workflow runs this suite before building an image.

## Future implementation
- JWT Authentication: Implement token-based auth to avoid sending credentials on every request to the Data Collector.
- Microservices Testing: Implement comprehensive unit and integration tests for each microservice (More testing).

## Kubernetes

The local deployment uses the following CI/CD chain:

```text
Gitea Actions → OpenTofu → Multipass → Ansible → Kubernetes (kubeadm)
```

OpenTofu creates one control-plane VM and two worker VMs. Ansible installs containerd and Kubernetes, bootstraps the cluster with kubeadm and installs Flannel. The application workflow builds the project images on the self-hosted runner and imports them into containerd on every node; therefore no external container registry is required for the local environment.

### Prerequisites

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

### Set up Gitea and its runner

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
export OPENSKY_USER=<opensky-client-id>
export OPENSKY_PASSWORD=<opensky-client-secret>
export GRAFANA_PASSWORD=<strong-local-grafana-password>
bash pkg/scripts/gitea-setup/setup-pipeline.sh
```

The script stores these secrets in Gitea: `SSH_PRIVATE_KEY`, `SSH_PUBLIC_KEY`, `NEO4J_PASSWORD`, `REDIS_PASSWORD`, `OPENSKY_USER`, `OPENSKY_PASSWORD` and
`GRAFANA_PASSWORD`.
They are never stored in Kubernetes manifests or committed to Git.

### First deployment

Push the repository to the Gitea remote, then open its **Actions** page.

1. Run **Provision K8s Cluster**. It creates the VMs, configures kubeadm and writes the host kubeconfig to `~/k8s-config`.
2. Run **Deploy to Kubernetes**. It builds and distributes the project images, creates runtime secrets and TLS certificates, applies the manifests and waits for the rollouts.

Future changes under `terraform/` or `ansible/` trigger the provisioning
workflow; changes under `k8s/`, `microservices/` or `frontend/` trigger the
application deployment workflow.

The infrastructure workflow installs Metrics Server and a resource-limited
`kube-prometheus-stack`. Its Prometheus discovers the two application services
through `ServiceMonitor` resources, so the old standalone Kubernetes Prometheus
deployment is no longer used. At the end of a successful deployment, use the
control-plane address printed by the workflow:

```bash
curl -k https://<control-plane-ip>:30443/docs/user
open http://<control-plane-ip>:32000
```

Grafana uses `admin` as username and the `GRAFANA_PASSWORD` Gitea secret as its
password. Prometheus is available from its Grafana datasource or via port-forward:

```bash
KUBECONFIG=~/k8s-config kubectl -n monitoring port-forward \
  svc/kube-prom-stack-kube-prome-prometheus 9090:9090
```

The root [`prometheus.yml`](prometheus.yml) and the Prometheus service in
`docker-compose.yml` are retained only for the separate Docker Compose
development environment; they are not deployed to Kubernetes.

The stateful components use `hostPath` volumes under
`/var/lib/flight-data` on the first worker, which Ansible labels for local
storage. This is intentional for a local lab cluster: data survives pod
restarts on that node, but it is not a portable production storage strategy.

### Destroy the local environment

Run **Destroy K8s Cluster** manually from Gitea Actions. It calls
`tofu destroy` and removes the generated host kubeconfig, inventory and join
command. It never runs automatically.
