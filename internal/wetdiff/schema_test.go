package wetdiff

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/schemas"
)

// schemaPattern returns the pattern at path inside the embedded schema of kind,
// without its ^ and $ anchors.
func schemaPattern(t *testing.T, kind string, path ...string) string {
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
	return strings.TrimSuffix(strings.TrimPrefix(pattern, "^"), "$")
}

// TestReconcilableFollowsTheSchemas: the stacks this version reconciles are
// named with the claims schemas' own rules for an environment and a Component,
// so a valid claim's stack is never refused and the two cannot drift apart.
func TestReconcilableFollowsTheSchemas(t *testing.T) {
	env := schemaPattern(t, "Platform", "properties", "environments", "propertyNames", "pattern")
	component := schemaPattern(t, "Component", "properties", "name", "pattern")
	want := `^(?:github|aws/` + env + `/(?:_baseline|components/` + component + `))$`
	if got := reconcilable.String(); got != want {
		t.Errorf("reconcilable = %s\nwant %s (from the schemas)", got, want)
	}
}
