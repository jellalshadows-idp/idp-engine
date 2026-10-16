package workflowspec

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// repoRoot is the repository root, seen from this package's directory.
const repoRoot = "../.."

// Paths and values the rules share (ADR-0021).
const (
	setupIDPPath = ".github/actions/setup-idp/action.yaml"
	setupIDP     = "./.idp/engine/.github/actions/setup-idp"
	engineDir    = ".idp/engine"
	workflowRepo = "${{ job.workflow_repository }}"
	workflowSHA  = "${{ job.workflow_sha }}"
)

// strmap decodes a mapping of scalars, keeping each value's literal text
// whatever its YAML type (false, 7 and "x" all become strings).
type strmap map[string]string

func (m *strmap) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: want a mapping", n.Line)
	}
	out := strmap{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if v.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: %s must be a scalar", v.Line, k.Value)
		}
		out[k.Value] = v.Value
	}
	*m = out
	return nil
}

// strlist decodes a scalar or a sequence of scalars (needs: a, or needs: [a, b]).
type strlist []string

func (l *strlist) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*l = strlist{n.Value}
		return nil
	}
	var s []string
	if err := n.Decode(&s); err != nil {
		return err
	}
	*l = s
	return nil
}

// perms is a permissions block; present is false when the key is absent.
type perms struct {
	present bool
	scalar  string            // read-all or write-all, which the rules forbid
	levels  map[string]string // scope -> read | write | none
}

func (p *perms) UnmarshalYAML(n *yaml.Node) error {
	p.present = true
	if n.Kind == yaml.ScalarNode {
		p.scalar = n.Value
		return nil
	}
	var m strmap
	if err := n.Decode(&m); err != nil {
		return err
	}
	p.levels = map[string]string(m)
	return nil
}

type workflow struct {
	path        string
	root        *yaml.Node
	On          yaml.Node       `yaml:"on"`
	Env         strmap          `yaml:"env"`
	Permissions perms           `yaml:"permissions"`
	Concurrency *concurrency    `yaml:"concurrency"`
	Jobs        map[string]*job `yaml:"jobs"`
}

type concurrency struct {
	Group            string `yaml:"group"`
	CancelInProgress bool   `yaml:"cancel-in-progress"`
	Queue            string `yaml:"queue"`
}

type job struct {
	Name           string    `yaml:"name"`
	Needs          strlist   `yaml:"needs"`
	If             string    `yaml:"if"`
	Environment    string    `yaml:"environment"`
	TimeoutMinutes int       `yaml:"timeout-minutes"`
	Env            strmap    `yaml:"env"`
	Permissions    perms     `yaml:"permissions"`
	Outputs        strmap    `yaml:"outputs"`
	Uses           string    `yaml:"uses"`
	With           strmap    `yaml:"with"`
	Secrets        yaml.Node `yaml:"secrets"`
	Steps          []step    `yaml:"steps"`
}

type step struct {
	ID               string `yaml:"id"`
	Name             string `yaml:"name"`
	If               string `yaml:"if"`
	Uses             string `yaml:"uses"`
	Run              string `yaml:"run"`
	Shell            string `yaml:"shell"`
	WorkingDirectory string `yaml:"working-directory"`
	With             strmap `yaml:"with"`
	Env              strmap `yaml:"env"`
}

// label names a step in failure messages.
func (s step) label() string {
	for _, l := range []string{s.ID, s.Name, s.Uses} {
		if l != "" {
			return l
		}
	}
	first, _, _ := strings.Cut(s.Run, "\n")
	return first
}

type action struct {
	path   string
	root   *yaml.Node
	Inputs map[string]struct {
		Required bool `yaml:"required"`
	} `yaml:"inputs"`
	Runs struct {
		Using string `yaml:"using"`
		Steps []step `yaml:"steps"`
	} `yaml:"runs"`
}

// callSpec is the workflow_call interface of a reusable workflow.
type callSpec struct {
	Inputs map[string]struct {
		Required bool   `yaml:"required"`
		Type     string `yaml:"type"`
	} `yaml:"inputs"`
	Secrets map[string]struct {
		Required bool `yaml:"required"`
	} `yaml:"secrets"`
}

// readYAML parses a repo-relative file into out and returns its document node.
func readYAML(t *testing.T, rel string, out any) *yaml.Node {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	if err := doc.Decode(out); err != nil {
		t.Fatalf("decode %s: %v", rel, err)
	}
	return &doc
}

func loadWorkflow(t *testing.T, rel string) *workflow {
	t.Helper()
	w := &workflow{path: rel}
	w.root = readYAML(t, rel, w)
	return w
}

