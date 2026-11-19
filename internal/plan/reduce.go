package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// reduced is the part of tofu show -json that Parse reads: the format version
// and each resource change's address and actions. It holds no value, no state
// and no configuration, so it is what crosses jobs as an artifact (ADR-0031).
type reduced struct {
	FormatVersion   string          `json:"format_version"`
	ResourceChanges []reducedChange `json:"resource_changes"`
}

type reducedChange struct {
	Address string `json:"address"`
	Change  struct {
		Actions []string `json:"actions"`
	} `json:"change"`
}

// Reduce reads the JSON of tofu show -json for one stack and returns its plan
// and its reduced form, one line of JSON that Parse reads back to the same
// plan. It rejects what Parse rejects.
func Reduce(stack string, r io.Reader) (*Plan, []byte, error) {
	var raw reduced
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("plan %s: %w", stack, err)
	}
	if raw.ResourceChanges == nil {
		raw.ResourceChanges = []reducedChange{}
	}
	out, err := json.Marshal(raw)
	if err != nil {
		return nil, nil, err
	}
	p, err := Parse(stack, bytes.NewReader(out))
	if err != nil {
		return nil, nil, err
	}
	return p, append(out, '\n'), nil
}
