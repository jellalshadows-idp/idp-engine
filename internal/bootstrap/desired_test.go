package bootstrap

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestDesiredGolden(t *testing.T) {
	cfg := Config{ApproverID: 42, Writer: AppCredentials{ID: 2}}
	envs := Environments(cfg)
	cases := []struct {
		name  string
		value any
	}{
		{"ruleset-main", MainRuleset()},
		{"ruleset-wet", WetRuleset(cfg.Writer.ID)},
		{"environment-approval", envs[0].putBody()},
		{"environment-write", envs[1].putBody()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.MarshalIndent(tc.value, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			path := filepath.Join("testdata", tc.name+".golden.json")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run `go test ./internal/bootstrap/ -run TestDesiredGolden -update` to create it)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s differs from its golden file:\n%s", tc.name, got)
			}
		})
	}
}

func TestEnvironmentsOrderAndReviewers(t *testing.T) {
	envs := Environments(Config{ApproverID: 42})
	if len(envs) != 2 || envs[0].Name != EnvApproval || envs[1].Name != EnvWrite {
		t.Fatalf("Environments = %+v, want [idp-approval idp-write]", envs)
	}
	if len(envs[0].ReviewerIDs) != 1 || envs[0].ReviewerIDs[0] != 42 {
		t.Errorf("idp-approval reviewers = %v, want [42]", envs[0].ReviewerIDs)
	}
	if len(envs[1].ReviewerIDs) != 0 {
		t.Errorf("idp-write must have no reviewers (approval lives in idp-approval), got %v", envs[1].ReviewerIDs)
	}
}
