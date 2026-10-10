package plan

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func parseFixture(t *testing.T, name string) *Plan {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p, err := Parse("github", f)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

const member = `module.group_platform.terraform_data.member`

func TestParseCreatesFixture(t *testing.T) {
	want := []Change{
		{member + `["alice"]`, Create},
		{member + `["bob"]`, Create},
		{member + `["carol"]`, Create},
		{"terraform_data.replaced", Create},
	}
	if got := parseFixture(t, "creates.json").Changes; !reflect.DeepEqual(got, want) {
		t.Errorf("changes =\n%v\nwant\n%v", got, want)
	}
}

func TestParseMixedFixture(t *testing.T) {
	p := parseFixture(t, "mixed.json")
	want := []Change{
		{member + `["bob"]`, Update},
		{member + `["carol"]`, Delete},
		{member + `["dave"]`, Create},
		{"terraform_data.replaced", Replace},
	}
	if !reflect.DeepEqual(p.Changes, want) {
		t.Errorf("changes =\n%v\nwant\n%v", p.Changes, want)
	}
	if got := p.Counts(); got != (Counts{Create: 1, Update: 1, Replace: 1, Delete: 1}) || got.Total() != 4 {
		t.Errorf("counts = %+v (total %d)", got, got.Total())
	}
}

func TestParseNormalizesActions(t *testing.T) {
	src := `{"format_version":"1.2","resource_changes":[
		{"address":"data.x.y","change":{"actions":["read"]}},
		{"address":"b","change":{"actions":["create","delete"]}},
		{"address":"a","change":{"actions":["delete","create"]}},
		{"address":"c","change":{"actions":["no-op"]}},
		{"address":"d","change":{"actions":["forget"]}}]}`
	p, err := Parse("s", strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []Change{{"a", Replace}, {"b", Replace}, {"d", Forget}}
	if !reflect.DeepEqual(p.Changes, want) {
		t.Errorf("changes = %v, want %v", p.Changes, want)
	}
}

func TestParseRejectsUnknownActions(t *testing.T) {
	src := `{"format_version":"1.2","resource_changes":[{"address":"x","change":{"actions":["teleport"]}}]}`
	if _, err := Parse("s", strings.NewReader(src)); err == nil || !strings.Contains(err.Error(), "teleport") {
		t.Errorf("err = %v, want it to name the unsupported action", err)
	}
}

func TestParseRejectsOtherFormatVersions(t *testing.T) {
	for _, v := range []string{`"2.0"`, `""`} {
		src := `{"format_version":` + v + `,"resource_changes":[]}`
		if _, err := Parse("s", strings.NewReader(src)); err == nil {
			t.Errorf("format_version %s: want an error", v)
		}
	}
}

func TestParseEmptyPlanHasNonNilChanges(t *testing.T) {
	p, err := Parse("s", strings.NewReader(`{"format_version":"1.2"}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Changes == nil || len(p.Changes) != 0 {
		t.Errorf("changes = %#v, want empty and non-nil", p.Changes)
	}
}

func TestFingerprint(t *testing.T) {
	mixed := parseFixture(t, "mixed.json")
	f := FingerprintOf(mixed)
	if f.Empty() {
		t.Fatal("fingerprint of a plan with changes must not be empty")
	}
	wantDestructive := []string{
		"github: delete " + member + `["carol"]`,
		"github: replace terraform_data.replaced",
	}
	if got := f.Destructive(); !reflect.DeepEqual(got, wantDestructive) {
		t.Errorf("Destructive = %v, want %v", got, wantDestructive)
	}
	wider := Fingerprint{"github": append(append([]Change{}, mixed.Changes...), Change{"extra", Create})}
	if missing := f.Missing(wider); len(missing) != 0 {
		t.Errorf("Missing(wider) = %v, want none", missing)
	}
	if missing := wider.Missing(f); !reflect.DeepEqual(missing, []string{"github: create extra"}) {
		t.Errorf("Missing(narrower) = %v", missing)
	}
	if missing := f.Missing(Fingerprint{"other": mixed.Changes}); len(missing) != 4 {
		t.Errorf("a stack missing from g must make every change missing, got %v", missing)
	}
	if !(Fingerprint{"github": {}}).Empty() || !(Fingerprint{}).Empty() {
		t.Error("fingerprints without changes must be empty")
	}
}

func TestReadFingerprint(t *testing.T) {
	f, err := ReadFingerprint(strings.NewReader(`{"github":[{"address":"a","action":"create"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f, Fingerprint{"github": {{"a", Create}}}) {
		t.Errorf("fingerprint = %v", f)
	}
	for _, bad := range []string{
		`{"github":[{"address":"a","action":"teleport"}]}`,
		`{"github":[{"address":"","action":"create"}]}`,
		`{"github":[{"address":"a","action":"create","extra":1}]}`,
		`[]`,
	} {
		if _, err := ReadFingerprint(strings.NewReader(bad)); err == nil {
			t.Errorf("ReadFingerprint(%s) succeeded, want an error", bad)
		}
	}
}
