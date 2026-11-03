package schemas

import (
	"bytes"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func compile(t *testing.T, kind string) *jsonschema.Schema {
	t.Helper()
	raw, err := FS.ReadFile(File(kind))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	url := "https://schemas.idp.invalid/" + File(kind)
	if err := c.AddResource(url, doc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile(url)
	if err != nil {
		t.Fatalf("%s does not compile: %v", kind, err)
	}
	return sch
}

func instance(t *testing.T, js string) any {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(js))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFile(t *testing.T) {
	if got := File("Component"); got != "component.json" {
		t.Errorf("File(Component) = %q", got)
	}
}

func TestSchemas(t *testing.T) {
	tests := []struct {
		kind  string
		name  string
		doc   string
		valid bool
	}{
		{"Platform", "minimal", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{}}}`, true},
		{"Platform", "full", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":5255579,"archiveOnDestroy":false,"requiredApprovals":2},"naming":{"requiredPrefix":"e2e-","reservedPrefixes":["spike-"]},"environments":{"dev":{"aws":{"accountId":"000000000001","region":"eu-west-1"}},"pro":{"protected":true}},"modules":{"allowedSources":["git::https://github.com/acme/"]}}`, true},
		{"Platform", "missing writerAppId", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme"},"environments":{"dev":{}}}`, false},
		{"Platform", "bad environment name", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"Production":{}}}`, false},
		{"Platform", "approvals out of range", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1,"requiredApprovals":11},"environments":{"dev":{}}}`, false},
		{"Platform", "aws account id of 11 digits", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"00000000001","region":"eu-west-1"}}}}`, false},
		{"Platform", "aws account id of 13 digits", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"0000000000001","region":"eu-west-1"}}}}`, false},
		{"Platform", "aws account id as a number", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":1,"region":"eu-west-1"}}}}`, false},
		{"Platform", "aws region with three parts", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"000000000001","region":"us-gov-west-1"}}}}`, true},
		{"Platform", "aws region in uppercase", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"000000000001","region":"EU-west-1"}}}}`, false},
		{"Platform", "aws region without its number", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"000000000001","region":"eu-west"}}}}`, false},
		{"Platform", "aws block without a region", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"000000000001"}}}}`, false},
		{"Platform", "aws emulator endpoint", `{"apiVersion":"idp/v1","kind":"Platform","github":{"org":"acme","writerAppId":1},"environments":{"dev":{"aws":{"accountId":"000000000001","region":"eu-west-1","endpoint":"http://localhost:4566"}}}}`, false},
		{"Group", "valid", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","members":[{"user":"alice","role":"maintainer"}]}`, true},
		{"Group", "bad role", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","members":[{"user":"alice","role":"owner"}]}`, false},
		{"Group", "no members", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","members":[]}`, false},
		{"Group", "unknown field", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","members":[{"user":"a","role":"member"}],"team":"x"}`, false},
		{"Group", "multi-line description", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","description":"one\ntwo","members":[{"user":"alice","role":"member"}]}`, false},
		{"Component", "multi-line description", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","description":"one\ntwo"}`, false},
		{"Component", "valid", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","environments":["dev","pro"],"github":{"topics":["java"]}}`, true},
		{"Component", "owner without prefix", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"platform"}`, false},
		{"Component", "aws registry", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","environments":["dev"],"aws":{"registry":true}}`, true},
		{"Component", "empty aws block", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":{}}`, true},
		{"Component", "registry false", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":{"registry":false}}`, true},
		{"Component", "registry as a string", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":{"registry":"true"}}`, false},
		{"Component", "unknown aws field", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":{"registry":true,"bucket":"logs"}}`, false},
		{"Component", "aws is not a mapping", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":true}`, false},
		{"Component", "features not supported yet", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","features":[]}`, false},
		{"Component", "duplicate environment", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","environments":["dev","dev"]}`, false},
		{"Component", "wrong kind", `{"apiVersion":"idp/v1","kind":"Group","name":"api","owner":"group:platform"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.kind+"/"+tt.name, func(t *testing.T) {
			err := compile(t, tt.kind).Validate(instance(t, tt.doc))
			if tt.valid && err != nil {
				t.Errorf("want valid, got %v", err)
			}
			if !tt.valid && err == nil {
				t.Error("want invalid, got valid")
			}
		})
	}
}
