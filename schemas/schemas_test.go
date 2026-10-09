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
		{"Group", "valid", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","members":[{"user":"alice","role":"maintainer"}]}`, true},
		{"Group", "bad role", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","members":[{"user":"alice","role":"owner"}]}`, false},
		{"Group", "no members", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","members":[]}`, false},
		{"Group", "unknown field", `{"apiVersion":"idp/v1","kind":"Group","name":"platform","members":[{"user":"a","role":"member"}],"team":"x"}`, false},
		{"Component", "valid", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","environments":["dev","pro"],"github":{"topics":["java"]}}`, true},
		{"Component", "owner without prefix", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"platform"}`, false},
		{"Component", "aws not supported yet", `{"apiVersion":"idp/v1","kind":"Component","name":"api","owner":"group:platform","aws":{"registry":true}}`, false},
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
