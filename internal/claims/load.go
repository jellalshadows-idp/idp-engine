package claims

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Load reads and validates the claims repo rooted at dir. It reports every problem
// at once; the model is only meaningful when there are no diagnostics. The error
// is reserved for failures of the tool itself (e.g. a broken embedded schema).
func Load(dir string) (*Model, []Diagnostic, error) {
	v, err := newValidator()
	if err != nil {
		return nil, nil, err
	}
	l := &loader{dir: dir, v: v, model: &Model{}, groupFiles: map[string]bool{}, docs: map[string]*document{}}
	l.loadPlatform()
	l.checkClaimsRoot()
	l.loadGroups()
	l.loadComponents()
	l.checkSemantics()
	sort.Slice(l.model.Groups, func(i, j int) bool { return l.model.Groups[i].Name < l.model.Groups[j].Name })
	sort.Slice(l.model.Components, func(i, j int) bool { return l.model.Components[i].Name < l.model.Components[j].Name })
	sortDiagnostics(l.diags)
	return l.model, l.diags, nil
}

type loader struct {
	dir        string
	v          *validator
	model      *Model
	diags      []Diagnostic
	platformOK bool                 // config/platform.yaml decoded cleanly
	groupFiles map[string]bool      // group names that have a file, valid or not
	docs       map[string]*document // "Group/<name>" or "Component/<name>" → its document
}

func (l *loader) add(ds ...Diagnostic) { l.diags = append(l.diags, ds...) }

func (l *loader) read(rel string) (*document, bool) {
	full := filepath.Join(l.dir, filepath.FromSlash(rel))
	if info, err := os.Lstat(full); err == nil && !info.Mode().IsRegular() {
		l.add(Diagnostic{File: rel, Message: "must be a regular file, not a symlink or special file"})
		return nil, false
	}
	data, err := os.ReadFile(full)
	if err != nil {
		msg := err.Error()
		if errors.Is(err, fs.ErrNotExist) {
			msg = "file not found"
		}
		l.add(Diagnostic{File: rel, Message: msg})
		return nil, false
	}
	doc, ds := parseFile(rel, data)
	l.add(ds...)
	return doc, doc != nil
}

// decode validates doc against its kind's schema and, only if valid, decodes it.
func (l *loader) decode(kind string, doc *document, out any) bool {
	ds := l.v.validate(kind, doc)
	l.add(ds...)
	if len(ds) > 0 {
		return false
	}
	if err := doc.root.Decode(out); err != nil {
		l.add(Diagnostic{File: doc.file, Line: doc.root.Line, Message: err.Error()})
		return false
	}
	return true
}

func (l *loader) loadPlatform() {
	doc, ok := l.read("config/platform.yaml")
	if !ok || !l.decode("Platform", doc, &l.model.Platform) {
		return
	}
	l.platformOK = true
	n := l.model.Platform.Naming
	for _, p := range n.ReservedPrefixes {
		if n.RequiredPrefix != "" && p == n.RequiredPrefix {
			l.add(Diagnostic{File: doc.file, Line: doc.lines["/naming/requiredPrefix"],
				Message: fmt.Sprintf("naming.requiredPrefix %q is also listed in naming.reservedPrefixes", p)})
		}
	}
}

// checkClaimsRoot reports anything under claims/ other than groups/ and components/.
func (l *loader) checkClaimsRoot() {
	entries, err := os.ReadDir(filepath.Join(l.dir, "claims"))
	if errors.Is(err, fs.ErrNotExist) {
		return // a repo with no claims yet is valid
	}
	if err != nil {
		l.add(Diagnostic{File: "claims", Message: err.Error()})
		return
	}
	for _, e := range entries {
		name := e.Name()
		switch {
		case strings.HasPrefix(name, "."):
		case e.IsDir() && (name == "groups" || name == "components"):
		case e.IsDir() && name == "workspaces":
			l.add(Diagnostic{File: "claims/workspaces", Message: "Workspace claims are not supported yet (they arrive in Phase 3)"})
		default:
			l.add(Diagnostic{File: "claims/" + name, Message: "unexpected entry; claims/ holds only groups/ and components/"})
		}
	}
}

