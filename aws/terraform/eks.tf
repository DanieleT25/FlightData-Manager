# Managed Kubernetes, replacing the kubeadm cluster that Ansible builds on the
# Multipass VMs locally. The trade is explicit: the control plane costs about
# $0.10/hour and is never free, but AWS runs it across availability zones and
# the three Ansible playbooks become unnecessary in this track.
#
# Nodes live in the private application subnets and reach the internet — and the
# EKS API — through the shared NAT Gateway.

# ── Cluster IAM role ──────────────────────────────────────────────────────────

data "aws_iam_policy_document" "eks_cluster_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["eks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "eks_cluster" {
  name               = "${local.name}-eks-cluster"
  assume_role_policy = data.aws_iam_policy_document.eks_cluster_assume.json

  tags = { Name = "${local.name}-eks-cluster" }
}

resource "aws_iam_role_policy_attachment" "eks_cluster" {
  role       = aws_iam_role.eks_cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

# ── Node IAM role ─────────────────────────────────────────────────────────────

data "aws_iam_policy_document" "eks_node_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "eks_node" {
  name               = "${local.name}-eks-node"
  assume_role_policy = data.aws_iam_policy_document.eks_node_assume.json

  tags = { Name = "${local.name}-eks-node" }
}

# The three policies every managed node needs: join the cluster, let the VPC CNI
# hand out addresses to pods, and pull images from ECR.
resource "aws_iam_role_policy_attachment" "eks_node" {
  for_each = toset([
    "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
    "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
    "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
  ])

  role       = aws_iam_role.eks_node.name
  policy_arn = each.value
}

# ── Cluster ───────────────────────────────────────────────────────────────────

resource "aws_eks_cluster" "main" {
  name     = local.name
  role_arn = aws_iam_role.eks_cluster.arn
  version  = var.kubernetes_version

  vpc_config {
    # Both tiers: EKS places its managed network interfaces in these subnets and
    # needs at least two availability zones, which is one of the reasons the
    # second zone exists at all.
    subnet_ids = concat(
      [for s in aws_subnet.app : s.id],
      [for s in aws_subnet.public : s.id],
    )

    # The API server has to be reachable from the GitHub runner, whose address
    # changes on every job, so it stays public and is protected by IAM
    # authentication rather than by network filtering. The nodes themselves
    # remain private and reach it over the internal endpoint.
    endpoint_public_access  = true
    endpoint_private_access = true
  }

  # Access entries replace the old aws-auth ConfigMap. Bootstrapping the creator
  # as administrator means the GitHub Actions role can run kubectl immediately
  # after creating the cluster, with no second authorisation step.
  access_config {
    authentication_mode                         = "API"
    bootstrap_cluster_creator_admin_permissions = true
  }

  depends_on = [aws_iam_role_policy_attachment.eks_cluster]

  tags = { Name = local.name }
}

# ── Node group ────────────────────────────────────────────────────────────────

resource "aws_eks_node_group" "main" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "${local.name}-nodes"
  node_role_arn   = aws_iam_role.eks_node.arn

  # Private subnets only: nodes have no public address and are reachable only
  # from inside the VPC.
  subnet_ids = [for s in aws_subnet.app : s.id]

  # See the variable for why this size and not another: the account's plan
  # refuses larger types outright, and the pod ceiling here follows the network
  # interfaces rather than the memory, because the VPC CNI gives every pod a
  # real VPC address.
  instance_types = [var.node_instance_type]
  disk_size      = var.node_disk_gb

  scaling_config {
    desired_size = var.node_count
    min_size     = var.node_count
    # The account's quota is 8 vCPUs for standard on-demand instances and every
    # t3 size uses 2, so three nodes is the practical ceiling here.
    max_size = var.node_count + 1
  }

  update_config {
    max_unavailable = 1
  }

  depends_on = [aws_iam_role_policy_attachment.eks_node]

  tags = { Name = "${local.name}-nodes" }

  lifecycle {
    # The desired size drifts as soon as anything scales the group, and letting
    # OpenTofu reset it on the next apply would fight the cluster.
    ignore_changes = [scaling_config[0].desired_size]
  }
}
