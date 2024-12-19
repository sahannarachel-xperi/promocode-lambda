output "lambda_invoke_arn" {
  description = "ARN to be used for invoking the promocode Lambda function"
  value       = aws_lambda_function.lambda.invoke_arn
}

output "lambda_arn" {
  description = "ARN of the promocode Lambda function"
  value       = aws_lambda_function.lambda.arn
}

output "lambda_function_name" {
  description = "Name of the promocode Lambda function"
  value       = aws_lambda_function.lambda.function_name
}

output "s3_trigger_configuration" {
  description = "S3 event trigger configuration"
  value = {
    bucket = var.s3_source_bucket_name
    prefix = "qe-ft/"
  }
}

output "dynamodb_table_names" {
  description = "DynamoDB tables used by the Lambda"
  value = {
    campaigns  = "ads-qrcode-promotions-qe-ft-campaign"
    promocodes = "ads-qrcode-promotions-qe-ft-promocode"
  }
}