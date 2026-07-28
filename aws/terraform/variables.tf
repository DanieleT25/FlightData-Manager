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
