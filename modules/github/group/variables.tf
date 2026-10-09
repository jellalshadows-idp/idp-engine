variable "name" {
  description = "Team name and slug: the Group claim name."
  type        = string
}

variable "description" {
  description = "Team description."
  type        = string
  default     = ""
}

variable "members" {
  description = "Team members: GitHub login and role (maintainer or member)."
  type = list(object({
    user = string
    role = string
  }))

  validation {
    condition     = alltrue([for m in var.members : contains(["maintainer", "member"], m.role)])
    error_message = "Each member role must be maintainer or member."
  }
}
