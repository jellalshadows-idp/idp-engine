package workflowspec

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// parseWorkflow decodes inline YAML the same way loadWorkflow decodes a file.
func parseWorkflow(t *testing.T, src string) *workflow {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	w := &workflow{path: "inline.yaml", root: &doc}
	if err := doc.Decode(w); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return w
}

func TestUsesRefsKeepTheTrailingComment(t *testing.T) {
	sha := strings.Repeat("b", 40)
	src := "jobs:\n  a:\n    steps:\n" +
		"      - uses: a/b@" + sha + " # v1.2.3\n" +
		"      - uses: c/d@" + sha + " # v3.2.0 # zizmor: ignore[x] y\n" +
		"      - uses: e/f@" + sha + "\n"
	w := parseWorkflow(t, src)
	got := usesRefs(w.root)
	want := []string{"# v1.2.3", "# v3.2.0 # zizmor: ignore[x] y", ""}
	if len(got) != len(want) {
		t.Fatalf("usesRefs found %d entries, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].comment != want[i] {
			t.Errorf("entry %d comment = %q, want %q", i, got[i].comment, want[i])
		}
	}
	if got[0].value != "a/b@"+sha {
		t.Errorf("entry 0 value = %q", got[0].value)
	}
}

func TestInvokes(t *testing.T) {
	tests := []struct {
		name, run, sub string
		want           bool
	}{
		{"plain call", "idp comment --pr 1", "comment", true},
		{"bare subcommand", "idp comment", "comment", true},
		{"after then", "if x; then idp comment; fi", "comment", true},
		{"after a semicolon", "x; idp issue open", "issue", true},
		{"double space", "idp  comment", "comment", true},
		{"indented, in a script", "set -e\n  idp issue open", "issue", true},
		{"any call", "idp plan", "", true},
		{"other subcommand", "idp plan", "comment", false},
		{"echo mentions it", `echo "idp comment failed"`, "comment", false},
		{"printf mentions it", `printf 'idp issue\n'`, "issue", false},
		{"comment line", "# idp comment later", "comment", false},
		{"empty", "", "", false},
		{"idp as a path", "mkdir -p $RUNNER_TEMP/idp", "", false},
		{"idp alone, any call", "idp", "", true},
		{"idp alone, wanted sub", "idp", "comment", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := invokes(tt.run, tt.sub); got != tt.want {
				t.Errorf("invokes(%q, %q) = %v, want %v", tt.run, tt.sub, got, tt.want)
			}
		})
	}
}

func TestStrmapStrlistPerms(t *testing.T) {
	w := parseWorkflow(t, `
env:
  A: false
  B: 7
  C: x
permissions: {}
jobs:
  one:
    needs: a
    permissions: read-all
  two:
    needs: [a, b]
    permissions:
      contents: read
  three: {}
`)
	if w.Env["A"] != "false" || w.Env["B"] != "7" || w.Env["C"] != "x" {
		t.Errorf("env = %v, want literal strings", w.Env)
	}
	if !w.Permissions.present || w.Permissions.scalar != "" || len(w.Permissions.levels) != 0 {
		t.Errorf("permissions: {} = %+v, want present and empty", w.Permissions)
	}
	if got := w.Jobs["one"]; len(got.Needs) != 1 || got.Needs[0] != "a" || got.Permissions.scalar != "read-all" {
		t.Errorf("job one = %+v, want scalar needs and read-all", got)
	}
	if got := w.Jobs["two"]; len(got.Needs) != 2 || got.Permissions.levels["contents"] != "read" {
		t.Errorf("job two = %+v, want list needs and contents: read", got)
	}
	if w.Jobs["three"].Permissions.present {
		t.Error("absent permissions must not be present")
	}
}

const writerKeyOK = `
on:
  workflow_call:
    secrets:
      IDP_WRITER_PRIVATE_KEY:
        required: false
permissions: {}
jobs:
  write:
    environment: idp-write
    permissions: {}
    steps:
      - uses: x/y@v1
        with:
          private-key: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
  approve:
    environment: idp-approval
    permissions: {}
    steps:
      - run: echo ok
`

