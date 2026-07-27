terraform {
  required_version = ">= 1.10"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  # Deliberately empty: the bucket name embeds the AWS account id and this
  # repository is public, so it is supplied at init time instead of being
  # committed here:
  #
  #   tofu -chdir=aws/terraform init \
  #     -backend-config="bucket=<state-bucket>" \
  #     -backend-config="key=aws/terraform.tfstate" \
  #     -backend-config="region=eu-south-1"
  #
  # The workflows read the bucket name from the TF_STATE_BUCKET secret.
  #
  # State is locked with OpenTofu's native S3 locking (>= 1.10); the separate
  # DynamoDB lock table that older versions required is deprecated.
  backend "s3" {
    encrypt      = true
    use_lockfile = true
  }
}

provider "aws" {
  region = var.aws_region

  # Applied to every resource that supports tagging. The resource group in
  # main.tf collects the project's resources through the Project tag, and the
  # same tags make per-project cost allocation possible in Cost Explorer.
  default_tags {
    tags = local.common_tags
  }
}
