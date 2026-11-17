locals {
  # The issuer of GitHub Actions' OIDC tokens (naming.OIDCProviderHost): its
  # condition keys are <issuer>:aud and <issuer>:sub.
  issuer = "token.actions.githubusercontent.com"
}

# The Component's image repository in this environment's account (spec §4.4).
resource "aws_ecr_repository" "this" {
  name = var.repository_name

  # A pushed tag never moves, so a later CI run cannot replace an image that is
  # already deployed under it.
  image_tag_mutability = "IMMUTABLE"

  # Deleting the Component deletes its repository with any images in it (spec
  # §6.3). Every delete already waits for approval in idp-approval, and the
  # images are build output of the archived repository's commits.
  force_delete = true

  image_scanning_configuration {
    scan_on_push = true
  }
}

# The role the Component's CI assumes in this environment (spec §4.4): only a
# GitHub Actions job of the Component's repository, running in this
# environment, with a token for STS.
resource "aws_iam_role" "ci" {
  name = var.role_name

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Federated = var.oidc_provider_arn }
      Action    = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "${local.issuer}:aud" = "sts.amazonaws.com"
          "${local.issuer}:sub" = "repo:${var.github_repository}:environment:${var.environment}"
        }
      }
    }]
  })
}

# Push to this repository, and nothing else: no pull, no other repository.
resource "aws_iam_role_policy" "push" {
  name = "ecr-push"
  role = aws_iam_role.ci.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        # docker login. The action is account-level: IAM accepts only "*".
        Sid      = "ECRLogin"
        Effect   = "Allow"
        Action   = ["ecr:GetAuthorizationToken"]
        Resource = "*"
      },
      {
        Sid    = "ECRPush"
        Effect = "Allow"
        Action = [
          "ecr:BatchCheckLayerAvailability",
          "ecr:CompleteLayerUpload",
          "ecr:InitiateLayerUpload",
          "ecr:PutImage",
          "ecr:UploadLayerPart",
        ]
        Resource = aws_ecr_repository.this.arn
      },
    ]
  })
}
