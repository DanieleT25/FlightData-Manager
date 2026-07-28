# Network layer: two availability zones, three subnet tiers per zone.
#
#   public       Internet Gateway route. Holds the NAT Gateway and, later, the
#                public load balancer.
#   private app  Egress through the NAT Gateway. Will host the EKS nodes.
#   private data ---. No route to the internet at all. Will host RDS and
#                ElastiCache, which never need to reach out.
#
# Both zones share a single NAT Gateway, in the first zone. This is a deliberate
# cost decision: a NAT Gateway costs about $0.045/hour, so one per zone would
# double the standing cost of the whole project. The trade-off is that an outage
# of the first zone also cuts egress for the second one — acceptable for a lab,
# not for production, where each zone gets its own.

locals {
  azs = slice(data.aws_availability_zones.available.names, 0, var.az_count)

  # /24 is ample for the public and data tiers, which hold a handful of network
  # interfaces. The app tier gets /20 because the EKS VPC CNI assigns a real VPC
  # address to every pod, not just to every node.
  public_subnets = { for i, az in local.azs : az => cidrsubnet(var.vpc_cidr, 8, i) }
  app_subnets    = { for i, az in local.azs : az => cidrsubnet(var.vpc_cidr, 4, i + 1) }
  data_subnets   = { for i, az in local.azs : az => cidrsubnet(var.vpc_cidr, 8, i + 48) }
}

resource "aws_vpc" "main" {
  cidr_block = var.vpc_cidr

  # Both are required for EKS to resolve its endpoints and for RDS and
  # ElastiCache to be reachable by their DNS names instead of raw addresses.
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = { Name = local.name }
}

# ── Public tier ───────────────────────────────────────────────────────────────

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = { Name = local.name }
}

resource "aws_subnet" "public" {
  for_each = local.public_subnets

  vpc_id                  = aws_vpc.main.id
  availability_zone       = each.key
  cidr_block              = each.value
  map_public_ip_on_launch = true

  tags = {
    Name = "${local.name}-public-${each.key}"
    Tier = "public"
    # Tells the AWS Load Balancer Controller it may create internet-facing load
    # balancers here. Setting it now avoids reworking the subnets in phase 4.
    "kubernetes.io/role/elb" = "1"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = { Name = "${local.name}-public" }
}

resource "aws_route_table_association" "public" {
  for_each = aws_subnet.public

  subnet_id      = each.value.id
  route_table_id = aws_route_table.public.id
}

# ── Shared NAT Gateway ────────────────────────────────────────────────────────

# Since February 2024 every public IPv4 address is billed, including one that is
# attached, so this address adds roughly $0.005/hour on top of the gateway.
resource "aws_eip" "nat" {
  domain = "vpc"

  tags = { Name = "${local.name}-nat" }
}

resource "aws_nat_gateway" "main" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[local.azs[0]].id

  # The gateway is useless until the Internet Gateway exists, and OpenTofu
  # cannot infer that from the arguments alone.
  depends_on = [aws_internet_gateway.main]

  tags = { Name = local.name }
}

# ── Private application tier ──────────────────────────────────────────────────

resource "aws_subnet" "app" {
  for_each = local.app_subnets

  vpc_id            = aws_vpc.main.id
  availability_zone = each.key
  cidr_block        = each.value

  tags = {
    Name = "${local.name}-app-${each.key}"
    Tier = "app"
    # Marks these subnets as the place for internal load balancers.
    "kubernetes.io/role/internal-elb" = "1"
  }
}

# One shared route table: every application subnet reaches the internet through
# the single NAT Gateway, whichever zone it sits in.
resource "aws_route_table" "app" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.main.id
  }

  tags = { Name = "${local.name}-app" }
}

resource "aws_route_table_association" "app" {
  for_each = aws_subnet.app

  subnet_id      = each.value.id
  route_table_id = aws_route_table.app.id
}

# ── Private data tier ─────────────────────────────────────────────────────────

resource "aws_subnet" "data" {
  for_each = local.data_subnets

  vpc_id            = aws_vpc.main.id
  availability_zone = each.key
  cidr_block        = each.value

  tags = {
    Name = "${local.name}-data-${each.key}"
    Tier = "data"
  }
}

# No default route: this table only carries the VPC-local one AWS creates
# implicitly. Databases can be reached from inside the VPC and can reach nothing
# outside it, which also means their traffic never crosses the NAT Gateway.
resource "aws_route_table" "data" {
  vpc_id = aws_vpc.main.id

  tags = { Name = "${local.name}-data" }
}

resource "aws_route_table_association" "data" {
  for_each = aws_subnet.data

  subnet_id      = each.value.id
  route_table_id = aws_route_table.data.id
}

# ── S3 gateway endpoint ───────────────────────────────────────────────────────

# Gateway endpoints are free, and this one pays off twice: container image
# layers pulled from ECR are actually fetched from S3, and NAT Gateways charge
# about $0.045 per GB processed. Without it every image pull would be billed
# twice over — as NAT traffic and as transfer.
resource "aws_vpc_endpoint" "s3" {
  vpc_id            = aws_vpc.main.id
  service_name      = "com.amazonaws.${var.aws_region}.s3"
  vpc_endpoint_type = "Gateway"

  route_table_ids = [
    aws_route_table.app.id,
    aws_route_table.data.id,
  ]

  tags = { Name = "${local.name}-s3" }
}
