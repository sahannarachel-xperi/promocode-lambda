
variable "instance_name" {
  description = "Unique identifier for this deployment instance"
}

variable "zipfile" {
  description = "The file name of the zip file containing the Lambda function"
}

variable "tivo_owner" {
  description = "Owner tag for resources"
}

variable "datacenter" {
  description = "The deployed datacenter"
  default     = "tek2"
}

variable "log_level" {
  type        = string
  description = "Lambda logging level (DEBUG, INFO, WARN, ERROR)"
  default     = "INFO"

  validation {
    condition     = contains(["DEBUG", "INFO", "WARN", "ERROR"], var.log_level)
    error_message = "Valid values are (DEBUG, INFO, WARN, ERROR)"
  }
}

variable "lambda_timeout" {
  description = "Lambda timeout in seconds"
  default     = 900  # 15 minutes
}

variable "s3_source_bucket_name" {
  description = "S3 bucket containing campaigns and promocodes (ads-qrcode-promotions-qe-ft-tek2)"
}

variable "event_trigger_pattern" {
  description = "S3 event trigger pattern"
  default     = "qe-ft/*"  # Triggers on all files in qe-ft directory
}

variable "retention_in_days" {
  description = "CloudWatch logs retention period in days"
  default     = 7
}

variable "memory_size" {
  description = "Lambda function memory size in MB"
  default     = 128
}

variable "environment_variables" {
  description = "Environment variables for the Lambda function"
  type        = map(string)
  default = {
    LOG_LEVEL = "INFO"
  }
}

variable "event_trigger_file_name" {
  description = "The name of the file for which s3 events got trigger on any update"
}

variable "aws_region" {
  description = "The AWS region in which terraform operations will be performed"
}

/*variable "namespace" {
  description = "The namespace in which the service will be deployed"
}*/

variable aws_account {
  description = "The AWS account number"
}

variable "table_name_campaign" {
  description = "The Campaign table"
}

variable "table_name_promocode" {
  description = "The promocode table"
}