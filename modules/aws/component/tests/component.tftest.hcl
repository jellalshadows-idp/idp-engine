mock_provider "aws" {
  # A mock provider invents computed values; the push policy's resource is the
  # repository's ARN, so pin it to the form AWS returns.
  mock_resource "aws_ecr_repository" {
    defaults = {
      arn = "arn:aws:ecr:eu-west-1:000000000001:repository/api"
    }
  }
}

# The values the renderer passes from internal/naming for Component api in dev,
# account 000000000001 (spec §5.5; naming's TestIdentities).
variables {
  github_repository = "acme/api"
  environment       = "dev"
  repository_name   = "api"
  role_name         = "api-dev-ci"
  oidc_provider_arn = "arn:aws:iam::000000000001:oidc-provider/token.actions.githubusercontent.com"
}

run "names_are_the_ones_naming_computed" {
  command = plan

  assert {
    condition     = aws_ecr_repository.this.name == "api"
    error_message = "the repository must be named repository_name, naming.RepositoryName (spec §5.5)"
  }
  assert {
    condition     = aws_iam_role.ci.name == "api-dev-ci" && coalesce(aws_iam_role.ci.path, "/") == "/"
    error_message = "the role must be named role_name, naming.RoleName, at the default path, so its ARN is naming.RoleARN (spec §5.5)"
  }
}

run "the_trust_policy_admits_only_the_components_ci_in_its_environment" {
  command = plan

  assert {
    condition     = jsondecode(aws_iam_role.ci.assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:sub"] == "repo:acme/api:environment:dev"
    error_message = "the trust policy's sub must be repo:<org>/<name>:environment:<env> (spec §4.4, §8.3)"
  }
  assert {
    condition     = jsondecode(aws_iam_role.ci.assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:aud"] == "sts.amazonaws.com"
    error_message = "the trust policy's aud must be sts.amazonaws.com (spec §4.4, §8.3)"
  }
  assert {
    condition = jsondecode(aws_iam_role.ci.assume_role_policy) == {
      Version = "2012-10-17"
      Statement = [{
        Effect    = "Allow"
        Principal = { Federated = "arn:aws:iam::000000000001:oidc-provider/token.actions.githubusercontent.com" }
        Action    = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
            "token.actions.githubusercontent.com:sub" = "repo:acme/api:environment:dev"
          }
        }
      }]
    }
    error_message = "the trust policy must be one statement: the account's GitHub OIDC provider, sts:AssumeRoleWithWebIdentity, and exact matches on aud and sub, nothing else"
  }
}

run "the_subject_follows_the_environment" {
  command = plan

  variables {
    environment = "pro"
    role_name   = "api-pro-ci"
  }

  assert {
    condition     = jsondecode(aws_iam_role.ci.assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:sub"] == "repo:acme/api:environment:pro"
    error_message = "the role of one environment must not admit another environment's CI"
  }
}

run "the_ci_role_may_only_push_to_its_repository" {
  command = plan

  assert {
    condition     = aws_iam_role_policy.push.role == "api-dev-ci"
    error_message = "the push policy must belong to the CI role"
  }
  assert {
    condition = jsondecode(aws_iam_role_policy.push.policy) == {
      Version = "2012-10-17"
      Statement = [
        {
          Sid      = "ECRLogin"
          Effect   = "Allow"
          Action   = ["ecr:GetAuthorizationToken"]
          Resource = "*"
        },
        {
          Sid      = "ECRPush"
          Effect   = "Allow"
          Action   = ["ecr:BatchCheckLayerAvailability", "ecr:CompleteLayerUpload", "ecr:InitiateLayerUpload", "ecr:PutImage", "ecr:UploadLayerPart"]
          Resource = "arn:aws:ecr:eu-west-1:000000000001:repository/api"
        },
      ]
    }
    error_message = "the role may log in to the registry (an account-level action) and push to its own repository, nothing else (spec §4.4)"
  }
}

run "the_repository_keeps_its_tags_and_scans_pushes" {
  command = plan

  assert {
    condition     = aws_ecr_repository.this.image_tag_mutability == "IMMUTABLE"
    error_message = "a pushed tag must never move"
  }
  assert {
    condition     = aws_ecr_repository.this.image_scanning_configuration[0].scan_on_push
    error_message = "every push must be scanned"
  }
  assert {
    condition     = aws_ecr_repository.this.force_delete
    error_message = "deleting the Component must delete its repository even with images in it (spec §6.3)"
  }
}

run "rejects_another_issuer" {
  command = plan

  variables {
    oidc_provider_arn = "arn:aws:iam::000000000001:oidc-provider/accounts.google.com"
  }

  expect_failures = [var.oidc_provider_arn]
}

run "rejects_a_wildcard_repository" {
  command = plan

  variables {
    github_repository = "acme/*"
  }

  expect_failures = [var.github_repository]
}

run "rejects_an_environment_outside_the_name_rule" {
  command = plan

  variables {
    environment = "*"
  }

  expect_failures = [var.environment]
}
