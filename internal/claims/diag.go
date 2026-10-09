// Package claims loads and validates a claims repo: config/platform.yaml,
// claims/groups/*.yaml and claims/components/*.yaml (spec §4). Every problem is
// reported at once, located by file and line (spec §4.7).
package claims

import (
	"fmt"
	"sort"
	"strings"
)

// Diagnostic is one problem found in a claims repo.
type Diagnostic struct {
	File    string // slash-separated path relative to the claims repo root
	Line    int    // 1-based; 0 when the problem has no line (e.g. a missing file)
	Message string
}

func (d Diagnostic) String() string {
	if d.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", d.File, d.Line, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.File, d.Message)
}

// Annotation formats d as a GitHub Actions error annotation, escaped as the
// workflow-commands docs require, so the problem shows inline on the PR.
func (d Diagnostic) Annotation() string {
	props := "file=" + escapeProperty(d.File)
	if d.Line > 0 {
		props += fmt.Sprintf(",line=%d", d.Line)
	}
	return "::error " + props + "::" + escapeData(d.Message)
}

var (
	dataEscaper     = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	propertyEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

func escapeData(s string) string     { return dataEscaper.Replace(s) }
func escapeProperty(s string) string { return propertyEscaper.Replace(s) }

// sortDiagnostics orders diagnostics by file, line and message, so output is stable.
func sortDiagnostics(ds []Diagnostic) {
	sort.Slice(ds, func(i, j int) bool {
		if ds[i].File != ds[j].File {
			return ds[i].File < ds[j].File
		}
		if ds[i].Line != ds[j].Line {
			return ds[i].Line < ds[j].Line
		}
		return ds[i].Message < ds[j].Message
	})
}
