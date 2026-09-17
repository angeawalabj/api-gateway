variable "ami_id"            { type = string }
variable "instance_type"     { type = string  default = "t2.micro" }
variable "subnet_id"         { type = string }
variable "vpc_id"            { type = string }
variable "key_name"          { type = string }
variable "s3_bucket"         { type = string }
variable "postgres_dsn"      { type = string  sensitive = true }
variable "redis_addr"        { type = string }
variable "environment"       { type = string  default = "dev" }
variable "admin_cidr"        { type = string  default = "0.0.0.0/0" }
variable "gateway_version"   { type = string  default = "latest" }

resource "aws_security_group" "gateway" {
  name        = "api-gateway-${var.environment}-sg"
  description = "API Gateway security group"
  vpc_id      = var.vpc_id

  ingress { from_port = 8080  to_port = 8080  protocol = "tcp" cidr_blocks = ["0.0.0.0/0"] description = "Gateway HTTP" }
  ingress { from_port = 9090  to_port = 9090  protocol = "tcp" cidr_blocks = ["10.0.0.0/8"] description = "Prometheus" }
  ingress { from_port = 22    to_port = 22    protocol = "tcp" cidr_blocks = [var.admin_cidr] description = "SSH" }
  egress  { from_port = 0     to_port = 0     protocol = "-1"  cidr_blocks = ["0.0.0.0/0"] }

  tags = { Name = "api-gateway-${var.environment}-sg", Environment = var.environment }
}

resource "aws_iam_role" "gateway" {
  name = "api-gateway-${var.environment}-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{ Action = "sts:AssumeRole", Effect = "Allow", Principal = { Service = "ec2.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy" "s3_read" {
  name = "gateway-s3-read"
  role = aws_iam_role.gateway.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = ["s3:GetObject", "s3:ListBucket"],
      Resource = ["arn:aws:s3:::${var.s3_bucket}", "arn:aws:s3:::${var.s3_bucket}/*"] }]
  })
}

resource "aws_iam_instance_profile" "gateway" {
  name = "api-gateway-${var.environment}-profile"
  role = aws_iam_role.gateway.name
}

resource "aws_instance" "gateway" {
  ami                    = var.ami_id
  instance_type          = var.instance_type
  subnet_id              = var.subnet_id
  vpc_security_group_ids = [aws_security_group.gateway.id]
  key_name               = var.key_name
  iam_instance_profile   = aws_iam_instance_profile.gateway.name

  user_data = base64encode(templatefile("${path.module}/user_data.sh.tpl", {
    s3_bucket       = var.s3_bucket
    gateway_version = var.gateway_version
    postgres_dsn    = var.postgres_dsn
    redis_addr      = var.redis_addr
    environment     = var.environment
  }))

  tags = { Name = "api-gateway-${var.environment}", Environment = var.environment, ManagedBy = "terraform" }

  lifecycle { create_before_destroy = true }
}

output "instance_id"      { value = aws_instance.gateway.id }
output "public_ip"        { value = aws_instance.gateway.public_ip }
output "security_group_id"{ value = aws_security_group.gateway.id }
