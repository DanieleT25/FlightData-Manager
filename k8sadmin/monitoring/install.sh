#!/usr/bin/env bash
# Deploy kube-prometheus-stack via Helm into the monitoring namespace
set -euo pipefail

RELEASE_NAME="kube-prom-stack"
NAMESPACE="monitoring"
GRAFANA_PASSWORD="${GRAFANA_PASSWORD:-admin}"

echo "==> Adding prometheus-community Helm repo..."
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
helm repo update

echo "==> Creating namespace ${NAMESPACE}..."
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

echo "==> Installing ${RELEASE_NAME}..."
helm upgrade --install "${RELEASE_NAME}" \
  prometheus-community/kube-prometheus-stack \
  --namespace "${NAMESPACE}" \
  --values "$(dirname "$0")/values.yaml" \
  --set grafana.adminPassword="${GRAFANA_PASSWORD}" \
  --wait --timeout=10m

echo ""
echo "==> Done. Stack installed in namespace '${NAMESPACE}'."
echo ""
echo "    Access Grafana:"
echo "      kubectl port-forward svc/${RELEASE_NAME}-grafana 3001:80 -n ${NAMESPACE}"
echo "      http://localhost:3001  (admin / ${GRAFANA_PASSWORD})"
echo ""
echo "    Access Prometheus:"
echo "      kubectl port-forward svc/${RELEASE_NAME}-kube-promethe-prometheus 9090:9090 -n ${NAMESPACE}"
echo "      http://localhost:9090"
