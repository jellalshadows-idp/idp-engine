mock_provider "aws" {}

run "the_provider_is_github_actions_issuer" {
  command = plan

  assert {
    condition     = aws_iam_openid_connect_provider.this.url == "https://token.actions.githubusercontent.com"
    error_message = "the provider must be GitHub Actions' issuer, naming.OIDCProviderHost, so its ARN is naming.OIDCProviderARN (spec §5.5)"
  }
}

run "the_provider_accepts_only_the_sts_audience" {
  command = plan

  assert {
    condition     = tolist(aws_iam_openid_connect_provider.this.client_id_list) == tolist(["sts.amazonaws.com"])
    error_message = "the only client id must be sts.amazonaws.com, the aud of every CI role's trust policy (spec §4.4)"
  }
}
