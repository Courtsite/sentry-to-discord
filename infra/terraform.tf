terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 3.27"
    }
  }

  backend "s3" {
    bucket         = "sentry-to-discord-terraform-state"
    key            = "prod.tfstate"
    region         = "us-east-1"
    profile        = "srp"
    dynamodb_table = "sentry-to-discord-terraform-locks"
    encrypt        = true
  }

  required_version = ">= 0.14.9"
}

provider "aws" {
  profile = "srp"
  region  = "us-east-1"
}
