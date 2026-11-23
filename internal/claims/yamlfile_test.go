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
		{name: "an unclosed flow sequence points at its start", data: "a: 1\nb: [1, 2\n", wantMsg: "invalid YAML: line 2: did not find expected ',' or ']'", wantLine: 2},
		{name: "a flow sequence left open at the end points at the last line", data: "a: 1\nb: [\n", wantMsg: "invalid YAML: line 2: ", wantLine: 2},
		{name: "a flow sequence left open at the end of a CRLF file points at the last line", data: "a: 1\r\nb: [1,\r\n", wantMsg: "invalid YAML: line 2: ", wantLine: 2},
		{name: "unclosed flow mappings on a one-line file point at it", data: "{{{\n", wantMsg: "invalid YAML: line 1: ", wantLine: 1},
		{name: "an unclosed quoted scalar points at its start", data: "x: 1\na: \"x\n", wantMsg: "invalid YAML: line 2: found unexpected end of stream", wantLine: 2},
		{name: "a syntax error on the first line points at it", data: "a: b: c\n", wantMsg: "invalid YAML: line 1: mapping values are not allowed in this context", wantLine: 1},
		{name: "a misplaced mapping value points at its line", data: "a: 1\nb: c: d\n", wantMsg: "invalid YAML: line 2: mapping values are not allowed in this context", wantLine: 2},
		{name: "a tab points at its line", data: "a: 1\n\tb: 2\n", wantMsg: "invalid YAML: line 2: found a tab character that violates indentation", wantLine: 2},
		{name: "an unknown anchor points at its use", data: "a: 1\nb: *y\n", wantMsg: "invalid YAML: line 2: unknown anchor 'y' referenced", wantLine: 2},
		{name: "a bad escape points at its line", data: "a: 1\nb: \"\\q\"\n", wantMsg: "invalid YAML: line 2: found unknown escape character", wantLine: 2},
		{name: "non-string key", data: "1: a\n", wantMsg: "mapping keys must be strings", wantLine: 1},
		{name: "non-finite number", data: "a: .inf\n", wantMsg: "numbers must be finite", wantLine: 1},
		{name: "duplicate key", data: "a: 1\nb: 2\na: 3\n", wantMsg: `duplicate key "a" (first defined at line 1)`, wantLine: 3},
		{name: "date-like value", data: "a: 2024-01-01\n"},
		{name: "merge key", data: "base: &b {x: 1}\nuse:\n  <<: *b\n"},
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

func TestParseFileSelfReferentialAlias(t *testing.T) {
	doc, ds := parseFile("f.yaml", []byte("a: &a [*a]\n"))
	if len(ds) != 0 || doc == nil {
		t.Fatalf("want a document, got %v", ds)
	}
	if got := doc.line([]string{"a"}); got != 1 {
		t.Errorf("line = %d, want 1", got)
	}
}

func TestDocumentLineThroughAlias(t *testing.T) {
	doc, ds := parseFile("f.yaml", []byte("base: &base\n  role: member\nuse: *base\n"))
	if len(ds) != 0 || doc == nil {
		t.Fatalf("want a document, got %v", ds)
	}
	if got := doc.line([]string{"use", "role"}); got != 3 {
		t.Errorf("line(use/role) = %d, want 3", got)
	}
	if got := doc.line([]string{"base", "role"}); got != 2 {
		t.Errorf("line(base/role) = %d, want 2", got)
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
