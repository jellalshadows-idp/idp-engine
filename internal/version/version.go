// Package version identifies this build of idp.
package version

// Version is the release tag of this build, set at release time with
// -ldflags "-X github.com/jellalshadows-idp/idp-engine/internal/version.Version=v0.1.0".
// Rendered module sources pin it (spec §5.3).
var Version = "dev"

// Repo is the engine's module path; rendered module sources point at it.
const Repo = "github.com/jellalshadows-idp/idp-engine"
