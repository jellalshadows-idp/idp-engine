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
		{name: "aws is not supported yet", kind: "Component", src: "apiVersion: idp/v1\nkind: Component\nname: api\nowner: group:platform\naws:\n  registry: true\n", wantLine: 5, wantContains: "aws"},
		{name: "a group filed as a component", kind: "Component", src: group, wantLine: 2, wantContains: "/kind"},
		{name: "non-string keys", kind: "Group", src: "1: a\n", wantLine: 1, wantContains: "mapping keys must be strings"},
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
