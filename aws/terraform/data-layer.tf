# Managed replacements for the two StatefulSets the local cluster runs:
#
#   Redis      -> ElastiCache   idempotency keys of user-manager
#   PostgreSQL -> RDS           user records of user-manager
#
# Neo4j has no managed AWS equivalent here and stays on Neo4j Aura, outside the
# account. Its credentials are put into Parameter Store by hand so that they
# never pass through OpenTofu, and therefore never land in the state file.
#
# Both engines sit in the data subnets, which have no route to the internet, and
# each is fronted by its own security group.

# ── Security groups ───────────────────────────────────────────────────────────

# Ingress is granted to the application subnet ranges rather than to another
# security group, because the EKS nodes do not exist yet. Phase 4 can narrow
# this to a security-group reference, which keeps working if the CIDRs change.
resource "aws_security_group" "postgres" {
  name        = "${local.name}-postgres"
  description = "PostgreSQL access from the application subnets"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "PostgreSQL from the application tier"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = values(local.app_subnets)
  }

  # No egress rule: RDS never initiates outbound connections, and the data
  # subnets could not route them anywhere in any case.

  tags = { Name = "${local.name}-postgres" }
}

resource "aws_security_group" "redis" {
  name        = "${local.name}-redis"
  description = "Redis access from the application subnets"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "Redis from the application tier"
    from_port   = 6379
    to_port     = 6379
    protocol    = "tcp"
    cidr_blocks = values(local.app_subnets)
  }

  tags = { Name = "${local.name}-redis" }
}

# ── PostgreSQL ────────────────────────────────────────────────────────────────

resource "aws_db_subnet_group" "postgres" {
  name       = "${local.name}-postgres"
  subnet_ids = [for s in aws_subnet.data : s.id]

  tags = { Name = "${local.name}-postgres" }
}

# The generated password is stored in the state file in clear text — that is how
# OpenTofu works, and the reason the state bucket is private and encrypted. RDS
# rejects '/', '@', '"' and spaces in master passwords.
resource "random_password" "postgres" {
  length           = 32
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

resource "aws_db_instance" "postgres" {
  identifier     = "${local.name}-postgres"
  engine         = "postgres"
  engine_version = "16"
  instance_class = var.postgres_instance_class

  # gp2 is what the free tier covers, together with 20 GB of storage.
  allocated_storage = var.postgres_storage_gb
  storage_type      = "gp2"
  storage_encrypted = true

  db_name  = "user_manager"
  username = "user_manager"
  password = random_password.postgres.result

  db_subnet_group_name   = aws_db_subnet_group.postgres.name
  vpc_security_group_ids = [aws_security_group.postgres.id]
  publicly_accessible    = false

  # The reference diagram shows RDS in both zones, which denotes a Multi-AZ
  # deployment: one database with a standby, not two databases. It is disabled
  # here because the free tier does not cover the standby, the same trade-off
  # already accepted for the shared NAT Gateway. Setting this to true is all it
  # takes to restore the diagram's design.
  multi_az = false

  # This environment is created and destroyed repeatedly, so retaining backups
  # would only add storage cost and slow every teardown down. A final snapshot
  # would also make the destroy workflow fail unless it were named uniquely.
  backup_retention_period = 0
  skip_final_snapshot     = true
  deletion_protection     = false

  # Performance Insights and enhanced monitoring are billable beyond the free
  # tier, and phase 6 covers observability through Prometheus instead.
  performance_insights_enabled = false

  apply_immediately = true

  tags = { Name = "${local.name}-postgres" }
}

# ── Redis ─────────────────────────────────────────────────────────────────────

resource "aws_elasticache_subnet_group" "redis" {
  name       = "${local.name}-redis"
  subnet_ids = [for s in aws_subnet.data : s.id]

  tags = { Name = "${local.name}-redis" }
}

# A single node, which is what the free tier covers. AUTH and encryption in
# transit would require a replication group and a larger node type, so access is
# controlled by the security group and by the subnet having no route out — the
# same posture as the in-cluster Redis of the local deployment.
resource "aws_elasticache_cluster" "redis" {
  cluster_id           = "${local.name}-redis"
  engine               = "redis"
  node_type            = var.redis_node_type
  num_cache_nodes      = 1
  parameter_group_name = "default.redis7"
  port                 = 6379

  subnet_group_name  = aws_elasticache_subnet_group.redis.name
  security_group_ids = [aws_security_group.redis.id]

  tags = { Name = "${local.name}-redis" }
}

# ── Connection details in Parameter Store ─────────────────────────────────────

# Phase 4 turns these into a Kubernetes Secret at deploy time, the same way the
# local pipeline builds app-secrets from the Gitea secrets. Standard-tier
# parameters are free.
locals {
  ssm_prefix = "/${var.project_name}/${var.environment}"
}

resource "aws_ssm_parameter" "postgres_host" {
  name  = "${local.ssm_prefix}/postgres/host"
  type  = "String"
  value = aws_db_instance.postgres.address
}

resource "aws_ssm_parameter" "postgres_port" {
  name  = "${local.ssm_prefix}/postgres/port"
  type  = "String"
  value = tostring(aws_db_instance.postgres.port)
}

resource "aws_ssm_parameter" "postgres_db" {
  name  = "${local.ssm_prefix}/postgres/db"
  type  = "String"
  value = aws_db_instance.postgres.db_name
}

resource "aws_ssm_parameter" "postgres_user" {
  name  = "${local.ssm_prefix}/postgres/user"
  type  = "String"
  value = aws_db_instance.postgres.username
}

resource "aws_ssm_parameter" "postgres_password" {
  name  = "${local.ssm_prefix}/postgres/password"
  type  = "SecureString"
  value = random_password.postgres.result
}

resource "aws_ssm_parameter" "redis_host" {
  name  = "${local.ssm_prefix}/redis/host"
  type  = "String"
  value = aws_elasticache_cluster.redis.cache_nodes[0].address
}

resource "aws_ssm_parameter" "redis_port" {
  name  = "${local.ssm_prefix}/redis/port"
  type  = "String"
  value = tostring(aws_elasticache_cluster.redis.cache_nodes[0].port)
}
