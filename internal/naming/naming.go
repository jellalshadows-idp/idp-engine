// Package naming computes the deterministic AWS identities of a Component in an
// environment (spec §5.5, ADR-0006). The GitHub stack receives these values from
// here, never from AWS state, and the AWS modules must create exactly these
// names: they are a contract. Every function is pure, over names the schemas
// already constrain (lowercase; a Component name of at most 40 characters and an
// environment name of at most 10), so none can fail.
package naming

// OIDCProviderHost is the GitHub Actions OIDC issuer. Each environment's account
// has one IAM OIDC provider for it, created by the env's _baseline stack (spec §5.4).
const OIDCProviderHost = "token.actions.githubusercontent.com"

// RepositoryName is the ECR repository of a Component: its own name (spec §4.4).
func RepositoryName(component string) string { return component }

// RoleName is the IAM role that a Component's CI assumes in an environment to push
// to its repository (spec §4.4). It is at most 54 characters.
func RoleName(component, env string) string { return component + "-" + env + "-ci" }

// RoleARN is the ARN of RoleName in the environment's account (spec §5.5).
func RoleARN(accountID, component, env string) string {
	return "arn:aws:iam::" + accountID + ":role/" + RoleName(component, env)
}

// RegistryURL is the URL of a Component's ECR repository in an environment's
// account and region (spec §5.5). floci answers with a localhost:4566 host instead
// (ADR-0013), so emulator output is compared by repository name, never by URL.
func RegistryURL(accountID, region, component string) string {
	return accountID + ".dkr.ecr." + region + ".amazonaws.com/" + RepositoryName(component)
}

// OIDCProviderARN is the ARN of the GitHub OIDC provider in an account (spec §5.5).
func OIDCProviderARN(accountID string) string {
	return "arn:aws:iam::" + accountID + ":oidc-provider/" + OIDCProviderHost
}
