data "archive_file" "lambda_zip" {
  type        = "zip"
  source_file = "../build/sentry-to-discord"
  output_path = "../build/sentry-to-discord.zip"
}

resource "aws_lambda_function" "lambda_func" {
  filename         = data.archive_file.lambda_zip.output_path
  function_name    = var.app_name
  handler          = "sentry-to-discord"
  source_code_hash = base64sha256(data.archive_file.lambda_zip.output_path)
  runtime          = "go1.x"
  role             = aws_iam_role.lambda_exec.arn

  environment {
    variables = {
      DISCORD_WEBHOOK_URL = var.discord_webhook_url
      CLIENT_SECRET = var.client_secret
    }
  }
}

# Assume role setup
resource "aws_iam_role" "lambda_exec" {
  name_prefix = var.app_name

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF

}

# Attach role to Managed Policy
variable "iam_policy_arn" {
  description = "IAM Policy to be attached to role"
  type        = list(string)

  default = [
    "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  ]
}

resource "aws_iam_policy_attachment" "role_attach" {
  name       = "policy-${var.app_name}"
  roles      = [aws_iam_role.lambda_exec.id]
  count      = length(var.iam_policy_arn)
  policy_arn = element(var.iam_policy_arn, count.index)
}
