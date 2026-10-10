package plan

import (
	"strings"
	"testing"
)

func TestDecide(t *testing.T) {
	create := Fingerprint{"github": {{"a", Create}}}
	update := Fingerprint{"github": {{"a", Update}}}
	destroy := Fingerprint{"github": {{"a", Delete}}}
	tests := []struct {
		name       string
		current    Fingerprint
		pr         *PRPlan
		why        string
		want       Decision
		wantReason string
	}{
		{"nothing planned is auto even without a PR", Fingerprint{"github": {}}, nil, "", Auto, "no changes"},
		{"destructive is approval even with a matching PR", destroy, &PRPlan{Number: 1, Fingerprint: destroy}, "", Approval, "destructive changes: github: delete a"},
		{"no PR is approval with its reason", create, nil, "no merged pull request", Approval, "no merged pull request"},
		{"no PR without a reason gets a default one", create, nil, "", Approval, "no pull request plan"},
		{"subset of the PR is auto", create, &PRPlan{Number: 7, Fingerprint: Fingerprint{"github": {{"a", Create}, {"b", Create}}}}, "", Auto, "PR #7"},
		{"an action the PR did not show is approval", update, &PRPlan{Number: 7, Fingerprint: create}, "", Approval, "changes not shown in PR #7: github: update a"},
		{"a stack the PR did not show is approval", Fingerprint{"aws/dev/_baseline": {{"x", Create}}}, &PRPlan{Number: 7, Fingerprint: create}, "", Approval, "aws/dev/_baseline: create x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Decide(tt.current, tt.pr, tt.why)
			if v.Decision != tt.want || !strings.Contains(v.Reason, tt.wantReason) {
				t.Errorf("Decide = %+v, want %s containing %q", v, tt.want, tt.wantReason)
			}
		})
	}
}

func TestDecideSummarizesLongLists(t *testing.T) {
	var cs []Change
	for _, a := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		cs = append(cs, Change{a, Delete})
	}
	v := Decide(Fingerprint{"github": cs}, nil, "")
	if !strings.HasSuffix(v.Reason, "and 2 more") {
		t.Errorf("reason = %q, want it to end with %q", v.Reason, "and 2 more")
	}
}

func TestNeedsPR(t *testing.T) {
	if NeedsPR(Fingerprint{}) || NeedsPR(Fingerprint{"github": {{"a", Delete}}}) || !NeedsPR(Fingerprint{"github": {{"a", Create}}}) {
		t.Error("NeedsPR must be true only for non-empty, non-destructive fingerprints")
	}
}
