package bootstrap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The in-memory fake echoes request bodies, so only real GitHub captures can
// prove that desired state matches the shape GitHub stores and returns.
func TestDesiredRulesetsMatchLiveGitHub(t *testing.T) {
	cases := []struct {
		name    string
		desired Ruleset
		fixture string
	}{
		{"idp-main", MainRuleset(), "live-ruleset-idp-main.json"},
		{"idp-wet", WetRuleset(5255579), "live-ruleset-idp-wet.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			norm, err := normalize(tc.desired)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join("testdata", tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			var live map[string]any
			if err := json.Unmarshal(raw, &live); err != nil {
				t.Fatal(err)
			}
			if got := Mismatches(rulesetView(norm.(map[string]any)), rulesetView(live)); len(got) != 0 {
				t.Errorf("desired ruleset drifts from live GitHub at %v", got)
			}
		})
	}
}
