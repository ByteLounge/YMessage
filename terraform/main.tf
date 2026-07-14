terraform {
  required_version = ">= 1.5.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# --- VPC NETWORK AND SUBNETS ---
resource "aws_vpc" "ymessage_vpc" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  tags = {
    Name = "ymessage-production-vpc"
  }
}

resource "aws_subnet" "public_subnet" {
  count             = 2
  vpc_id            = aws_vpc.ymessage_vpc.id
  cidr_block        = "10.0.${count.index}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]
  tags = {
    Name = "ymessage-public-subnet-${count.index}"
  }
}

resource "aws_subnet" "private_subnet" {
  count             = 2
  vpc_id            = aws_vpc.ymessage_vpc.id
  cidr_block        = "10.0.${count.index + 10}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]
  tags = {
    Name = "ymessage-private-subnet-${count.index}"
  }
}

data "aws_availability_zones" "available" {}

# --- DATABASE (RDS POSTGRESQL) ---
resource "aws_db_subnet_group" "db_subnets" {
  name       = "ymessage-db-subnet-group"
  subnet_ids = aws_subnet.private_subnet[*].id
}

resource "aws_db_instance" "postgres" {
  identifier             = "ymessage-db"
  allocated_storage      = 20
  max_allocated_storage  = 100
  engine                 = "postgres"
  engine_version         = "15.4"
  instance_class         = "db.t4g.micro"
  db_name                = "ymessage"
  username               = var.db_username
  password               = var.db_password
  db_subnet_group_name   = aws_db_subnet_group.db_subnets.name
  skip_final_snapshot    = true
  vpc_security_group_ids = [aws_security_group.db_sg.id]
}

# --- CACHE (ELASTICACHE REDIS) ---
resource "aws_elasticache_subnet_group" "redis_subnets" {
  name       = "ymessage-redis-subnet-group"
  subnet_ids = aws_subnet.private_subnet[*].id
}

resource "aws_elasticache_replication_group" "redis" {
  replication_group_id        = "ymessage-redis"
  description                 = "Redis cluster for YMessage pub-sub and caching"
  node_type                   = "cache.t4g.micro"
  num_cache_clusters          = 1
  parameter_group_name        = "default.redis7"
  port                        = 6379
  subnet_group_name           = aws_elasticache_subnet_group.redis_subnets.name
  security_group_ids          = [aws_security_group.redis_sg.id]
  at_rest_encryption_enabled = true
  transit_encryption_enabled  = true
}

# --- OBJECT STORAGE (S3 BUCKET) ---
resource "aws_s3_bucket" "media_bucket" {
  bucket        = "ymessage-media-production-storage"
  force_destroy = false
}

resource "aws_s3_bucket_public_access_block" "public_block" {
  bucket = aws_s3_bucket.media_bucket.id

  block_public_acls       = false
  block_public_policy     = false
  ignore_public_acls      = false
  restrict_public_buckets = false
}

# --- SECURITY GROUPS ---
resource "aws_security_group" "db_sg" {
  name        = "ymessage-db-sg"
  description = "Access to Postgres"
  vpc_id      = aws_vpc.ymessage_vpc.id

  ingress {
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/16"]
  }
}

resource "aws_security_group" "redis_sg" {
  name        = "ymessage-redis-sg"
  description = "Access to Redis"
  vpc_id      = aws_vpc.ymessage_vpc.id

  ingress {
    from_port   = 6379
    to_port     = 6379
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/16"]
  }
}
