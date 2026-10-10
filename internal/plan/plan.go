// Package plan reads OpenTofu plans (tofu show -json) and turns them into the
// summaries, fingerprints and gate decisions of spec §6.1 and §6.2 (ADR-0018).
package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Action is a normalized change kind.
type Action string

const (
	Create  Action = "create"
	Update  Action = "update"
	Delete  Action = "delete"
	Replace Action = "replace"
	Forget  Action = "forget" // dropped from state; the real resource stays
)

// Destructive reports whether the action removes a real resource.
func (a Action) Destructive() bool { return a == Delete || a == Replace }

func (a Action) valid() bool {
	switch a {
	case Create, Update, Delete, Replace, Forget:
		return true
	}
	return false
}

// Change is one resource address and what the plan does to it.
type Change struct {
	Address string `json:"address"`
	Action  Action `json:"action"`
}

// Plan is one stack's changes, sorted by address then action. no-op and read
// are excluded: they change nothing.
type Plan struct {
	Stack   string
	Changes []Change
}

// Parse reads the JSON of tofu show -json for one stack. An action it does not
// know fails the parse: an unknown action is never treated as harmless.
func Parse(stack string, r io.Reader) (*Plan, error) {
	var raw struct {
		FormatVersion   string `json:"format_version"`
		ResourceChanges []struct {
			Address string `json:"address"`
			Change  struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("plan %s: %w", stack, err)
	}
	if major, _, _ := strings.Cut(raw.FormatVersion, "."); major != "1" {
		return nil, fmt.Errorf("plan %s: unsupported format_version %q (want 1.x)", stack, raw.FormatVersion)
	}
	p := &Plan{Stack: stack, Changes: []Change{}}
	for _, rc := range raw.ResourceChanges {
		action, isChange, err := normalize(rc.Change.Actions)
		if err != nil {
			return nil, fmt.Errorf("plan %s: %s: %w", stack, rc.Address, err)
		}
		if isChange {
			p.Changes = append(p.Changes, Change{Address: rc.Address, Action: action})
		}
	}
	sortChanges(p.Changes)
	return p, nil
}

func normalize(actions []string) (Action, bool, error) {
	switch strings.Join(actions, ",") {
	case "no-op", "read":
		return "", false, nil
	case "create":
		return Create, true, nil
	case "update":
		return Update, true, nil
	case "delete":
		return Delete, true, nil
	case "delete,create", "create,delete":
		return Replace, true, nil
	case "forget":
		return Forget, true, nil
	}
	return "", false, fmt.Errorf("unsupported actions %v", actions)
}

func sortChanges(cs []Change) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Address != cs[j].Address {
			return cs[i].Address < cs[j].Address
		}
		return cs[i].Action < cs[j].Action
	})
}

// Counts tallies a plan's changes by action.
type Counts struct {
	Create, Update, Replace, Delete, Forget int
}

// Total is the number of changes.
func (c Counts) Total() int { return c.Create + c.Update + c.Replace + c.Delete + c.Forget }

// Counts tallies p's changes.
func (p *Plan) Counts() Counts {
	var c Counts
	for _, ch := range p.Changes {
		switch ch.Action {
		case Create:
			c.Create++
		case Update:
			c.Update++
		case Replace:
			c.Replace++
		case Delete:
			c.Delete++
		case Forget:
			c.Forget++
		}
	}
	return c
}

// Fingerprint is the set of changes per stack (spec §6.1 step 6): what a
// reviewer saw on the PR, and what the reconcile gate compares against it.
type Fingerprint map[string][]Change

// FingerprintOf collects the changes of plans, one entry per stack.
func FingerprintOf(plans ...*Plan) Fingerprint {
	f := Fingerprint{}
	for _, p := range plans {
		f[p.Stack] = append([]Change{}, p.Changes...)
	}
	return f
}

// Empty reports whether no stack has a change.
func (f Fingerprint) Empty() bool {
	for _, cs := range f {
		if len(cs) > 0 {
			return false
		}
	}
	return true
}

// Destructive lists "stack: action address" for every delete or replace, sorted.
func (f Fingerprint) Destructive() []string {
	return f.entries(func(_ string, c Change) bool { return c.Action.Destructive() })
}

// Missing lists "stack: action address" for every change of f that the same
// stack of g does not have, sorted. Empty means f is a subset of g.
func (f Fingerprint) Missing(g Fingerprint) []string {
	have := map[string]map[Change]bool{}
	for stack, cs := range g {
		have[stack] = map[Change]bool{}
		for _, c := range cs {
			have[stack][c] = true
		}
	}
	return f.entries(func(stack string, c Change) bool { return !have[stack][c] })
}

func (f Fingerprint) entries(keep func(stack string, c Change) bool) []string {
	out := []string{}
	for stack, cs := range f {
		for _, c := range cs {
			if keep(stack, c) {
				out = append(out, fmt.Sprintf("%s: %s %s", stack, c.Action, c.Address))
			}
		}
	}
	sort.Strings(out)
	return out
}

// ReadFingerprint decodes a fingerprint written by idp plan-summary, rejecting
// unknown fields, empty addresses and unknown actions.
func ReadFingerprint(r io.Reader) (Fingerprint, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var f Fingerprint
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("fingerprint: %w", err)
	}
	if f == nil {
		return nil, errors.New("fingerprint: want a JSON object")
	}
	for stack, cs := range f {
		for _, c := range cs {
			if c.Address == "" || !c.Action.valid() {
				return nil, fmt.Errorf("fingerprint: stack %s: invalid change %+v", stack, c)
			}
		}
		sortChanges(cs)
	}
	return f, nil
}
