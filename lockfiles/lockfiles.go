// Package lockfiles embeds the OpenTofu dependency lock files the renderer copies
// into each stack, so `tofu init` verifies provider checksums (spec §5.3, §7.5).
// Regenerate with `tofu providers lock` for linux/windows/darwin amd64 and darwin
// arm64 whenever a provider version changes (docs/runbooks/dependencies.md).
package lockfiles

import _ "embed"

// GitHub is the lock file of the GitHub stack (integrations/github).
//
//go:embed github.terraform.lock.hcl
var GitHub []byte

// AWS is the lock file of the AWS stacks (hashicorp/aws). The floci tests copy
// it into the stacks they apply, as the render will from Phase 2d.
//
//go:embed aws.terraform.lock.hcl
var AWS []byte
