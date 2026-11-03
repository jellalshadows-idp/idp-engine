package claims

import (
	"strings"
	"testing"
)

func mustDoc(t *testing.T, src string) *document {
	t.Helper()
	doc, ds := parseFile("claims/x.yaml", []byte(src))
	if len(ds) > 0 {
		t.Fatalf("fixture does not parse: %v", ds)
	}
	return doc
}

func TestValidate(t *testing.T) {
	v, err := newValidator()
	if err != nil {
		t.Fatal(err)
	}
	group := "apiVersion: idp/v1\nkind: Group\nname: platform\nmembers:\n  - user: alice\n    role: maintainer\n"
	tests := []struct {
		name         string
		kind         string
		src          string
		wantLine     int    // 0 means "valid"
		wantContains string // substring of the diagnostic message
	}{
		{name: "valid group", kind: "Group", src: group},
		{name: "bad role points at the role line", kind: "Group", src: strings.Replace(group, "maintainer", "owner", 1), wantLine: 6, wantContains: "/members/0/role"},
		{name: "missing members points at the document", kind: "Group", src: "apiVersion: idp/v1\nkind: Group\nname: platform\n", wantLine: 1, wantContains: "members"},
		{name: "unknown field points at its key", kind: "Group", src: group + "team: x\n", wantLine: 7, wantContains: "team"},
		{name: "bad name", kind: "Group", src: strings.Replace(group, "name: platform", "name: Platform", 1), wantLine: 3, wantContains: "/name"},
		{name: "valid component", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\nenvironments: [dev, pro]\n"},
		{name: "owner without group prefix", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: platform\n", wantLine: 4, wantContains: "/owner"},
		{name: "valid component with an aws registry", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\nenvironments: [dev]\naws:\n  registry: true\n"},
		{name: "unknown aws field points at its key", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\naws:\n  registry: true\n  bucket: logs\n", wantLine: 7, wantContains: "bucket"},
		{name: "registry that is not a boolean points at it", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\naws:\n  registry: \"yes\"\n", wantLine: 6, wantContains: "/aws/registry"},
		{name: "features are not supported yet", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\nfeatures: []\n", wantLine: 5, wantContains: "features"},
		{name: "a group filed as a component", kind: "Component", src: group, wantLine: 2, wantContains: "/kind"},
		{name: "unquoted aws account id points at it", kind: "Platform", src: "apiVersion: idp/v1\nkind: Platform\ngithub:\n  org: acme\n  writerAppId: 1\nenvironments:\n  dev:\n    aws:\n      accountId: 000000000001\n      region: eu-west-1\n", wantLine: 9, wantContains: "/environments/dev/aws/accountId"},
		{name: "self-referential anchor is a diagnostic, not a hang", kind: "Group", src: "a: &a [*a]\n", wantLine: 1, wantContains: "cannot read document"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := v.validate(tt.kind, mustDoc(t, tt.src))
			if tt.wantLine == 0 {
				if len(ds) != 0 {
					t.Fatalf("want valid, got %v", ds)
				}
				return
			}
			for _, d := range ds {
				if d.Line == tt.wantLine && strings.Contains(d.Message, tt.wantContains) {
					return
				}
			}
			t.Fatalf("diagnostics = %v, want one at line %d containing %q", ds, tt.wantLine, tt.wantContains)
		})
	}
}

func TestJSONInstanceKeepsTimestampsAsWritten(t *testing.T) {
	doc := mustDoc(t, "a: 2024-01-01\nb: 2024-01-01T10:00:00Z\n")
	inst, err := jsonInstance(doc.root)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := inst.(map[string]any)
	if !ok {
		t.Fatalf("instance = %T, want map", inst)
	}
	if m["a"] != "2024-01-01" || m["b"] != "2024-01-01T10:00:00Z" {
		t.Errorf("instance = %v, want the dates exactly as written", m)
	}
}
