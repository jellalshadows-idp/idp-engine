// Package workflowspec holds the contract tests of the engine's GitHub Actions
// files: the reusable workflows, the setup-idp composite action and the claims
// repo templates. The tests read the YAML and pin the rules that the spec and
// ADRs 0018 and 0021–0024 set, so a change that breaks one fails go test.
package workflowspec
