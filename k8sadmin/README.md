# k8sadmin — Kubernetes Administration Lab

Practical scripts and configs for Day 2 K8s operations used alongside the
`K8S-gitea-cicd` slide deck.

## Structure

```
k8sadmin/
├── metrics-server/
│   └── install.sh          # Deploy Metrics Server + Multipass patch
└── monitoring/
    ├── install.sh           # Deploy kube-prometheus-stack via Helm
    ├── cleanup.sh           # Remove stack + CRDs
    └── values.yaml          # Resource-tuned Helm values for Multipass
```

## Quick Start

```bash
# 1. Metrics Server (kubectl top)
bash metrics-server/install.sh

# 2. Full Prometheus + Grafana stack
GRAFANA_PASSWORD=mysecret bash monitoring/install.sh

# 3. Access Grafana (NodePort 32000 via values.yaml)
# http://<control-plane-ip>:32000   admin / mysecret

# 4. Teardown
bash monitoring/cleanup.sh
```

## Prerequisites

- `helm` ≥ 3.x installed on the machine running these scripts
- `kubectl` configured with a valid kubeconfig pointing at the cluster
- Cluster provisioned via this repository's `terraform/` workflow

The normal Gitea infrastructure workflow already runs both installation
scripts. Run them manually only when administering an existing cluster outside
the workflow.
