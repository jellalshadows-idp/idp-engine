package claims

import (
	"strings"
	"testing"
)

func TestParseFile(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		wantMsg  string // empty means valid
		wantLine int    // -1 means "any line > 0"
	}{
		{name: "valid mapping", data: "a: 1\n"},
		{name: "empty file", data: "", wantMsg: "file is empty", wantLine: 0},
		{name: "two documents", data: "a: 1\n---\nb: 2\n", wantMsg: "only one YAML document per file is allowed", wantLine: -1},
		{name: "not a mapping", data: "- a\n- b\n", wantMsg: "the document must be a mapping", wantLine: 1},
		{name: "invalid YAML reports a line", data: "a: 1\nb: [unclosed\n", wantMsg: "invalid YAML", wantLine: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, ds := parseFile("f.yaml", []byte(tt.data))
			if tt.wantMsg == "" {
				if len(ds) != 0 || doc == nil {
					t.Fatalf("want a document, got %v", ds)
				}
				return
			}
			if doc != nil || len(ds) != 1 || !strings.Contains(ds[0].Message, tt.wantMsg) {
				t.Fatalf("diagnostics = %v, want one containing %q", ds, tt.wantMsg)
			}
			if tt.wantLine == -1 && ds[0].Line <= 0 {
				t.Errorf("line = %d, want > 0", ds[0].Line)
			}
			if tt.wantLine >= 0 && ds[0].Line != tt.wantLine {
				t.Errorf("line = %d, want %d", ds[0].Line, tt.wantLine)
			}
		})
	}
}

func TestDocumentLine(t *testing.T) {
	src := "apiVersion: idp/v1\nkind: Group\nname: platform\nmembers:\n  - user: alice\n    role: owner\na/b: 1\n"
	doc, ds := parseFile("g.yaml", []byte(src))
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	tests := []struct {
		tokens []string
		want   int
	}{
		{nil, 1},
		{[]string{"name"}, 3},
		{[]string{"members"}, 4},
		{[]string{"members", "0"}, 5},
		{[]string{"members", "0", "role"}, 6},
		{[]string{"members", "0", "missing"}, 5},
		{[]string{"a/b"}, 7},
	}
	for _, tt := range tests {
		if got := doc.line(tt.tokens); got != tt.want {
			t.Errorf("line(%v) = %d, want %d", tt.tokens, got, tt.want)
		}
	}
	if got := pointer([]string{"a/b", "c~d"}); got != "/a~1b/c~0d" {
		t.Errorf("pointer = %q", got)
	}
}
