package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/actions"
	"github.com/jellalshadows-idp/idp-engine/internal/wetdiff"
	"github.com/jellalshadows-idp/idp-engine/internal/wetpush"
)

// runWetRoot prepares apply-github's wet commit (spec §6.2 step 4): a root
// holding wet-base, the github stack's state, its render when it applied, and
// the render of each AWS stack apply-aws marked applied, plus wet-paths, the
// paths idp wet-push commits, one per line. An applied orphan's path is listed
// with no render under the root, so wet-push deletes it. The root is saved as
// the idp-wet-root artifact, which the recover workflow pushes again.
func runWetRoot(args []string, stdout, stderr io.Writer, env Env) int {
	flags := flag.NewFlagSet("idp wet-root", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "", "the wet root to write; it must not exist yet (required)")
	base := flags.String("wet-base", "", "the wet commit the reconcile planned against, written to wet-base (required)")
	fresh := flags.String("new", "", "the new render's root (required)")
	appliedDir := flags.String("applied-dir", "", "apply-aws's markers, one per AWS stack that applied; it may be missing")
	state := flags.String("github-state", "", "the github stack's state; committed when the file exists")
	githubRender := flags.Bool("github-render", false, "commit the github stack's render: its apply succeeded or had nothing to apply (needs --github-state)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "idp wet-root: unexpected argument %q\n", flags.Arg(0))
		return exitUsage
	}
	if *out == "" || *base == "" || *fresh == "" {
		fmt.Fprintln(stderr, "idp wet-root: --out, --wet-base and --new are required")
		return exitUsage
	}
	if *githubRender && *state == "" {
		fmt.Fprintln(stderr, "idp wet-root: --github-render needs --github-state: the github stack's render is never committed without its state")
		return exitUsage
	}
	applied, err := readMarkers(*appliedDir)
	if err != nil {
		fmt.Fprintln(stderr, "idp wet-root:", err)
		return exitError
	}
	if _, err := os.Stat(*out); !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(stderr, "idp wet-root: --out %s already exists: give a fresh directory\n", *out)
		return exitError
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(filepath.Join(*out, "wet-base"), []byte(*base+"\n"), 0o644); err != nil {
		return fail(stderr, err)
	}
	notes := map[string]string{}
	if *state != "" {
		b, err := os.ReadFile(*state)
		switch {
		case err == nil:
			if err := writeFileIn(*out, githubStatePath, b); err != nil {
				return fail(stderr, err)
			}
			notes[githubStatePath] = "the github stack's state"
		case !errors.Is(err, fs.ErrNotExist):
			return fail(stderr, err)
		}
	}
	if *githubRender {
		applied = append(applied, "github")
	}
	for _, stack := range applied {
		rel := path.Join("rendered", stack)
		src := filepath.Join(*fresh, filepath.FromSlash(stack))
		if _, err := os.Stat(filepath.Join(src, wetdiff.StackFile)); errors.Is(err, fs.ErrNotExist) {
			notes[rel] = "deleted, the orphan's destroy applied"
			continue
		} else if err != nil {
			return fail(stderr, err)
		}
		if err := copyRender(filepath.Join(*out, filepath.FromSlash(rel)), src, rel, stderr); err != nil {
			return fail(stderr, err)
		}
		notes[rel] = "the applied render"
	}
	paths := slices.Sorted(maps.Keys(notes))
	var list strings.Builder
	for _, p := range paths {
		fmt.Fprintf(stdout, "%s: %s\n", p, notes[p])
		list.WriteString(p + "\n")
	}
	if err := os.WriteFile(filepath.Join(*out, "wet-paths"), []byte(list.String()), 0o644); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "wet-root: %d path(s) to commit\n", len(paths))
	if err := actions.SetOutput(env("GITHUB_OUTPUT"), "paths", strconv.Itoa(len(paths))); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}

// copyRender copies a stack's render from src to dst and leaves out any state
// file, naming it on stderr: wet-push refuses one, and its refusal would drop the
// github state's commit too (ADR-0030). rel is the render's path in wet.
func copyRender(dst, src, rel string, stderr io.Writer) error {
	srcFS := os.DirFS(src)
	return fs.WalkDir(srcFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(p))
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case !d.Type().IsRegular():
			return fmt.Errorf("%s: not a regular file", path.Join(rel, p))
		case wetpush.IsState(p):
			fmt.Fprintf(stderr, "idp wet-root: left out %s: wet holds no state but %s (ADR-0030)\n", path.Join(rel, p), githubStatePath)
			return nil
		}
		b, err := fs.ReadFile(srcFS, p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}

// githubStatePath is the one state file wet holds (spec §5.4).
const githubStatePath = "tfstate/github.tfstate"

// readMarkers reads apply-aws's applied-stack markers: each a file named after
// an AWS stack, each / as ., holding the stack and a newline. A missing
// directory holds none.
func readMarkers(dir string) ([]string, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var stacks []string
	for _, e := range entries {
		file := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		stack, ok := strings.CutSuffix(string(b), "\n")
		if !ok || !strings.HasPrefix(stack, "aws/") || !wetdiff.Reconcilable(stack) || wetdiff.Escape(stack) != e.Name() {
			return nil, fmt.Errorf("%s: not an applied-stack marker: want a file named after an AWS stack, each / as ., that holds the stack and a newline", file)
		}
		stacks = append(stacks, stack)
	}
	slices.Sort(stacks)
	return stacks, nil
}

// writeFileIn writes data at the slash path rel under root.
func writeFileIn(root, rel string, data []byte) error {
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
