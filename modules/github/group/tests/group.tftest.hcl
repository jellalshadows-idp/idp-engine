mock_provider "github" {}

variables {
  name        = "platform"
  description = "Platform team"
  members = [
    { user = "alice", role = "maintainer" },
    { user = "bob", role = "member" },
  ]
}

run "team_is_closed_and_named_after_the_claim" {
  command = plan

  assert {
    condition     = github_team.this.name == "platform"
    error_message = "the team must be named after the Group claim"
  }
  assert {
    condition     = github_team.this.privacy == "closed"
    error_message = "teams must be closed (visible to org members), spec §4.3"
  }
}

run "one_membership_per_member_with_its_role" {
  command = plan

  assert {
    condition     = length(github_team_membership.this) == 2
    error_message = "expected one membership per member"
  }
  assert {
    condition     = github_team_membership.this["alice"].role == "maintainer" && github_team_membership.this["bob"].role == "member"
    error_message = "membership roles must follow the claim"
  }
}

run "memberships_are_keyed_by_lowercase_login" {
  command = plan

  variables {
    members = [{ user = "Alice", role = "maintainer" }]
  }

  assert {
    condition     = github_team_membership.this["alice"].username == "Alice"
    error_message = "the key must be the lowercase login while the username keeps its case"
  }
}

run "rejects_logins_differing_only_by_case" {
  command = plan

  variables {
    members = [
      { user = "Alice", role = "member" },
      { user = "alice", role = "member" },
    ]
  }

  expect_failures = [var.members]
}

run "rejects_unknown_roles" {
  command = plan

  variables {
    members = [{ user = "carol", role = "owner" }]
  }

  expect_failures = [var.members]
}
