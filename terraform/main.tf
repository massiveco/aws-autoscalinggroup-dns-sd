resource "aws_autoscaling_notification" "dnssd_notifications" {
  group_names = [
    "${var.aws_autoscaling_group_name}",
  ]

  notifications = [
    "autoscaling:EC2_INSTANCE_LAUNCH",
    "autoscaling:EC2_INSTANCE_TERMINATE",
  ]

  topic_arn = "${aws_sns_topic.controllers_autoscaling.arn}"
}

resource "aws_sns_topic" "controllers_autoscaling" {
  display_name = "${var.aws_autoscaling_group_name} Autoscaling Events: DNS-SD"
}

resource "aws_sns_topic_subscription" "controllers_autoscaling" {
  topic_arn = "${aws_sns_topic.controllers_autoscaling.arn}"
  protocol  = "lambda"
  endpoint  = "${aws_lambda_function.controllers_dns_sd.arn}"
}

resource "aws_iam_role" "controllers_dns_sd" {
  name_prefix = "${var.aws_autoscaling_group_name}_ds_"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "aws_iam_role_policy" "controllers_dns_sd_logging" {
  name_prefix = "${var.aws_autoscaling_group_name}_ds_logging_"
  role        = "${aws_iam_role.controllers_dns_sd.id}"

  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "logs:PutLogEvents",
            "Resource": "arn:aws:logs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:log-group:*:*:*"
        },
        {
            "Effect": "Allow",
            "Action": [
                "logs:CreateLogStream",
                "logs:PutLogEvents"
            ],
            "Resource": "arn:aws:logs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:log-group:*"
        },
        {
            "Effect": "Allow",
            "Action": "logs:PutLogEvents",
            "Resource": "arn:aws:logs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:log-group:*"
        },
        {
            "Effect": "Allow",
            "Action": "logs:CreateLogGroup",
            "Resource": "*"
        }
    ]
}
EOF
}

resource "aws_iam_role_policy" "controllers_dns_sd" {
  name_prefix = "${var.aws_autoscaling_group_name}_ds_"
  role        = "${aws_iam_role.controllers_dns_sd.id}"

  //TODO: Restrict this to a single hosted zone
  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ec2:DescribeInstances",
                "autoscaling:DescribeAutoScalingGroups"
            ],
            "Resource": "*"
        },
        {
            "Effect": "Allow",
            "Action": [
                "route53:GetHostedZone",
                "route53:ChangeResourceRecordSets",
                "route53:ListResourceRecordSets"
            ],
            "Resource": "arn:aws:route53:::hostedzone/*"
        }
    ]
}
EOF
}

resource "aws_lambda_function" "controllers_dns_sd" {
  s3_bucket = "ma.ssive.co"
  s3_key    = "lambdas/massive_autoscaling_dns_sd.zip"

  function_name = "asg_${var.aws_autoscaling_group_name}_dns_sd"
  role          = "${aws_iam_role.controllers_dns_sd.arn}"
  handler       = "aws-autoscalinggroup-dns-sd"
  runtime       = "go1.x"
  description   = "Ma.ssive Autoscaling DNS-SD for: ${var.aws_autoscaling_group_name}"
}

resource "aws_lambda_permission" "with_sns" {
  statement_id  = "AllowExecutionFromSNS"
  action        = "lambda:InvokeFunction"
  function_name = "${aws_lambda_function.controllers_dns_sd.arn}"
  principal     = "sns.amazonaws.com"
  source_arn    = "${aws_sns_topic.controllers_autoscaling.arn}"
}
