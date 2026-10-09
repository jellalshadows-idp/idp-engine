output "team_id" {
  description = "Team id, used by Component modules for repository access and environment reviewers."
  value       = github_team.this.id
}

output "slug" {
  description = "Team slug."
  value       = github_team.this.slug
}
