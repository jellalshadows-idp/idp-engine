variable "github_repository" {
  description = "The Component's GitHub repository, <org>/<name>: only its CI may assume the role."
  type        = string

  validation {
    condition     = can(regex("^[A-Za-z0-9-]+/[a-z0-9-]+$", var.github_repository))
    error_message = "github_repository must be <org>/<name>, letters, digits and hyphens only, so the trust policy names exactly one repository."
  }
}

variable "environment" {
  description = "The GitHub environment whose CI jobs may assume the role (spec §4.4)."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9]{0,9}$", var.environment))
    error_message = "environment must match ^[a-z][a-z0-9]{0,9}$ (spec §4.6)."
  }
}

variable "repository_name" {
  description = "ECR repository name: naming.RepositoryName (spec §5.5)."
  type        = string
}

variable "role_name" {
  description = "CI role name: naming.RoleName (spec §5.5)."
  type        = string
}

variable "oidc_provider_arn" {
  description = "ARN of the account's GitHub Actions OIDC provider, which the environment's baseline creates: naming.OIDCProviderARN (spec §5.5)."
  type        = string

  validation {
    condition     = can(regex("^arn:aws:iam::[0-9]{12}:oidc-provider/token\\.actions\\.githubusercontent\\.com$", var.oidc_provider_arn))
    error_message = "oidc_provider_arn must be the GitHub Actions OIDC provider of an account in the aws partition."
  }
}
