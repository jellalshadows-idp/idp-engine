resource "github_repository" "this" {
  name                   = var.name
  description            = var.description
  visibility             = "public"
  auto_init              = true
  delete_branch_on_merge = true
  archive_on_destroy     = var.archive_on_destroy
}

# Topics have their own resource: github_repository's topics is Optional and
# Computed, so an empty list would leave the old topics in place. This resource
# does not clear them either: its create and update skip the API call for an
# empty set, and only a destroy clears the topics. So it exists only while the
# claim has topics, and removing the last topic is a delete, which the gate
# sends to approval.
resource "github_repository_topics" "this" {
  count = length(var.topics) > 0 ? 1 : 0

  repository = github_repository.this.name
  topics     = var.topics
}

resource "github_repository_vulnerability_alerts" "this" {
  repository = github_repository.this.name
  enabled    = true
}

resource "github_team_repository" "owner" {
  team_id    = var.owner_team_id
  repository = github_repository.this.name
  permission = "maintain"
}

resource "github_repository_ruleset" "default_branch" {
  name        = "idp-default-branch"
  repository  = github_repository.this.name
  target      = "branch"
  enforcement = "active"

  conditions {
    ref_name {
      include = ["~DEFAULT_BRANCH"]
      exclude = []
    }
  }

  bypass_actors {
    actor_id    = var.writer_app_id
    actor_type  = "Integration"
    bypass_mode = "always"
  }

  rules {
    deletion         = true
    non_fast_forward = true

    pull_request {
      required_approving_review_count = var.required_approvals
    }
  }
}

resource "github_repository_environment" "this" {
  for_each = var.environments

  environment = each.key
  repository  = github_repository.this.name

  dynamic "reviewers" {
    for_each = each.value.protected ? [1] : []
    content {
      teams = [var.owner_team_id]
    }
  }

  deployment_branch_policy {
    protected_branches     = false
    custom_branch_policies = true
  }

  # The team must have access to the repository before it can be a reviewer.
  depends_on = [github_team_repository.owner]
}

resource "github_repository_environment_deployment_policy" "default_branch" {
  for_each = var.environments

  repository     = github_repository.this.name
  environment    = github_repository_environment.this[each.key].environment
  branch_pattern = var.default_branch
}

locals {
  # One entry per environment variable, keyed "<env>/<name>". Environment names
  # never contain a slash (spec §4.6), so each key names exactly one variable,
  # and the addresses read github_actions_environment_variable.this["pro/AWS_REGION"].
  environment_variables = merge([
    for env, e in var.environments : {
      for name, value in e.variables : "${env}/${name}" => {
        environment = env
        name        = name
        value       = value
      }
    }
  ]...)
}

# The environments' GitHub Actions variables. The renderer passes them only for a
# Component with aws.registry: AWS_ROLE_ARN, ECR_REPOSITORY and AWS_REGION, computed
# from the names and the environment's aws block (spec §4.4, §5.5). Without
# variables this resource has no instances, so existing stacks plan no change.
resource "github_actions_environment_variable" "this" {
  for_each = local.environment_variables

  repository    = github_repository.this.name
  environment   = github_repository_environment.this[each.value.environment].environment
  variable_name = each.value.name
  value         = each.value.value
}