func loadAction(t *testing.T, rel string) *action {
	t.Helper()
	a := &action{path: rel}
	a.root = readYAML(t, rel, a)
	return a
}

// glob lists the repo-relative slash paths matching pattern, sorted.
func glob(t *testing.T, pattern string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(repoRoot, filepath.FromSlash(pattern)))
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		rel, err := filepath.Rel(repoRoot, m)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	slices.Sort(out)
	return out
}

// workflowFiles is every workflow the common rules cover: the engine's own and
// the claims repo templates.
func workflowFiles(t *testing.T) []string {
	t.Helper()
	files := append(glob(t, ".github/workflows/*.yaml"), glob(t, "examples/claims-repo/.github/workflows/*.yaml")...)
	if len(files) == 0 {
		t.Fatal("no workflow files found; is repoRoot right?")
	}
	return files
}

// reusableNames are the engine workflows claims repos call (ADR-0021);
// internalWorkflows run only in idp-engine.
var (
	reusableNames     = []string{"drift.yaml", "pr.yaml", "reconcile.yaml", "recover.yaml"}
	internalWorkflows = []string{"ci.yaml", "e2e.yaml"}
)

func reusableWorkflows(t *testing.T) []string {
	t.Helper()
	out := make([]string, 0, len(reusableNames))
	for _, n := range reusableNames {
		out = append(out, ".github/workflows/"+n)
	}
	return out
}

// anchors returns the line of every YAML anchor or alias under n.
func anchors(n *yaml.Node) []int {
	var lines []int
	if n.Anchor != "" || n.Kind == yaml.AliasNode {
		lines = append(lines, n.Line)
	}
	for _, c := range n.Content {
		lines = append(lines, anchors(c)...)
	}
	return lines
}

func actionFiles(t *testing.T) []string {
	t.Helper()
	return glob(t, ".github/actions/*/action.yaml")
}

// located is a step with where it lives, for failure messages.
type located struct {
	where string
	step  step
}

// everyStep returns the steps of every workflow job and every composite action.
func everyStep(t *testing.T) []located {
	t.Helper()
	var out []located
	for _, f := range workflowFiles(t) {
		w := loadWorkflow(t, f)
		for _, id := range slices.Sorted(maps.Keys(w.Jobs)) {
			for _, s := range w.Jobs[id].Steps {
				out = append(out, located{where: f + " job " + id, step: s})
			}
		}
	}
	for _, f := range actionFiles(t) {
		for _, s := range loadAction(t, f).Runs.Steps {
			out = append(out, located{where: f, step: s})
		}
	}
	return out
}

// usesRef is one uses: value with its trailing comment.
type usesRef struct {
	value, comment string
	line           int
}

// usesRefs walks a document and returns every uses: entry.
func usesRefs(n *yaml.Node) []usesRef {
	var out []usesRef
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Value == "uses" && v.Kind == yaml.ScalarNode {
				c := v.LineComment
				if c == "" {
					c = k.LineComment
				}
				out = append(out, usesRef{value: v.Value, comment: c, line: v.Line})
			}
		}
	}
	for _, c := range n.Content {
		out = append(out, usesRefs(c)...)
	}
	return out
}

// mapValue returns the value node of key in a mapping node, or nil.
func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// jobNode returns the mapping node of jobs.<id>, or nil when there is none.
func jobNode(w *workflow, id string) *yaml.Node {
	root := w.root
	if root != nil && root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	return mapValue(mapValue(root, "jobs"), id)
}

// stepNode returns the i-th step mapping of a job node, or nil.
func stepNode(job *yaml.Node, i int) *yaml.Node {
	steps := mapValue(job, "steps")
	if steps == nil || steps.Kind != yaml.SequenceNode || i >= len(steps.Content) {
		return nil
	}
	return steps.Content[i]
}

// scalars returns every scalar value under n; mapping keys are excluded.
func scalars(n *yaml.Node) []string {
	return scalarsExcept(n, nil)
}

// scalarsExcept is scalars without the value of the skip node.
func scalarsExcept(n, skip *yaml.Node) []string {
	if n == nil || n == skip {
		return nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		return []string{n.Value}
	case yaml.MappingNode:
		var out []string
		for i := 1; i < len(n.Content); i += 2 {
			out = append(out, scalarsExcept(n.Content[i], skip)...)
		}
		return out
	}
	var out []string
	for _, c := range n.Content {
		out = append(out, scalarsExcept(c, skip)...)
	}
	return out
}

// trigger returns the node of one on: event and whether it is present. A bare
// "workflow_dispatch:" is present with a null node.
func (w *workflow) trigger(name string) (*yaml.Node, bool) {
	if w.On.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(w.On.Content); i += 2 {
		if w.On.Content[i].Value == name {
			return w.On.Content[i+1], true
		}
	}
	return nil, false
}

