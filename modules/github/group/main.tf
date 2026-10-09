resource "github_team" "this" {
  name        = var.name
  description = var.description
  privacy     = "closed"
}

resource "github_team_membership" "this" {
  for_each = { for m in var.members : lower(m.user) => m }

  team_id  = github_team.this.id
  username = each.value.user
  role     = each.value.role
}
