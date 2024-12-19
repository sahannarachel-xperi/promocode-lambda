variable "instance_name" {}
variable "tivo_owner" {}

data "aws_region" "current" {}

data "aws_caller_identity" "current" {}

provider "aws" {
  region = local.aws_region
}

locals {
  datacenter              = "tek2"
  aws_region             = "us-east-1"
  event_trigger_file_name = "qe-ft/*"  # Triggers on all files in qe-ft directory
  retention_in_days      = 7
  instance_name          = substr("${var.instance_name}", 0, 7)
  period                 = "3600"
}

terraform {
  backend "s3" {
    encrypt        = true
    bucket         = "tivo-inception-serverless-terraform-storage"
    dynamodb_table = "tivo-inception-serverless-terraform-locking"
    region         = "us-west-2"
  }
}

module "promocode-lambda" {
  source                     = "../terraform"
  instance_name              = local.instance_name
  tivo_owner                 = var.tivo_owner
  zipfile                    = "../promocode-lambda.zip"
  log_level                  = "INFO"
  datacenter                 = local.datacenter
  event_trigger_file_name    = local.event_trigger_file_name
  retention_in_days          = local.retention_in_days
  s3_source_bucket_name      = "ads-qrcode-promotions-qe-ft-tek2"
  table_name_campaign        = "ads-qrcode-promotions-qe-ft-campaign"
  table_name_promocode       = "ads-qrcode-promotions-qe-ft-promocode"
  aws_region                 = data.aws_region.current.name
  aws_account                = data.aws_caller_identity.current.account_id
  # Your S3 bucket name

  environment_variables = {
    INSTANCE_NAME = local.instance_name
    LOG_LEVEL     = "INFO"
    AWS_REGION    = local.aws_region
  }
}