func TestWriterKeyErrors(t *testing.T) {
	tests := []struct {
		name, src string
		bad       bool
	}{
		{"key only in the write job", writerKeyOK, false},
		{"key in a step outside the write environment", `
jobs:
  j:
    steps:
      - env:
          K: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
        run: echo
`, true},
		{"key in job-level env", `
jobs:
  j:
    env:
      K: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
    steps:
      - run: echo
`, true},
		{"key in job outputs", `
jobs:
  j:
    outputs:
      K: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
    steps:
      - run: echo
`, true},
		{"key passed to a caller job", `
jobs:
  j:
    uses: o/r/.github/workflows/a.yaml@v1
    secrets:
      IDP_WRITER_PRIVATE_KEY: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
`, true},
		{"key passed to the pr workflow", `
jobs:
  pr:
    uses: jellalshadows-idp/idp-engine/.github/workflows/pr.yaml@0123456789abcdef0123456789abcdef01234567
    secrets:
      IDP_WRITER_PRIVATE_KEY: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
`, true},
		{"key passed to the drift workflow", `
jobs:
  drift:
    uses: jellalshadows-idp/idp-engine/.github/workflows/drift.yaml@0123456789abcdef0123456789abcdef01234567
    secrets:
      IDP_WRITER_PRIVATE_KEY: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
`, true},
		{"key passed by name to the reconcile workflow", `
jobs:
  reconcile:
    uses: jellalshadows-idp/idp-engine/.github/workflows/reconcile.yaml@0123456789abcdef0123456789abcdef01234567
    secrets:
      IDP_READER_PRIVATE_KEY: ${{ secrets.IDP_READER_PRIVATE_KEY }}
      IDP_WRITER_PRIVATE_KEY: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
`, false},
		{"key passed by name to the recover workflow", `
jobs:
  recover:
    uses: jellalshadows-idp/idp-engine/.github/workflows/recover.yaml@0123456789abcdef0123456789abcdef01234567
    secrets:
      IDP_WRITER_PRIVATE_KEY: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
`, false},
		{"another secret passed as the key to the reconcile workflow", `
jobs:
  reconcile:
    uses: jellalshadows-idp/idp-engine/.github/workflows/reconcile.yaml@0123456789abcdef0123456789abcdef01234567
    secrets:
      IDP_WRITER_PRIVATE_KEY: ${{ secrets.OTHER }}
`, true},
		{"key passed under a second name to the reconcile workflow", `
jobs:
  reconcile:
    uses: jellalshadows-idp/idp-engine/.github/workflows/reconcile.yaml@0123456789abcdef0123456789abcdef01234567
    secrets:
      IDP_WRITER_PRIVATE_KEY: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
      OTHER: ${{ secrets.IDP_WRITER_PRIVATE_KEY }}
`, true},
		{"secrets inherit to the reconcile workflow", `
jobs:
  reconcile:
    uses: jellalshadows-idp/idp-engine/.github/workflows/reconcile.yaml@0123456789abcdef0123456789abcdef01234567
    secrets: inherit
`, true},
		{"secret in workflow-level env", `
env:
  K: ${{ secrets.ANYTHING }}
jobs:
  j:
    steps:
      - run: echo
`, true},
		{"secrets inherit", `
jobs:
  j:
    uses: o/r/.github/workflows/a.yaml@v1
    secrets: inherit
`, true},
		{"secrets by index", `
jobs:
  j:
    steps:
      - env:
          K: ${{ secrets['IDP_WRITER_PRIVATE_KEY'] }}
        run: echo
`, true},
		{"toJSON of all secrets", `
jobs:
  j:
    steps:
      - env:
          K: ${{ toJSON(secrets) }}
        run: echo
`, true},
		{"approval job calls a workflow", `
jobs:
  a:
    environment: idp-approval
    uses: o/r/.github/workflows/a.yaml@v1
`, true},
		{"approval job runs an action", `
jobs:
  a:
    environment: idp-approval
    steps:
      - uses: x/y@v1
`, true},
		{"approval job reads a secret in job env", `
jobs:
  a:
    environment: idp-approval
    env:
      K: ${{ secrets.OTHER }}
    steps:
      - run: echo
`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := writerKeyErrors(parseWorkflow(t, tt.src))
			if (len(errs) > 0) != tt.bad {
				t.Errorf("writerKeyErrors = %v, want bad=%v", errs, tt.bad)
			}
		})
	}
}

func TestBotWriteErrors(t *testing.T) {
	tests := []struct {
		name string
		s    step
		errs int
	}{
		{"correct step", step{Run: "idp comment --pr 1", Env: strmap{"GITHUB_TOKEN": "${{ github.token }}"}}, 0},
		{"missing token", step{Run: "idp comment --pr 1"}, 1},
		{"wrong token", step{Run: "idp issue open", Env: strmap{"GITHUB_TOKEN": "${{ secrets.X }}"}}, 1},
		{"GH_TOKEN set", step{Run: "idp issue open", Env: strmap{"GITHUB_TOKEN": "${{ github.token }}", "GH_TOKEN": "x"}}, 1},
		{"not a bot write", step{Run: "idp plan"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := botWriteErrors("inline", tt.s); len(got) != tt.errs {
				t.Errorf("botWriteErrors = %v, want %d error(s)", got, tt.errs)
			}
		})
	}
}
