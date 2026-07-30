# Workflow logs and plan comments are world-readable on a public repository, so
# nothing here may expose the account id — that rules out outputting
# aws_caller_identity or any resource ARN, since ARNs embed it.

output "region" {
  description = "Region every resource is deployed into"
  value       = data.aws_region.current.region
}

output "availability_zones" {
  description = "Availability zones usable by this account, for the phase 2 subnets"
  value       = data.aws_availability_zones.available.names
}

output "resource_group_name" {
  description = "Console resource group collecting everything tagged with this project"
  value       = aws_resourcegroups_group.project.name
}

output "vpc_id" {
  description = "VPC hosting every resource of the project"
  value       = aws_vpc.main.id
}

output "public_subnet_ids" {
  description = "Public subnets, one per zone — NAT Gateway and public load balancer"
  value       = [for s in aws_subnet.public : s.id]
}

output "app_subnet_ids" {
  description = "Private subnets for the EKS nodes, with egress through the NAT Gateway"
  value       = [for s in aws_subnet.app : s.id]
}

output "data_subnet_ids" {
  description = "Private subnets for RDS and ElastiCache, with no route to the internet"
  value       = [for s in aws_subnet.data : s.id]
}

output "nat_public_ip" {
  description = "Address every outbound connection from the private subnets appears to come from"
  value       = aws_eip.nat.public_ip
}

# Endpoints are printed, credentials are not: the workflow summary is public.
# The password lives only in Parameter Store, readable with
#   aws ssm get-parameter --name /flightdata-manager/lab/postgres/password --with-decryption
output "postgres_endpoint" {
  description = "PostgreSQL endpoint, reachable only from inside the VPC"
  value       = aws_db_instance.postgres.endpoint
}

output "redis_endpoint" {
  description = "Redis endpoint, reachable only from inside the VPC"
  value       = "${aws_elasticache_cluster.redis.cache_nodes[0].address}:${aws_elasticache_cluster.redis.cache_nodes[0].port}"
}

output "ssm_parameter_prefix" {
  description = "Parameter Store path holding every connection detail of the data layer"
  value       = local.ssm_prefix
}

output "cluster_name" {
  description = "EKS cluster name — feed it to: aws eks update-kubeconfig --name <this> --region eu-south-1"
  value       = aws_eks_cluster.main.name
}

output "cluster_version" {
  description = "Kubernetes version served by the control plane"
  value       = aws_eks_cluster.main.version
}

# The registry URL embeds the account id, which must not reach a public log, so
# only the repository paths are published. The full URL is rebuilt at deploy
# time from the caller's own identity.
output "ecr_repository_names" {
  description = "ECR repositories, one per service"
  value       = [for r in aws_ecr_repository.service : r.name]
}
