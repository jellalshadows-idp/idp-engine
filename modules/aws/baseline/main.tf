# The GitHub Actions OIDC provider of an environment's AWS account (spec §5.4).
# IAM identifies a provider by its URL, so an account holds one: two
# environments in one account would create it twice, which claims validation
# rejects. Its ARN is naming.OIDCProviderARN(accountId), and the CI roles of
# modules/aws/component trust it by that computed ARN, never through this
# stack's state (spec §5.5).
resource "aws_iam_openid_connect_provider" "this" {
  url = "https://token.actions.githubusercontent.com"

  # The audience that aws-actions/configure-aws-credentials requests.
  client_id_list = ["sts.amazonaws.com"]

  # No thumbprint_list: IAM validates GitHub's issuer against its own trusted
  # root CAs and ignores thumbprints for it, and the attribute is optional.
}
