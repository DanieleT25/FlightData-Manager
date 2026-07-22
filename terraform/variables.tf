variable "ubuntu_image" {
  description = "Ubuntu image tag recognised by Multipass (e.g. '24.04', 'lts')"
  type        = string
  default     = "24.04"
}

# ── Control-plane ──────────────────────────────────────────────────────────────

variable "control_plane_count" {
  description = "Number of control-plane nodes (1 for a standard single-master setup)"
  type        = number
  default     = 1
}

variable "control_plane_cpus" {
  description = "vCPUs allocated to each control-plane node (kubeadm requires >= 2)"
  type        = number
  default     = 2
}

variable "control_plane_memory" {
  description = "RAM allocated to each control-plane node (kubeadm requires >= 2 GB)"
  type        = string
  default     = "2G"
}

variable "control_plane_disk" {
  description = "Disk size for each control-plane node"
  type        = string
  default     = "10G"
}

# ── Workers ────────────────────────────────────────────────────────────────────

variable "worker_count" {
  description = "Number of Kubernetes worker nodes"
  type        = number
  default     = 2
}

variable "worker_cpus" {
  description = "vCPUs allocated to each worker node (kubeadm requires >= 2)"
  type        = number
  default     = 2
}

variable "worker_memory" {
  description = "RAM allocated to each worker node (3 GB supports the application and monitoring stack)"
  type        = string
  default     = "3G"
}

variable "worker_disk" {
  description = "Disk size for each worker node"
  type        = string
  default     = "10G"
}

# ── Ansible ────────────────────────────────────────────────────────────────────

variable "ssh_private_key_path" {
  description = "Path to the SSH private key used by Ansible"
  type        = string
  default     = "./terraform/id_ed25519"
}
