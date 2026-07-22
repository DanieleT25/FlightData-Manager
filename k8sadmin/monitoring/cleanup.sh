#!/usr/bin/env bash
# Tear down kube-prometheus-stack and Metrics Server
set -euo pipefail

RELEASE_NAME="kube-prom-stack"
NAMESPACE="monitoring"

echo "==> Uninstalling Helm release ${RELEASE_NAME}..."
helm uninstall "${RELEASE_NAME}" -n "${NAMESPACE}" || true

echo "==> Deleting namespace ${NAMESPACE}..."
kubectl delete namespace "${NAMESPACE}" --ignore-not-found

echo "==> Removing Metrics Server..."
kubectl delete -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml --ignore-not-found

echo "==> Removing CRDs left by kube-prometheus-stack..."
kubectl get crd -o name | grep monitoring.coreos.com | xargs -r kubectl delete || true

echo "==> Done."
