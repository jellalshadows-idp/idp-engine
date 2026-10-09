// Package schemas embeds the JSON Schemas (draft 2020-12) of every claim kind.
// They are the single source of truth for claim structure (ADR-0015): the CLI
// validates with them, and editors use them through
// `# yaml-language-server: $schema=...`.
package schemas

import (
	"embed"
	"strings"
)

// FS holds platform.json, group.json and component.json.
//
//go:embed *.json
var FS embed.FS

// Kinds lists the kinds that have a schema, in a stable order.
var Kinds = []string{"Platform", "Group", "Component"}

// File returns the schema file name of a kind ("Group" → "group.json").
func File(kind string) string { return strings.ToLower(kind) + ".json" }