// claimFiles lists the *.yaml files of a claims directory and reports anything
// else. Dotfiles (e.g. .gitkeep) are ignored.
func (l *loader) claimFiles(dirRel string) []string {
	entries, err := os.ReadDir(filepath.Join(l.dir, filepath.FromSlash(dirRel)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		l.add(Diagnostic{File: dirRel, Message: err.Error()})
		return nil
	}
	var files []string
	for _, e := range entries {
		rel := dirRel + "/" + e.Name()
		switch {
		case strings.HasPrefix(e.Name(), "."):
		case e.IsDir():
			l.add(Diagnostic{File: rel, Message: "unexpected directory; claims are files directly in " + dirRel})
		case !e.Type().IsRegular():
			// A symlink would let a claims PR make the loader read any file on the runner.
			l.add(Diagnostic{File: rel, Message: "unexpected symlink or special file; claim files must be regular files"})
		case strings.HasSuffix(e.Name(), ".yaml"):
			files = append(files, rel)
		default:
			l.add(Diagnostic{File: rel, Message: "unexpected file; claim files must end with .yaml"})
		}
	}
	sort.Strings(files)
	return files
}

func fileStem(rel string) string { return strings.TrimSuffix(path.Base(rel), ".yaml") }

// checkFileName reports whether name equals the file's stem, adding a diagnostic
// when it does not. A claim whose name mismatches is excluded from the model: the
// mismatch diagnostic already covers it, and since names then equal file stems
// they are unique per directory, so semantic checks never mix up documents.
func (l *loader) checkFileName(doc *document, name string) bool {
	want := fileStem(doc.file)
	if name == want {
		return true
	}
	l.add(Diagnostic{File: doc.file, Line: doc.lines["/name"],
		Message: fmt.Sprintf("name %q must match the file name %q", name, want)})
	return false
}

func (l *loader) loadGroups() {
	for _, rel := range l.claimFiles("claims/groups") {
		l.groupFiles[fileStem(rel)] = true
		doc, ok := l.read(rel)
		if !ok {
			continue
		}
		var g Group
		if !l.decode("Group", doc, &g) {
			continue
		}
		if !l.checkFileName(doc, g.Name) {
			continue
		}
		l.docs["Group/"+g.Name] = doc
		l.model.Groups = append(l.model.Groups, g)
	}
}

func (l *loader) loadComponents() {
	for _, rel := range l.claimFiles("claims/components") {
		doc, ok := l.read(rel)
		if !ok {
			continue
		}
		var c Component
		if !l.decode("Component", doc, &c) {
			continue
		}
		if !l.checkFileName(doc, c.Name) {
			continue
		}
		l.docs["Component/"+c.Name] = doc
		l.model.Components = append(l.model.Components, c)
	}
}

func (l *loader) checkSemantics() {
	for _, g := range l.model.Groups {
		doc := l.docs["Group/"+g.Name]
		l.checkPrefix(doc, g.Name)
		seen := map[string]bool{}
		for i, m := range g.Members {
			key := strings.ToLower(m.User) // GitHub logins are case-insensitive
			if seen[key] {
				l.add(Diagnostic{File: doc.file, Line: doc.lines[fmt.Sprintf("/members/%d/user", i)],
					Message: fmt.Sprintf("member %q is listed more than once", m.User)})
			}
			seen[key] = true
		}
	}
	for _, c := range l.model.Components {
		doc := l.docs["Component/"+c.Name]
		l.checkPrefix(doc, c.Name)
		if !l.groupFiles[c.OwnerGroup()] {
			l.add(Diagnostic{File: doc.file, Line: doc.lines["/owner"],
				Message: fmt.Sprintf("owner group %q does not exist (no claims/groups/%s.yaml)", c.OwnerGroup(), c.OwnerGroup())})
		}
		if !l.platformOK {
			continue
		}
		for i, env := range c.Environments {
			if _, ok := l.model.Platform.Environments[env]; !ok {
				l.add(Diagnostic{File: doc.file, Line: doc.lines[fmt.Sprintf("/environments/%d", i)],
					Message: fmt.Sprintf("environment %q is not defined in config/platform.yaml", env)})
			}
		}
	}
}

// checkPrefix enforces config/platform.yaml naming (ADR-0014 isolation).
func (l *loader) checkPrefix(doc *document, name string) {
	n := l.model.Platform.Naming
	line := doc.lines["/name"]
	if n.RequiredPrefix != "" && !strings.HasPrefix(name, n.RequiredPrefix) {
		l.add(Diagnostic{File: doc.file, Line: line,
			Message: fmt.Sprintf("name %q must start with %q in this claims repo (config/platform.yaml naming.requiredPrefix)", name, n.RequiredPrefix)})
	}
	for _, p := range n.ReservedPrefixes {
		if strings.HasPrefix(name, p) {
			l.add(Diagnostic{File: doc.file, Line: line,
				Message: fmt.Sprintf("name %q uses the reserved prefix %q (config/platform.yaml naming.reservedPrefixes)", name, p)})
		}
	}
}
