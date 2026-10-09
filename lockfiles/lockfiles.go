// Package lockfiles embeds the OpenTofu dependency lock files the renderer copies
// into each stack, so `tofu init` verifies provider checksums (spec §5.3, §7.5).
// Regenerate with `tofu providers lock` for linux/windows/darwin amd64 and darwin
// arm64 whenever a provider version changes.
package lockfiles

import _ "embed"

// GitHub is the lock file of the GitHub stack (integrations/github).
//
//go:embed github.terraform.lock.hcl
var GitHub []byte
