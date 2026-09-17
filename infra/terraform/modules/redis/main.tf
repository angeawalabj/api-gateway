variable "cluster_id"        { type = string }
variable "subnet_ids"        { type = list(string) }
variable "security_group_id" { type = string }
variable "environment"       { type = string }

resource "aws_elasticache_subnet_group" "this" {
  name       = "${var.cluster_id}-subnet-group"
  subnet_ids = var.subnet_ids
}

resource "aws_elasticache_cluster" "this" {
  cluster_id           = var.cluster_id
  engine               = "redis"
  engine_version       = "7.1"
  node_type            = "cache.t3.micro"   # Free Tier
  num_cache_nodes      = 1
  parameter_group_name = "default.redis7"
  port                 = 6379

  subnet_group_name  = aws_elasticache_subnet_group.this.name
  security_group_ids = [var.security_group_id]

  snapshot_retention_limit = 0

  tags = { Name = var.cluster_id, Environment = var.environment }
}

output "endpoint" {
  value = "${aws_elasticache_cluster.this.cache_nodes[0].address}:${aws_elasticache_cluster.this.port}"
}