// triggerNames lists the on: events in file order.
func (w *workflow) triggerNames() []string {
	var out []string
	if w.On.Kind == yaml.MappingNode {
		for i := 0; i < len(w.On.Content); i += 2 {
			out = append(out, w.On.Content[i].Value)
		}
	}
	return out
}

func (w *workflow) call(t *testing.T) callSpec {
	t.Helper()
	n, ok := w.trigger("workflow_call")
	if !ok {
		t.Fatalf("%s: no workflow_call trigger", w.path)
	}
	var c callSpec
	if err := n.Decode(&c); err != nil {
		t.Fatalf("%s: workflow_call: %v", w.path, err)
	}
	return c
}

func mustJob(t *testing.T, w *workflow, id string) *job {
	t.Helper()
	j, ok := w.Jobs[id]
	if !ok {
		t.Fatalf("%s: no job %q", w.path, id)
	}
	return j
}

// mustStep returns the index and the step with the given id.
func mustStep(t *testing.T, where string, steps []step, id string) (int, step) {
	t.Helper()
	for i, s := range steps {
		if s.ID == id {
			return i, s
		}
	}
	t.Fatalf("%s: no step with id %q", where, id)
	return -1, step{}
}

// wantPerms compares a permissions block with the exact scopes expected.
func wantPerms(t *testing.T, where string, p perms, want map[string]string) {
	t.Helper()
	if !p.present || p.scalar != "" {
		t.Errorf("%s: want permissions %v, got none or a scalar", where, want)
		return
	}
	if !maps.Equal(p.levels, want) {
		t.Errorf("%s: permissions = %v, want %v", where, p.levels, want)
	}
}

func wantEnv(t *testing.T, where string, s step, want map[string]string) {
	t.Helper()
	for _, k := range slices.Sorted(maps.Keys(want)) {
		if got := s.Env[k]; got != want[k] {
			t.Errorf("%s: env %s = %q, want %q", where, k, got, want[k])
		}
	}
}

func wantWith(t *testing.T, where string, s step, want map[string]string) {
	t.Helper()
	for _, k := range slices.Sorted(maps.Keys(want)) {
		if got := s.With[k]; got != want[k] {
			t.Errorf("%s: with %s = %q, want %q", where, k, got, want[k])
		}
	}
}

func wantRun(t *testing.T, where string, s step, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if !strings.Contains(s.Run, sub) {
			t.Errorf("%s: run lacks %q", where, sub)
		}
	}
}

// wantOptionalSecrets checks that a reusable workflow declares exactly these
// secrets and requires none: a fork PR gets no secrets and must still reach the
// verdict, and each job that needs a secret names the missing one (ruling P-11).
func wantOptionalSecrets(t *testing.T, where string, c callSpec, names ...string) {
	t.Helper()
	if got, want := slices.Sorted(maps.Keys(c.Secrets)), slices.Sorted(slices.Values(names)); !slices.Equal(got, want) {
		t.Errorf("%s: secrets = %v, want %v", where, got, want)
	}
	for _, n := range slices.Sorted(maps.Keys(c.Secrets)) {
		if c.Secrets[n].Required {
			t.Errorf("%s: secret %s must not be required", where, n)
		}
	}
}

// wantSecrets checks that a reusable workflow declares exactly these secrets,
// each required (true) or optional (false).
func wantSecrets(t *testing.T, where string, c callSpec, want map[string]bool) {
	t.Helper()
	got := map[string]bool{}
	for n, s := range c.Secrets {
		got[n] = s.Required
	}
	if !maps.Equal(got, want) {
		t.Errorf("%s: secrets (name: required) = %v, want %v", where, got, want)
	}
}

// commandFields splits one script line into fields, or returns nil for lines
// that cannot run a command: blank lines, comments, and echo or printf lines,
// which only print a message that may mention idp or tofu.
func commandFields(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}
	f := strings.Fields(line)
	if f[0] == "echo" || f[0] == "printf" {
		return nil
	}
	return f
}

// invokes reports whether a run script calls idp with the subcommand sub
// anywhere on a line (after then, after ;, with extra spaces). An empty sub
// matches any idp call.
func invokes(run, sub string) bool {
	for _, l := range strings.Split(run, "\n") {
		f := commandFields(l)
		for i, w := range f {
			if w != "idp" {
				continue
			}
			if sub == "" {
				return true
			}
			if i+1 < len(f) && strings.TrimRight(f[i+1], ";|&") == sub {
				return true
			}
		}
	}
	return false
}
