output "oidc_provider_arn" {
  description = "ARN of the account's GitHub Actions OIDC provider: naming.OIDCProviderARN."
  value       = aws_iam_openid_connect_provider.this.arn
}
