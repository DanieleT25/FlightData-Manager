locals {
  name = "${var.project_name}-${var.environment}"

  common_tags = {
    Project     = var.project_name
    Environment = var.environment
    ManagedBy   = "opentofu"
  }
}

data "aws_caller_identity" "current" {}

data "aws_region" "current" {}

# Restricting to available zones keeps later phases from placing subnets in a
# zone the account cannot use.
data "aws_availability_zones" "available" {
  state = "available"
}

# The only resource of this phase. Resource Groups are free, so the pipeline can
# be validated end to end — OIDC, remote state, locking, apply and destroy —
# before anything billable is provisioned. It stays useful afterwards: the
# console shows every resource carrying the Project tag as one group.
resource "aws_resourcegroups_group" "project" {
  name        = local.name
  description = "All resources of ${var.project_name} (${var.environment})"

  resource_query {
    query = jsonencode({
      ResourceTypeFilters = ["AWS::AllSupported"]
      TagFilters = [
        {
          Key    = "Project"
          Values = [var.project_name]
        }
      ]
    })
  }
}
