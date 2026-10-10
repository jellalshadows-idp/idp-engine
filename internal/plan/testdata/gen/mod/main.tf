variable "roles" {
  type = map(string)
}

resource "terraform_data" "member" {
  for_each = var.roles
  input    = each.value
}
