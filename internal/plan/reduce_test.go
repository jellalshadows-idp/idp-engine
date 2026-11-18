package plan

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestReduceKeepsOnlyWhatTheFingerprintReads: a reduced plan holds the format
// version and each resource change's address and actions, nothing of the
// values, the state or the configuration, and parses to the same plan as the
// full one (ADR-0031).
func TestReduceKeepsOnlyWhatTheFingerprintReads(t *testing.T) {
	for _, name := range []string{"mixed.json", "creates.json"} {
		raw, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		full, err := Parse("aws/dev/components/api", bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		p, reduced, err := Reduce("aws/dev/components/api", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		again, err := Parse("aws/dev/components/api", bytes.NewReader(reduced))
		if err != nil || !reflect.DeepEqual(p, full) || !reflect.DeepEqual(again, full) {
			t.Errorf("%s: Reduce's plan %+v and its reparse %+v (err %v), want %+v", name, p, again, err, full)
		}
		var doc struct {
			Top     map[string]json.RawMessage
			Changes []map[string]json.RawMessage `json:"resource_changes"`
		}
		if err := json.Unmarshal(reduced, &doc.Top); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(reduced, &doc); err != nil {
			t.Fatal(err)
		}
		if got := slices.Sorted(maps.Keys(doc.Top)); !slices.Equal(got, []string{"format_version", "resource_changes"}) {
			t.Errorf("%s: reduced keys %v", name, got)
		}
		for _, rc := range doc.Changes {
			var change map[string]json.RawMessage
			if err := json.Unmarshal(rc["change"], &change); err != nil {
				t.Fatal(err)
			}
			if got := slices.Sorted(maps.Keys(rc)); !slices.Equal(got, []string{"address", "change"}) || len(change) != 1 || change["actions"] == nil {
				t.Errorf("%s: reduced resource change %s", name, rc)
			}
		}
	}
}

// TestReduceFormat pins the bytes: one line of JSON, no-op changes kept, and an
// empty list rather than null.
func TestReduceFormat(t *testing.T) {
	for in, want := range map[string]string{
		`{"format_version":"1.2","terraform_version":"1.13.2","resource_changes":[{"address":"a","mode":"managed","change":{"actions":["no-op"],"before":{"secret":"x"}}}],"prior_state":{}}`: `{"format_version":"1.2","resource_changes":[{"address":"a","change":{"actions":["no-op"]}}]}` + "\n",
		`{"format_version":"1.2"}`: `{"format_version":"1.2","resource_changes":[]}` + "\n",
	} {
		_, got, err := Reduce("github", strings.NewReader(in))
		if err != nil || string(got) != want {
			t.Errorf("Reduce(%s) = %q, %v; want %q", in, got, err, want)
		}
	}
}

// TestReduceRejectsWhatParseRejects: an unknown action or format never crosses
// jobs as an artifact.
func TestReduceRejectsWhatParseRejects(t *testing.T) {
	for in, want := range map[string]string{
		`{"format_version":"2.0"}`: `plan github: unsupported format_version "2.0" (want 1.x)`,
		`{"format_version":"1.2","resource_changes":[{"address":"x","change":{"actions":["frobnicate"]}}]}`: "plan github: x: unsupported actions [frobnicate]",
		`not json`: "plan github: invalid character 'o' in literal null (expecting 'u')",
	} {
		if _, _, err := Reduce("github", strings.NewReader(in)); err == nil || err.Error() != want {
			t.Errorf("Reduce(%s) err = %v, want %q", in, err, want)
		}
	}
}
