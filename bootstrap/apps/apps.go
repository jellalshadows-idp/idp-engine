// Package apps embeds the GitHub App manifests, so the Apps' permissions are
// reviewable code (spec §9.2).
package apps

import "embed"

// FS holds reader.json and writer.json.
//
//go:embed *.json
var FS embed.FS
