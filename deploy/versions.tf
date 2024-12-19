terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    itsi = {
      source  = "TiVo/splunk-itsi"
      version = "~> 1.0"
    }
    splunk = {
      source = "splunk/splunk"
    }
  }
  required_version = "~> 1.5"
}