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
