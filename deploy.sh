#!/bin/bash
set -e

kind create cluster --config ./kind/config.yml --name flight-data-cluster

kubectl delete namespace flight-data --ignore-not-found=true
kubectl create namespace flight-data

echo "--- Docker Build ---"
docker build -t user-manager -f microservices/user-manager/Dockerfile .
docker build -t data-collector -f microservices/data-collector/Dockerfile .
docker build -t alert-system -f microservices/alert-system/Dockerfile .
docker build -t alert-notifier -f microservices/alert-notifier-system/Dockerfile .


echo "--- Kind Load ---"
kind load docker-image user-manager --name flight-data-cluster
kind load docker-image data-collector --name flight-data-cluster
kind load docker-image alert-system --name flight-data-cluster
kind load docker-image alert-notifier --name flight-data-cluster

echo "--- Secrets & ConfigMaps ---"

kubectl create secret generic nginx-certs -n flight-data \
  --from-file=./pkg/certs/nginx-cert.pem \
  --from-file=./pkg/certs/nginx-key.pem

kubectl create secret generic grpc-certs -n flight-data \
  --from-file=./pkg/certs/ca-cert.pem \
  --from-file=./pkg/certs/server-cert.pem \
  --from-file=./pkg/certs/server-key.pem \
  --from-file=./pkg/certs/client-cert.pem \
  --from-file=./pkg/certs/client-key.pem

echo "--- ConfigMaps ---"
kubectl create configmap nginx-config -n flight-data --from-file=nginx.conf
kubectl create configmap prometheus-config -n flight-data --from-file=prometheus.yml

echo "--- Manifest ---"
kubectl apply -f k8s-manifest/

echo "DEPLOY COMPLETED!"