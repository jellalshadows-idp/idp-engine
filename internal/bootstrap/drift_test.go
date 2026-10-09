package bootstrap

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
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

const bypassHiddenMsg = "$.bypass_actors (not returned to this token; GitHub only shows bypass actors to callers with write access)"

// A read-only token never receives bypass_actors; that must not be read as "[]".
func TestRulesetDriftFailsClosedWhenBypassActorsHidden(t *testing.T) {
	fake, api := newFake(t)
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	main := fake.RulesetByName("idp-main")
	delete(main, "bypass_actors")

	got, err := b.rulesetDrift(ctx, main["id"].(int64), MainRuleset())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got, bypassHiddenMsg) {
		t.Errorf("drift = %v, want it to include %q", got, bypassHiddenMsg)
	}
}

// The owner-token shape (present but empty) keeps comparing normally.
func TestRulesetDriftEmptyBypassActorsIsNotHidden(t *testing.T) {
	fake, api := newFake(t)
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	main := fake.RulesetByName("idp-main")
	main["bypass_actors"] = []any{}

	got, err := b.rulesetDrift(ctx, main["id"].(int64), MainRuleset())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("drift = %v, want none", got)
	}
}

// A JSON null is as unverifiable as an absent key.
func TestRulesetDriftFailsClosedWhenBypassActorsNull(t *testing.T) {
	fake, api := newFake(t)
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	main := fake.RulesetByName("idp-main")
	main["bypass_actors"] = nil

	got, err := b.rulesetDrift(ctx, main["id"].(int64), MainRuleset())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got, bypassHiddenMsg) {
		t.Errorf("drift = %v, want it to include %q", got, bypassHiddenMsg)
	}
}

// idp-wet desires one bypass actor, so the generic mismatch must be replaced
// by the dedicated message, not reported next to it.
func TestRulesetDriftWetHiddenBypassActorsReportsOnlyDedicatedMessage(t *testing.T) {
	fake, api := newFake(t)
	b := &Bootstrapper{API: api, Cfg: validConfig(), Log: io.Discard}
	ctx := context.Background()
	if err := b.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	wet := fake.RulesetByName("idp-wet")
	delete(wet, "bypass_actors")

	got, err := b.rulesetDrift(ctx, wet["id"].(int64), WetRuleset(b.Cfg.Writer.ID))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{bypassHiddenMsg}; !slices.Equal(got, want) {
		t.Errorf("drift = %v, want exactly %v", got, want)
	}
}
