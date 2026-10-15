// Package e2e is the end-to-end harness (spec §8.5, ADR-0027): it drives a
// claims repo through the real pipelines and checks the results through the
// GitHub API.
package e2e

import (
	"fmt"
	"regexp"
	"strings"
)

// prefix is the only claim prefix the harness creates, changes or deletes.
const prefix = "e2e-"

var runPattern = regexp.MustCompile(`^[a-z0-9]{1,12}$`)

// Names are the claim names one run owns.
type Names struct {
	Group     string
	Component string
}

// NewNames derives the names of a run from its id.
func NewNames(run string) (Names, error) {
	if !runPattern.MatchString(run) {
		return Names{}, fmt.Errorf("run %q must be 1 to 12 lowercase letters or digits", run)
	}
	n := Names{Group: prefix + "r" + run + "-team", Component: prefix + "r" + run + "-app"}
	return n, n.guard()
}

func (n Names) guard() error {
	for _, name := range []string{n.Group, n.Component} {
		if !strings.HasPrefix(name, prefix) {
			return fmt.Errorf("refusing to touch %q: the harness manages only names that start with %q", name, prefix)
		}
	}
	return nil
}

func (n Names) GroupPath() string     { return "claims/groups/" + n.Group + ".yaml" }
func (n Names) ComponentPath() string { return "claims/components/" + n.Component + ".yaml" }
