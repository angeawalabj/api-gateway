variable "identifier"        { type = string }
variable "db_name"           { type = string  default = "gateway" }
variable "username"          { type = string  default = "gwuser" }
variable "password"          { type = string  sensitive = true }
variable "subnet_ids"        { type = list(string) }
variable "security_group_id" { type = string }
variable "environment"       { type = string }

resource "aws_db_subnet_group" "this" {
  name       = "${var.identifier}-subnet-group"
  subnet_ids = var.subnet_ids
  tags       = { Environment = var.environment }
}

resource "aws_db_instance" "this" {
  identifier            = var.identifier
  engine                = "postgres"
  engine_version        = "16.1"
  instance_class        = "db.t3.micro"   # Free Tier
  allocated_storage     = 20
  max_allocated_storage = 20

  db_name  = var.db_name
  username = var.username
  password = var.password

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [var.security_group_id]

  multi_az                = false
  publicly_accessible     = false
  skip_final_snapshot     = true
  backup_retention_period = 1

  tags = { Name = var.identifier, Environment = var.environment }
}

output "endpoint" { value = aws_db_instance.this.endpoint }
output "port"     { value = aws_db_instance.this.port }
output "dsn" {
  value     = "postgres://${var.username}:${var.password}@${aws_db_instance.this.endpoint}/${var.db_name}?sslmode=require"
  sensitive = true
}
