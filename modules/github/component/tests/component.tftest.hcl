mock_provider "github" {}

variables {
  name               = "api"
  description        = "Orders API"
  topics             = ["java", "orders"]
  owner_team_id      = "4242"
  environments       = { dev = { protected = false }, pro = { protected = true } }
  required_approvals = 1
  archive_on_destroy = true
  writer_app_id      = 5255579
}

run "repository_is_public_and_follows_the_claim" {
  command = plan

  assert {
    condition     = github_repository.this.name == "api" && github_repository.this.visibility == "public"
    error_message = "the repository must be named after the claim and public (Free plan, spec §2.3)"
  }
  assert {
    condition     = github_repository.this.auto_init && github_repository.this.delete_branch_on_merge
    error_message = "auto_init and delete_branch_on_merge must be on (spec §4.4)"
  }
  assert {
    condition     = github_repository.this.archive_on_destroy
    error_message = "archive_on_destroy must follow the platform config"
  }
  assert {
    condition     = tolist(github_repository.this.topics) == tolist(["java", "orders"])
    error_message = "topics must follow the claim"
  }
}

run "archive_on_destroy_can_be_turned_off" {
  command = plan

  variables {
    archive_on_destroy = false
  }

  assert {
    condition     = !github_repository.this.archive_on_destroy
    error_message = "archive_on_destroy must follow the variable (false in idp-claims-e2e)"
  }
}

run "vulnerability_alerts_are_on" {
  command = plan

  assert {
    condition     = github_repository_vulnerability_alerts.this.enabled
    error_message = "vulnerability alerts must be on (spec §4.4)"
  }
}

run "owner_team_maintains_the_repository" {
  command = plan

  assert {
    condition     = github_team_repository.owner.permission == "maintain" && github_team_repository.owner.team_id == "4242"
    error_message = "the owner team must get maintain permission"
  }
}

run "ruleset_requires_prs_and_lets_only_the_writer_bypass" {
  command = plan

  assert {
    condition     = github_repository_ruleset.default_branch.enforcement == "active" && github_repository_ruleset.default_branch.target == "branch"
    error_message = "the ruleset must be an active branch ruleset"
  }
  assert {
    condition     = tolist(github_repository_ruleset.default_branch.conditions[0].ref_name[0].include) == tolist(["~DEFAULT_BRANCH"])
    error_message = "the ruleset must target the default branch"
  }
  assert {
    condition     = length(github_repository_ruleset.default_branch.bypass_actors) == 1 && github_repository_ruleset.default_branch.bypass_actors[0].actor_id == 5255579 && github_repository_ruleset.default_branch.bypass_actors[0].actor_type == "Integration"
    error_message = "only the writer App may bypass (spec §4.4)"
  }
  assert {
    condition     = github_repository_ruleset.default_branch.rules[0].deletion && github_repository_ruleset.default_branch.rules[0].non_fast_forward
    error_message = "deletion and force pushes must be blocked"
  }
  assert {
    condition     = github_repository_ruleset.default_branch.rules[0].pull_request[0].required_approving_review_count == 1
    error_message = "PR approvals must follow github.requiredApprovals"
  }
}

run "protected_environments_require_the_owner_team" {
  command = plan

  assert {
    condition     = length(github_repository_environment.this) == 2
    error_message = "one environment per claimed environment"
  }
  assert {
    condition     = length(github_repository_environment.this["pro"].reviewers) == 1 && contains(tolist(github_repository_environment.this["pro"].reviewers[0].teams), 4242)
    error_message = "a protected environment must require the owner team"
  }
  assert {
    condition     = length(github_repository_environment.this["dev"].reviewers) == 0
    error_message = "an unprotected environment must not require reviewers"
  }
}

run "environments_deploy_only_from_the_default_branch" {
  command = plan

  assert {
    condition     = alltrue([for e in github_repository_environment.this : !e.deployment_branch_policy[0].protected_branches && e.deployment_branch_policy[0].custom_branch_policies])
    error_message = "environments must use a custom branch policy"
  }
  assert {
    condition     = length(github_repository_environment_deployment_policy.default_branch) == 2 && alltrue([for p in github_repository_environment_deployment_policy.default_branch : p.branch_pattern == "main"])
    error_message = "every environment must deploy only from the default branch"
  }
}

run "rejects_out_of_range_approvals" {
  command = plan

  variables {
    required_approvals = 11
  }

  expect_failures = [var.required_approvals]
}
