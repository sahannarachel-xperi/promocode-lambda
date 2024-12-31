data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  base_function_name = "promocode-lambda"
  shortened_base_function_name = "promocode-lambda"
  function_name      = "${local.base_function_name}-${var.instance_name}"
  modified_instance_name = replace(var.instance_name, "." , "-")
  iam_name = substr("${local.base_function_name}-${var.instance_name}", 0, 59)
  tags = {
    TivoEnv     = var.instance_name
    TivoOwner   = var.tivo_owner
    TivoTTL     = "AlwaysOn"
    TivoService = "APS"
    TivoInfo    = "promocode-lambda"
  }
}

# IAM Role
resource "aws_iam_role" "lambda_role" {
  name        = "${local.base_function_name}-${var.instance_name}"
  description = "IAM Role for ${local.function_name}"
  tags        = local.tags
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Principal = {
        Service = ["lambda.amazonaws.com"]
      }
      Effect = "Allow"
    }]
  })
}

# IAM Policies
resource "aws_iam_policy" "lambda_policy" {
  name = "${local.base_function_name}-${var.instance_name}-policy"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:CreateLogGroup"
        ]
        Resource = "arn:aws:logs:*:${data.aws_caller_identity.current.account_id}:log-group:/aws/lambda/${local.function_name}:*"
      },
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:ListBucket",
          "s3:GetBucketNotification",
          "s3:PutBucketNotification"
        ]
        Resource = [
          "arn:aws:s3:::${var.s3_source_bucket_name}",
          "arn:aws:s3:::${var.s3_source_bucket_name}/qe-ft/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "dynamodb:PutItem",
          "dynamodb:UpdateItem",
          "dynamodb:GetItem",
          "dynamodb:DeleteItem",
          "dynamodb:Query",
          "dynamodb:Scan",
          "dynamodb:BatchWriteItem",
          "dynamodb:BatchGetItem",
          "dynamodb:DescribeTable"
        ]
        Resource = [
          "arn:aws:dynamodb:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:table/Campaigns",
          "arn:aws:dynamodb:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:table/Promocodes"
        ]
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_policy" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = aws_iam_policy.lambda_policy.arn
}

# Lambda Function
resource "aws_lambda_function" "lambda" {
  filename         = var.zipfile
  function_name    = local.function_name
  role             = aws_iam_role.lambda_role.arn
  handler          = "bootstrap"
  source_code_hash = filebase64sha256(var.zipfile)
  runtime          = "provided.al2"
  architectures    = ["arm64"]
  publish          = true
  timeout          = var.lambda_timeout

  environment {
    variables = {
      INSTANCE_NAME = var.instance_name
      LOG_LEVEL    = var.log_level
    }
  }

  logging_config {
    log_format = "JSON"
  }

  tags = local.tags
}

# S3 Event Trigger
resource "aws_s3_bucket_notification" "lambda_trigger" {
  bucket = var.s3_source_bucket_name

  lambda_function {
    lambda_function_arn = aws_lambda_function.lambda.arn
    events = [
      "s3:ObjectCreated:Put",
      "s3:ObjectCreated:Post",
      "s3:ObjectRemoved:*"
    ]
    filter_prefix = "qe-ft/"
  }
}

# S3 Lambda Permission - single permission is sufficient
resource "aws_lambda_permission" "s3_permission" {
  statement_id  = "AllowS3Invoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.lambda.function_name
  principal     = "s3.amazonaws.com"
  source_arn    = "arn:aws:s3:::${var.s3_source_bucket_name}"
}

# Add S3 bucket notification for redemption files
resource "aws_s3_bucket_notification" "bucket_notification" {
  bucket = var.s3_source_bucket_name

  lambda_function {
    lambda_function_arn = aws_lambda_function.lambda.arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = "qe-ft/redemptions/"
    filter_suffix       = ".csv"
  }
}

# CloudWatch Log Group
resource "aws_cloudwatch_log_group" "lambda_log_group" {
  name              = "/aws/lambda/${aws_lambda_function.lambda.function_name}"
  retention_in_days = var.retention_in_days
  tags              = local.tags
}

# CloudWatch Events Permission
resource "aws_lambda_permission" "cloudwatch_permission" {
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.lambda.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.lambda_event_rule.arn
}

resource "aws_cloudwatch_event_rule" "lambda_event_rule" {
  name          = "${local.shortened_base_function_name}-${var.instance_name}-event"
  description   = "This resource is used to create s3 object events for lambda function."
  state         = "ENABLED"
  tags          = local.tags
  event_pattern = jsonencode({
    source      = ["aws.s3"]
    detail-type = ["AWS API Call via CloudTrail"]
    detail = {
      eventSource = ["s3.amazonaws.com"]
      eventName   = [
        "PutObject",
        "DeleteObject",
        "DeleteObjects",
        "CompleteMultipartUpload"
      ]
      requestParameters = {
        bucketName = [var.s3_source_bucket_name]
        key = [{
          prefix = "qe-ft/"
        }]
      }
    }
  })
}

resource "aws_cloudwatch_event_target" "lambda_event_rule_target" {
  rule = aws_cloudwatch_event_rule.lambda_event_rule.name
  arn  = aws_lambda_function.lambda.arn
}


resource "aws_iam_policy" "dynamodb_policy" {
  name        = "${local.function_name}-dynamodb"
  description = "${local.function_name} Instance Role Policy.This policy includes all the DynamoDb permissions to all the tables needed by the service."
  policy = <<-EOF
  {
    "Version": "2012-10-17",
    "Statement": [
      {
        "Sid": "DynamoDBFullAccessCampaign",
        "Effect": "Allow",
        "Action": [
          "dynamodb:*"
        ],
        "Resource": ["arn:aws:dynamodb:${var.aws_region}:${var.aws_account}:table/${var.table_name_campaign}"]
      },
      {
        "Sid": "DynamoDBFullAccessPromocode",
        "Effect": "Allow",
        "Action": [
          "dynamodb:*"
        ],
        "Resource": ["arn:aws:dynamodb:${var.aws_region}:${var.aws_account}:table/${var.table_name_promocode}"]
      }
    ]
  }
  EOF
}

resource "aws_iam_role_policy_attachment" "dynamodb" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = aws_iam_policy.dynamodb_policy.arn
}

# Make sure Lambda has proper IAM role permissions
resource "aws_iam_role_policy_attachment" "lambda_s3" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = "arn:aws:iam::aws:policy/AWSLambdaExecute"
}

# Add specific S3 permissions if needed
resource "aws_iam_role_policy" "lambda_s3_policy" {
  name = "lambda_s3_policy"
  role = aws_iam_role.lambda_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:ListBucket"
        ]
        Resource = [
          "arn:aws:s3:::${var.s3_source_bucket_name}",
          "arn:aws:s3:::${var.s3_source_bucket_name}/*"
        ]
      }
    ]
  })
}