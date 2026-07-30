#!/usr/bin/env bash
# check-orphans.sh
# Answers one question after a teardown: is anything still billable?
#
# `tofu destroy` removes what OpenTofu created. It cannot remove what it never
# knew about — a load balancer or a volume created by Kubernetes itself, or a
# resource left behind by an interrupted run. This script asks the service APIs
# directly, which is the only authoritative source; the console resource group
# is an eventually consistent index and lags behind deletions by hours.
#
# Usage:
#   bash pkg/scripts/aws-setup/check-orphans.sh
#
# Exit code is 0 when the account is clean, 1 when something is still up.

set -uo pipefail

REGION="${AWS_REGION:-eu-south-1}"
PROJECT="${PROJECT_NAME:-flightdata-manager}"

FOUND=0

# Prints a finding and remembers that the account is not clean. Anything listed
# here either costs money or blocks the VPC from being deleted.
report() {
  local label="$1" value="$2"
  if [[ -n "$value" && "$value" != "None" ]]; then
    printf '  %-34s %s\n' "$label" "$value"
    FOUND=1
  fi
}

echo "Checking region ${REGION} for anything still running..."
echo ""

# ── What OpenTofu manages, and should therefore be gone ──────────────────────

report "EKS cluster" \
  "$(aws eks list-clusters --region "$REGION" --query 'clusters[]' --output text)"

report "EC2 instance" \
  "$(aws ec2 describe-instances --region "$REGION" \
      --filters "Name=instance-state-name,Values=pending,running,stopping,stopped" \
      --query 'Reservations[].Instances[].InstanceId' --output text)"

report "NAT Gateway" \
  "$(aws ec2 describe-nat-gateways --region "$REGION" \
      --filter "Name=state,Values=pending,available" \
      --query 'NatGateways[].NatGatewayId' --output text)"

report "RDS instance" \
  "$(aws rds describe-db-instances --region "$REGION" \
      --query 'DBInstances[].DBInstanceIdentifier' --output text)"

report "ElastiCache cluster" \
  "$(aws elasticache describe-cache-clusters --region "$REGION" \
      --query 'CacheClusters[].CacheClusterId' --output text)"

report "VPC" \
  "$(aws ec2 describe-vpcs --region "$REGION" \
      --filters "Name=tag:Project,Values=${PROJECT}" \
      --query 'Vpcs[].VpcId' --output text)"

# ── What OpenTofu cannot see ─────────────────────────────────────────────────

# Created by a Service of type LoadBalancer, never recorded in the state, and
# holds network interfaces that stop the VPC from being deleted.
report "Load balancer (from k8s)" \
  "$(aws elbv2 describe-load-balancers --region "$REGION" \
      --query 'LoadBalancers[].LoadBalancerName' --output text 2>/dev/null)"

report "Classic load balancer" \
  "$(aws elb describe-load-balancers --region "$REGION" \
      --query 'LoadBalancerDescriptions[].LoadBalancerName' --output text 2>/dev/null)"

# Left by a PersistentVolumeClaim. Billed per GB for as long as it exists, even
# detached.
report "Unattached EBS volume" \
  "$(aws ec2 describe-volumes --region "$REGION" \
      --filters "Name=status,Values=available" \
      --query 'Volumes[].VolumeId' --output text)"

# Free in itself, but a leftover interface keeps its subnet — and so the whole
# VPC — from being deleted.
report "Orphaned network interface" \
  "$(aws ec2 describe-network-interfaces --region "$REGION" \
      --filters "Name=status,Values=available" \
      --query 'NetworkInterfaces[].NetworkInterfaceId' --output text)"

# Billed hourly since 2024 whether attached or not.
report "Elastic IP" \
  "$(aws ec2 describe-addresses --region "$REGION" \
      --query 'Addresses[].PublicIp' --output text)"

# Survives the cluster it belonged to and keeps charging for stored data.
report "EKS log group" \
  "$(aws logs describe-log-groups --region "$REGION" \
      --log-group-name-prefix "/aws/eks/${PROJECT}" \
      --query 'logGroups[].logGroupName' --output text 2>/dev/null)"

# Storage is free below 500 MB, but a repository full of images is worth
# knowing about.
report "ECR repository" \
  "$(aws ecr describe-repositories --region "$REGION" \
      --query 'repositories[].repositoryName' --output text 2>/dev/null)"

report "Manual RDS snapshot" \
  "$(aws rds describe-db-snapshots --region "$REGION" --snapshot-type manual \
      --query 'DBSnapshots[].DBSnapshotIdentifier' --output text)"

echo ""
if [[ "$FOUND" -eq 0 ]]; then
  echo "Nothing is running. The account costs nothing right now."
  echo ""
  echo "Still present on purpose, and needed by the next session:"
  echo "  - the OpenTofu state bucket"
  echo "  - the IAM role and the GitHub OIDC provider"
  echo "  - the cost budget"
  echo "  - the three Neo4j parameters under /${PROJECT}/"
else
  echo "Something above is still up. If it is a load balancer or a volume, it was"
  echo "created by Kubernetes rather than by OpenTofu — delete the workloads first:"
  echo "    kubectl delete svc --all --all-namespaces"
  echo "then run the destroy workflow again."
  exit 1
fi
