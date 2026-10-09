package cli

import (
	"flag"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/render"
	"github.com/jellalshadows-idp/idp-engine/internal/version"
)

func runValidate(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", ".", "claims repo root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp validate: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	m, ok := loadClaims(*dir, stdout, stderr, env)
	if !ok {
		return 1
	}
	fmt.Fprintf(stdout, "validate: ok (%d group(s), %d component(s))\n", len(m.Groups), len(m.Components))
	return 0
}

func runRender(args []string, stdout, stderr io.Writer, env Env) int {
	fs := flag.NewFlagSet("idp render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", ".", "claims repo root")
	out := fs.String("out", "rendered", "output directory (created, or replaced if it holds a previous render)")
	ref := fs.String("module-ref", "", "idp-engine git ref that module sources pin (default: this build's version)")
	modulesDir := fs.String("modules-dir", "", "use local module sources from this directory (validation only)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "idp render: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	opts := render.Options{ModuleRef: *ref}
	if *modulesDir != "" {
		rel, err := relativeModulesDir(*out, *modulesDir)
		if err != nil {
			fmt.Fprintln(stderr, "idp render:", err)
			return 2
		}
		opts.ModulesDir = rel
	}
	if opts.ModuleRef == "" && opts.ModulesDir == "" {
		if version.Version == "dev" {
			fmt.Fprintln(stderr, "idp render: this is a development build; pass --module-ref or --modules-dir")
			return 2
		}
		opts.ModuleRef = version.Version
	}
	m, ok := loadClaims(*dir, stdout, stderr, env)
	if !ok {
		return 1
	}
	files, err := render.Render(m, opts)
	if err != nil {
		return fail(stderr, err)
	}
	if err := render.WriteTree(*out, files); err != nil {
		return fail(stderr, err)
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintf(stdout, "render: wrote %s\n", path.Join(filepath.ToSlash(*out), p))
	}
	return 0
}

// loadClaims loads and validates a claims repo and prints every diagnostic: always
// to stderr, and also to stdout as a GitHub annotation inside GitHub Actions.
func loadClaims(dir string, stdout, stderr io.Writer, env Env) (*claims.Model, bool) {
	m, diags, err := claims.Load(dir)
	if err != nil {
		fmt.Fprintln(stderr, "idp:", err)
		return nil, false
	}
	inActions := env("GITHUB_ACTIONS") == "true"
	for _, d := range diags {
		if inActions {
			fmt.Fprintln(stdout, d.Annotation())
		}
		fmt.Fprintln(stderr, d)
	}
	if len(diags) > 0 {
		fmt.Fprintf(stderr, "validate: %d problem(s)\n", len(diags))
		return nil, false
	}
	return m, true
}

// relativeModulesDir expresses modulesDir relative to <out>/github, the directory
// OpenTofu resolves local module sources from. Local sources must start with ./ or ../.
func relativeModulesDir(out, modulesDir string) (string, error) {
	stack, err := filepath.Abs(filepath.Join(out, "github"))
	if err != nil {
		return "", err
	}
	mods, err := filepath.Abs(modulesDir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(stack, mods)
	if err != nil {
		return "", fmt.Errorf("--modules-dir must be on the same volume as --out: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "../") && !strings.HasPrefix(rel, "./") {
		rel = "./" + rel
	}
	return rel, nil
}
