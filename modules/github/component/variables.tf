variable "name" {
  description = "Repository name: the Component claim name."
  type        = string
}

variable "description" {
  description = "Repository description."
  type        = string
  default     = ""
}

variable "topics" {
  description = "Repository topics."
  type        = list(string)
  default     = []
}

variable "owner_team_id" {
  description = "Id of the owner Group's team (output team_id of the github/group module)."
  type        = string
}

variable "environments" {
  description = "GitHub environments to create; protected ones require the owner team's approval."
  type = map(object({
    protected = bool
  }))
  default = {}
}

variable "required_approvals" {
  description = "Approvals a PR to the default branch needs (platform github.requiredApprovals)."
  type        = number

  validation {
    condition     = var.required_approvals >= 0 && var.required_approvals <= 10 && floor(var.required_approvals) == var.required_approvals
    error_message = "required_approvals must be an integer between 0 and 10."
  }
}

variable "archive_on_destroy" {
  description = "Archive instead of delete when the Component claim is removed (platform github.archiveOnDestroy)."
  type        = bool
}

variable "writer_app_id" {
  description = "Id of the writer GitHub App: the only bypass actor of the default-branch ruleset."
  type        = number
}

variable "default_branch" {
  description = "Default branch name; environments deploy only from it."
  type        = string
  default     = "main"
}
