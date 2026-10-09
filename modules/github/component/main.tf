resource "github_repository" "this" {
  name                   = var.name
  description            = var.description
  visibility             = "public"
  auto_init              = true
  delete_branch_on_merge = true
  archive_on_destroy     = var.archive_on_destroy
  topics                 = var.topics
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
