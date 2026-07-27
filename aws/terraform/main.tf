locals {
  name = "${var.project_name}-${var.environment}"

  common_tags = {
    Project     = var.project_name
    Environment = var.environment
    ManagedBy   = "opentofu"
  }
}

# No aws_caller_identity data source here: reading it prints the account id in
# the plan log, and this repository's workflow logs are public. Nothing in this
# configuration needs it — the account id only ever appears inside secrets.

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
  name = local.name
  # Only [\s a-z A-Z 0-9 _ . -] is accepted here — no parentheses, colons or
  # equals signs, and the API rejects the whole call if any slip in.
  description = "All resources of ${var.project_name} in environment ${var.environment}"

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
