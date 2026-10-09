package main

import (
	"os"

	"github.com/jellalshadows-idp/idp-engine/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}
