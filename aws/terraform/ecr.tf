# One registry per service. The local pipeline sidesteps a registry entirely by
# importing images straight into containerd on every node with `ctr`; on EKS the
# nodes are managed and disposable, so images have to come from somewhere they
# can pull from — that is ECR.
#
# Pulls stay inside the VPC and never cross the NAT Gateway, because image
# layers are served from S3 and the data and app subnets both route S3 traffic
# through the free gateway endpoint created in phase 2.

locals {
  services = [
    "user-manager",
    "data-collector",
    "alert-system",
    "alert-notifier",
    "frontend",
  ]
}

resource "aws_ecr_repository" "service" {
  for_each = toset(local.services)

  name = "${var.project_name}/${each.key}"

  # Without this, `tofu destroy` fails on any repository that still holds an
  # image, which after the first deployment is all of them.
  force_delete = true

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = { Name = "${local.name}-${each.key}" }
}

# Every deployment pushes a new tag, and the free tier only covers 500 MB. This
# keeps the last few revisions — enough to roll back to a previous image — and
# expires the rest.
resource "aws_ecr_lifecycle_policy" "service" {
  for_each = aws_ecr_repository.service

  repository = each.value.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep the 5 most recent images"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = 5
        }
        action = { type = "expire" }
      }
    ]
  })
}
