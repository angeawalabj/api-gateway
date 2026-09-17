terraform {
  required_version = ">= 1.7"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 5.0" }
  }
  backend "local" { path = "terraform.tfstate" }
}

provider "aws" {
  region = var.aws_region
  default_tags {
    tags = { Project = "api-gateway", Environment = "dev", ManagedBy = "terraform" }
  }
}

variable "aws_region"  { type = string  default = "eu-west-1" }
variable "key_name"    { type = string  description = "Nom de la keypair EC2 existante" }
variable "db_password" { type = string  sensitive = true }
variable "s3_bucket"   { type = string  description = "Bucket S3 contenant le binaire gateway" }
variable "admin_cidr"  { type = string  default = "0.0.0.0/0" }

data "aws_vpc" "default" { default = true }

data "aws_subnets" "default" {
  filter { name = "vpc-id", values = [data.aws_vpc.default.id] }
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]
  filter { name = "name", values = ["ubuntu/images/hvm-ssd/ubuntu-*-24.04-amd64-server-*"] }
}

module "redis" {
  source            = "../../modules/redis"
  cluster_id        = "api-gateway-dev-redis"
  subnet_ids        = data.aws_subnets.default.ids
  security_group_id = module.ec2.security_group_id
  environment       = "dev"
}

module "rds" {
  source            = "../../modules/rds"
  identifier        = "api-gateway-dev-pg"
  password          = var.db_password
  subnet_ids        = data.aws_subnets.default.ids
  security_group_id = module.ec2.security_group_id
  environment       = "dev"
}

module "ec2" {
  source          = "../../modules/ec2"
  ami_id          = data.aws_ami.ubuntu.id
  instance_type   = "t2.micro"
  subnet_id       = tolist(data.aws_subnets.default.ids)[0]
  vpc_id          = data.aws_vpc.default.id
  key_name        = var.key_name
  s3_bucket       = var.s3_bucket
  postgres_dsn    = module.rds.dsn
  redis_addr      = module.redis.endpoint
  environment     = "dev"
  admin_cidr      = var.admin_cidr
}

output "gateway_url"   { value = "http://${module.ec2.public_ip}:8080" }
output "gateway_ip"    { value = module.ec2.public_ip }
output "redis_endpoint"{ value = module.redis.endpoint }
output "rds_endpoint"  { value = module.rds.endpoint }
