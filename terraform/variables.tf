variable "aws_region" {
  description = "The target AWS deployment region"
  type        = string
  default     = "us-east-1"
}

variable "db_username" {
  description = "Database master administrator username"
  type        = string
  default     = "postgres"
}

variable "db_password" {
  description = "Database master administrator password"
  type        = string
  sensitive   = true
}
