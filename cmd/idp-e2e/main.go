// Command idp-e2e is the end-to-end harness (spec §8.5, ADR-0027). It drives a
// claims repo through the real pipelines; it is not part of the released CLI.
package main

import (
	"os"

	"github.com/jellalshadows-idp/idp-engine/internal/e2e"
)

func main() {
	os.Exit(e2e.Main(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}
