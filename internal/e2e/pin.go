package e2e

import (
	"fmt"
	"regexp"
)

var (
	shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	enginePin  = regexp.MustCompile(`(idp-engine/\.github/workflows/[a-z]+\.yaml@)[0-9a-f]{40}`)
)

// PinTemplates returns the caller templates with every engine workflow ref
// pinned to sha.
func PinTemplates(templates map[string][]byte, sha string) (map[string][]byte, error) {
	if !shaPattern.MatchString(sha) {
		return nil, fmt.Errorf("engine sha %q is not a 40-character commit SHA", sha)
	}
	out := make(map[string][]byte, len(templates))
	for name, b := range templates {
		if !enginePin.Match(b) {
			return nil, fmt.Errorf("template %s has no engine workflow ref", name)
		}
		out[name] = enginePin.ReplaceAll(b, []byte("${1}"+sha))
	}
	return out, nil
}
