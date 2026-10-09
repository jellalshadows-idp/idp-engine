// Package bootstrap creates and verifies the org-level setup the IDP depends on
// (spec §9.2): the claims repo, the wet branch, rulesets, environments,
// secrets and variables. It deliberately lives outside the claims loop.
package bootstrap

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// Mismatches lists the paths where actual does not contain desired.
// Objects are compared as subsets, so keys GitHub adds on its own never count.
// Arrays must have the same length and match element by element. Both values
// must be in encoding/json's generic form (see normalize).
func Mismatches(desired, actual any) []string {
	var out []string
	walk("$", desired, actual, &out)
	return out
}

func walk(path string, desired, actual any, out *[]string) {
	switch d := desired.(type) {
	case map[string]any:
		a, ok := actual.(map[string]any)
		if !ok {
			*out = append(*out, path)
			return
		}
		keys := make([]string, 0, len(d))
		for k := range d {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walk(path+"."+k, d[k], a[k], out)
		}
	case []any:
		a, ok := actual.([]any)
		if !ok || len(a) != len(d) {
			*out = append(*out, path)
			return
		}
		for i := range d {
			walk(fmt.Sprintf("%s[%d]", path, i), d[i], a[i], out)
		}
	default:
		if !reflect.DeepEqual(desired, actual) {
			*out = append(*out, path)
		}
	}
}

// normalize turns any JSON-encodable value into the generic form Mismatches expects.
func normalize(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	err = json.Unmarshal(raw, &out)
	return out, err
}

// asList returns v as a list, or an empty list when GitHub omitted the field.
func asList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return []any{}
}
