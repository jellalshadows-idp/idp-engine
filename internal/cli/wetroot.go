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
	wet := flags.String("wet", "", "the wet branch's rendered/ tree (required); it may not exist: then it holds nothing")
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
	if *out == "" || *base == "" || *fresh == "" || *wet == "" {
		fmt.Fprintln(stderr, "idp wet-root: --out, --wet-base, --new and --wet are required")
		return exitUsage
	}
	if *githubRender && *state == "" {
		fmt.Fprintln(stderr, "idp wet-root: --github-render needs --github-state: the github stack's render is never committed without its state")
		return exitUsage
	}
	applied, leftOut, err := readMarkers(*appliedDir)
	if err != nil {
		fmt.Fprintln(stderr, "idp wet-root:", err)
		return exitError
	}
	if _, err := os.Stat(*out); !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(stderr, "idp wet-root: --out %s already exists: give a fresh directory\n", *out)
		return exitError
	}
	for _, l := range leftOut {
		fmt.Fprintln(stderr, l)
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
			if err := writeFileIn(*out, wetpush.GitHubState, b); err != nil {
				return fail(stderr, err)
			}
			notes[wetpush.GitHubState] = "the github stack's state"
		case !errors.Is(err, fs.ErrNotExist):
			return fail(stderr, err)
		}
	}
	if *githubRender {
		applied = append(applied, "github")
	}
	kept, err := keptBaselines(*wet, *fresh, applied)
	if err != nil {
		return fail(stderr, err)
	}
	for _, baseline := range slices.Sorted(maps.Keys(kept)) {
		fmt.Fprintf(stderr, "idp wet-root: kept rendered/%s in wet: %s (spec §6.2 step 4)\n", baseline, kept[baseline])
	}
	for _, stack := range applied {
		if _, ok := kept[stack]; ok {
			continue
		}
		rel := path.Join("rendered", stack)
		src := filepath.Join(*fresh, filepath.FromSlash(stack))
		has, err := hasRender(*fresh, stack)
		if err != nil {
			return fail(stderr, err)
		}
		if !has && stack == "github" {
			// The github stack is never an orphan: deleting its render would
			// make the next reconcile plan its whole org from nothing.
			fmt.Fprintf(stderr, "idp wet-root: left out %s: the new render has no github stack, so wet keeps its render\n", rel)
			continue
		}
		if !has {
			notes[rel] = "deleted, the orphan's destroy applied"
			continue
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

// hasRender reports whether root holds a render of stack.
func hasRender(root, stack string) (bool, error) {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(stack), wetdiff.StackFile))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// keptBaselines finds the applied orphan baselines whose render must stay in
// wet, each with why (spec §6.2 step 4): a Component of the baseline's
// environment keeps a render in wet after this commit, and a Component stack
// cannot plan without its baseline; or wet holds no render of the baseline, so
// it is not the tree the reconcile planned against, and cannot tell. wet may
// not exist.
func keptBaselines(wet, fresh string, applied []string) (map[string]string, error) {
	kept := map[string]string{}
	for _, stack := range applied {
		env, ok := strings.CutSuffix(stack, "/_baseline")
		if !ok {
			continue
		}
		rendered, err := hasRender(fresh, stack)
		if err != nil {
			return nil, err
		}
		if rendered {
			continue // no orphan: its applied render commits
		}
		inWet, err := hasRender(wet, stack)
		if err != nil {
			return nil, err
		}
		if !inWet {
			kept[stack] = "--wet holds no render of it, so it is not the tree the reconcile planned against"
			continue
		}
		staying, err := stayingComponents(wet, fresh, env, applied)
		if err != nil {
			return nil, err
		}
		if len(staying) > 0 {
			kept[stack] = "a Component stack needs its baseline, and these Component renders stay there: " + strings.Join(staying, ", ")
		}
	}
	return kept, nil
}

// stayingComponents lists, sorted, the rendered/ paths of env's Component
// renders in wet that this commit leaves there: a Component render leaves wet
// only when it applied and has no render in fresh.
func stayingComponents(wet, fresh, env string, applied []string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(wet, filepath.FromSlash(env), "components"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var staying []string
	for _, e := range entries {
		if !e.IsDir() {
			continue // a Component render is a directory
		}
		component := path.Join(env, "components", e.Name())
		inWet, err := hasRender(wet, component)
		if err != nil {
			return nil, err
		}
		inNew, err := hasRender(fresh, component)
		if err != nil {
			return nil, err
		}
		leaves := slices.Contains(applied, component) && !inNew
		if inWet && !leaves {
			staying = append(staying, "rendered/"+component)
		}
	}
	slices.Sort(staying)
	return staying, nil
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
			fmt.Fprintf(stderr, "idp wet-root: left out %s: wet holds no state but %s (ADR-0030)\n", path.Join(rel, p), wetpush.GitHubState)
			return nil
		}
		b, err := fs.ReadFile(srcFS, p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}

// readMarkers reads apply-aws's applied-stack markers: each a file named after
// an AWS stack, each / as ., holding the stack and a newline. A missing
// directory holds none. An entry that is no such marker (wrong content,
// misnamed, not a regular file) is not read as one: leftOut names it, one
// stderr line each, so one foreign entry never drops the github state's commit.
func readMarkers(dir string) (stacks, leftOut []string, err error) {
	if dir == "" {
		return nil, nil, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		file := filepath.Join(dir, e.Name())
		foreign := "idp wet-root: left out " + file + ": not an applied-stack marker: want a file named after an AWS stack, each / as ., that holds the stack and a newline"
		if !e.Type().IsRegular() {
			leftOut = append(leftOut, foreign)
			continue
		}
		b, err := readMarker(file)
		if err != nil {
			return nil, nil, err
		}
		stack, ok := strings.CutSuffix(string(b), "\n")
		if !ok || !strings.HasPrefix(stack, "aws/") || !wetdiff.Reconcilable(stack) || wetdiff.Escape(stack) != e.Name() {
			leftOut = append(leftOut, foreign)
			continue
		}
		stacks = append(stacks, stack)
	}
	slices.Sort(stacks)
	return stacks, leftOut, nil
}

// maxMarkerSize bounds a marker: the longest AWS stack path, aws/<env>/components/<name>,
// has 66 bytes, and its marker one more for the newline.
const maxMarkerSize = 128

// readMarker reads at most maxMarkerSize+1 bytes of a marker: a job that runs
// floci can upload any size, and a longer file is not a marker anyway, so a
// huge one neither stalls wet-root nor exhausts its memory (ADR-0031).
func readMarker(file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxMarkerSize+1))
}

// writeFileIn writes data at the slash path rel under root.
func writeFileIn(root, rel string, data []byte) error {
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
