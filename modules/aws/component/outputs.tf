output "repository_arn" {
  description = "ARN of the ECR repository."
  value       = aws_ecr_repository.this.arn
}

output "role_arn" {
  description = "ARN of the CI role: naming.RoleARN."
  value       = aws_iam_role.ci.arn
}
