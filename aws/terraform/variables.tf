variable "aws_region" {
  description = "Region hosting every resource of this project"
  type        = string
  default     = "eu-south-1" # Milan
}

variable "project_name" {
  description = "Name prefix and value of the Project tag on every resource"
  type        = string
  default     = "flightdata-manager"
}

variable "environment" {
  description = "Deployment environment, used as a name suffix and as a tag"
  type        = string
  default     = "lab"
}

variable "kubernetes_version" {
  description = "EKS control plane version — must be one AWS still covers by standard support"
  type        = string

  # This is not a matter of taste. Once a version leaves standard support the
  # control plane is billed at $0.60/hour instead of $0.10 — six times as much,
  # for a cluster that behaves identically. Check before changing it:
  #   aws eks describe-cluster-versions --region eu-south-1 \
  #     --query 'clusterVersions[?status==`STANDARD_SUPPORT`].clusterVersion'
  # 1.35 is in standard support until March 2027.
  default = "1.35"
}

variable "node_instance_type" {
  description = "Worker node size — constrained by what the account's plan allows, not only by the workload"
  type        = string

  # The AWS free plan restricts which EC2 types the account may launch at all:
  # t3.medium and above are refused with "not eligible under the Free Plan",
  # which would fail node group creation. Of what remains, t3.micro tops out at
  # four pods and 1 GB, and c7i-flex.large has more memory but does not exist in
  # eu-south-1a, so using it would mean giving up either the first zone or node
  # redundancy. t3.small is available in both zones and fits the workload: the
  # application needs about 0.96 GB and 14 pod slots against the 3.1 GB and 22
  # slots that two of these provide.
  default = "t3.small"
}

variable "node_count" {
  description = "Number of worker nodes, one per availability zone"
  type        = number
  default     = 2

  validation {
    # Standard on-demand instances are capped at 8 vCPUs in this account and
    # every t3 size uses 2, leaving room for four nodes at most.
    condition     = var.node_count >= 1 && var.node_count <= 3
    error_message = "The 8 vCPU service quota allows at most 3 nodes plus room to roll one."
  }
}

variable "node_disk_gb" {
  description = "Root volume of each worker node"
  type        = number
  default     = 20
}

variable "postgres_instance_class" {
  description = "RDS instance class — db.t4g.micro is covered by the free tier"
  type        = string
  default     = "db.t4g.micro"
}

variable "postgres_storage_gb" {
  description = "RDS allocated storage in GB — the free tier covers 20"
  type        = number
  default     = 20
}

variable "redis_node_type" {
  description = "ElastiCache node type — cache.t3.micro is covered by the free tier"
  type        = string
  default     = "cache.t3.micro"
}

variable "enable_cdn" {
  description = "Whether to create the S3 + CloudFront frontend"
  type        = bool

  # False until AWS confirms the account. CloudFront distributions are gated
  # behind manual account verification on new accounts — apply fails with:
  #   AccessDenied: Your account must be verified before you can add new
  #   CloudFront resources. To verify your account, please contact AWS Support.
  # Flip to true once that verification has gone through; nothing else in this
  # configuration depends on it.
  default = false
}

variable "vpc_cidr" {
  description = "Address range of the VPC, split into three subnet tiers per zone"
  type        = string
  default     = "10.0.0.0/16"
}

variable "az_count" {
  description = "Number of availability zones to spread the subnets across"
  type        = number
  default     = 2

  validation {
    condition     = var.az_count >= 1 && var.az_count <= 3
    error_message = "eu-south-1 offers three availability zones."
  }
}
