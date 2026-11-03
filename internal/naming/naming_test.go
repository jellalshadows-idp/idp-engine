package naming

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/schemas"
)

// AWS's own name rules, from the API references of IAM CreateRole and ECR
// CreateRepository (read 2026-11-03).
var (
	iamRoleName       = regexp.MustCompile(`^[\w+=,.@-]{1,64}$`)
	ecrRepositoryName = regexp.MustCompile(`^[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*$`)
)

func TestIdentities(t *testing.T) {
	tests := []struct {
		name, got, want string
	}{
		{"role ARN, spec §5.3", RoleARN("000000000003", "api", "pro"), "arn:aws:iam::000000000003:role/api-pro-ci"},
		{"registry URL, spec §5.3", RegistryURL("000000000003", "eu-west-1", "api"), "000000000003.dkr.ecr.eu-west-1.amazonaws.com/api"},
		{"role ARN, ADR-0013", RoleARN("000000000001", "api", "dev"), "arn:aws:iam::000000000001:role/api-dev-ci"},
		{"OIDC provider ARN, spec §5.5", OIDCProviderARN("000000000003"), "arn:aws:iam::000000000003:oidc-provider/token.actions.githubusercontent.com"},
		{"repository name, spec §4.4", RepositoryName("api"), "api"},
		{"role name, spec §4.4", RoleName("api", "staging"), "api-staging-ci"},
		{"harness component", RoleName("e2e-r123-app", "dev"), "e2e-r123-app-dev-ci"},
		{"registry URL keeps the region", RegistryURL("000000000002", "ap-southeast-2", "e2e-r123-app"), "000000000002.dkr.ecr.ap-southeast-2.amazonaws.com/e2e-r123-app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

// schemaPattern returns the regular expression at path inside the embedded schema
// of kind, so these tests follow the schemas instead of copying their rules.
func schemaPattern(t *testing.T, kind string, path ...string) *regexp.Regexp {
	t.Helper()
	raw, err := schemas.FS.ReadFile(schemas.File(kind))
	if err != nil {
		t.Fatal(err)
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}
	for _, key := range path {
		obj, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("%s: no object holds %q", schemas.File(kind), key)
		}
		node = obj[key]
	}
	pattern, ok := node.(string)
	if !ok {
		t.Fatalf("%s: %s is not a pattern", schemas.File(kind), strings.Join(path, "."))
	}
	return regexp.MustCompile(pattern)
}

// nameRules returns the schemas' rules for a Component name and an environment
// name, and checks that a Component's environments follow the Platform's rule.
func nameRules(t *testing.T) (component, env *regexp.Regexp) {
	t.Helper()
	component = schemaPattern(t, "Component", "properties", "name", "pattern")
	env = schemaPattern(t, "Platform", "properties", "environments", "propertyNames", "pattern")
	if claimEnv := schemaPattern(t, "Component", "properties", "environments", "items", "pattern"); claimEnv.String() != env.String() {
		t.Fatalf("a Component's environments (%s) and the Platform's (%s) follow different rules", claimEnv, env)
	}
	return component, env
}

// TestNamesFitAWSLimits proves spec §5.5's bound: the longest names the schemas
// allow, a 40-character Component (§4.2) and a 10-character environment (§4.6),
// give a 54-character role name, within IAM's 64, and a valid ECR repository name.
func TestNamesFitAWSLimits(t *testing.T) {
	componentRule, envRule := nameRules(t)
	longestComponent := "a" + strings.Repeat("b-", 19) + "c"
	longestEnv := "e" + strings.Repeat("9", 9)
	for _, n := range []struct {
		what, value string
		rule        *regexp.Regexp
	}{
		{"Component name", longestComponent, componentRule},
		{"environment name", longestEnv, envRule},
	} {
		if !n.rule.MatchString(n.value) {
			t.Fatalf("the schema rejects the %s %q; this test assumes it is the longest allowed", n.what, n.value)
		}
		if n.rule.MatchString(n.value + "x") {
			t.Fatalf("the schema allows a %s longer than %d characters: recheck the AWS limits (spec §5.5) before widening it", n.what, len(n.value))
		}
	}
	if role := RoleName(longestComponent, longestEnv); len(role) != 54 || !iamRoleName.MatchString(role) {
		t.Errorf("RoleName = %q (%d characters), want 54 characters that IAM accepts", role, len(role))
	}
	if repo := RepositoryName(longestComponent); len(repo) > 256 || !ecrRepositoryName.MatchString(repo) {
		t.Errorf("RepositoryName = %q, want a name ECR accepts", repo)
	}
}

// TestSchemaNameRulesKeepAWSNamesValid checks names at the edges of the schema
// rules: every name they accept gives AWS-valid names, and they reject the rest.
// ECR allows only lowercase, and IAM compares role names without regard to case,
// so an uppercase name must never reach naming.
func TestSchemaNameRulesKeepAWSNamesValid(t *testing.T) {
	componentRule, envRule := nameRules(t)
	tests := []struct {
		component string
		allowed   bool
	}{
		{"ab", true},
		{"a--b", true},
		{"a1-2b", true},
		{"e2e-r123-app", true},
		{"a", false},
		{"Api", false},
		{"API", false},
		{"a_b", false},
		{"a.b", false},
		{"-ab", false},
		{"ab-", false},
		{"1ab", false},
	}
	for _, tt := range tests {
		t.Run(tt.component, func(t *testing.T) {
			if got := componentRule.MatchString(tt.component); got != tt.allowed {
				t.Fatalf("the schema allows %q: %v, want %v", tt.component, got, tt.allowed)
			}
			if !tt.allowed {
				return
			}
			if repo := RepositoryName(tt.component); len(repo) < 2 || len(repo) > 256 || !ecrRepositoryName.MatchString(repo) {
				t.Errorf("RepositoryName(%q) = %q, which ECR rejects", tt.component, repo)
			}
			if role := RoleName(tt.component, "dev"); !iamRoleName.MatchString(role) {
				t.Errorf("RoleName(%q, \"dev\") = %q, which IAM rejects", tt.component, role)
			}
		})
	}
	for _, env := range []string{"Dev", "DEV", "pro-1", "1dev", "dev_1"} {
		if envRule.MatchString(env) {
			t.Errorf("the environment rule allows %q", env)
		}
	}
}
