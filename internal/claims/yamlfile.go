package claims

import (
	"bytes"
	"errors"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// document is one parsed claim file: its top-level mapping and an index from
// JSON pointers (RFC 6901) to the line where each value is declared.
type document struct {
	file  string
	root  *yaml.Node
	lines map[string]int
}

var yamlErrorLine = regexp.MustCompile(`line (\d+)`)

// parseFile parses exactly one YAML document whose root is a mapping. Anything
// else (empty, several documents, a list, invalid YAML) is a diagnostic.
func parseFile(file string, data []byte) (*document, []Diagnostic) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, []Diagnostic{{File: file, Message: "file is empty"}}
		}
		return nil, []Diagnostic{parseDiagnostic(file, err)}
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, []Diagnostic{parseDiagnostic(file, err)}
		}
		line := extra.Line
		if line == 0 {
			line = 1
		}
		return nil, []Diagnostic{{File: file, Line: line, Message: "only one YAML document per file is allowed"}}
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		line := doc.Line
		if line == 0 {
			line = 1
		}
		return nil, []Diagnostic{{File: file, Line: line, Message: "the document must be a mapping (key: value pairs)"}}
	}
	root := doc.Content[0]
	if ds := normalize(file, root); len(ds) > 0 {
		return nil, ds
	}
	return &document{file: file, root: root, lines: lineIndex(root)}, nil
}

// normalize makes the YAML tree fit the JSON value model the schemas validate.
// Claims have no timestamp type, so date-like scalars are retagged as strings
// and decode as the exact text written (not rewritten to RFC 3339). JSON has
// neither non-string object keys nor non-finite numbers, so those are reported
// at their line instead of surfacing later as a marshalling error. Aliases are
// not followed, consistent with lineIndex.
func normalize(file string, root *yaml.Node) []Diagnostic {
	var out []Diagnostic
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		switch n.Kind {
		case yaml.ScalarNode:
			switch n.ShortTag() {
			case "!!timestamp":
				n.Tag = "!!str"
			case "!!float":
				var f float64
				if err := n.Decode(&f); err == nil && (math.IsInf(f, 0) || math.IsNaN(f)) {
					out = append(out, Diagnostic{File: file, Line: n.Line, Message: "numbers must be finite (.inf and .nan are not supported)"})
				}
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				key, val := n.Content[i], n.Content[i+1]
				walk(key)
				if key.Kind != yaml.ScalarNode || (key.ShortTag() != "!!str" && key.ShortTag() != "!!merge") {
					out = append(out, Diagnostic{File: file, Line: key.Line, Message: "mapping keys must be strings"})
				}
				walk(val)
			}
		case yaml.SequenceNode:
			for _, c := range n.Content {
				walk(c)
			}
		}
	}
	walk(root)
	return out
}

func parseDiagnostic(file string, err error) Diagnostic {
	d := Diagnostic{File: file, Message: "invalid YAML: " + strings.TrimPrefix(err.Error(), "yaml: ")}
	if m := yamlErrorLine.FindStringSubmatch(err.Error()); m != nil {
		d.Line, _ = strconv.Atoi(m[1])
	}
	return d
}

// lineIndex maps JSON pointers into the tree to lines. Object members point at
// their key's line, which is where an editor should put the cursor.
//
// Aliases are deliberately not expanded: following them risks infinite
// recursion on self-referential anchors and exponential fan-out. An alias is
// indexed only at its use site, so errors inside aliased content resolve to the
// alias's own line. The decoder rejects self-referential anchors later.
func lineIndex(root *yaml.Node) map[string]int {
	idx := map[string]int{"": root.Line}
	var walk func(n *yaml.Node, ptr string)
	walk = func(n *yaml.Node, ptr string) {
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				key, val := n.Content[i], n.Content[i+1]
				child := ptr + "/" + escapePointer(key.Value)
				idx[child] = key.Line
				walk(val, child)
			}
		case yaml.SequenceNode:
			for i, item := range n.Content {
				child := ptr + "/" + strconv.Itoa(i)
				idx[child] = item.Line
				walk(item, child)
			}
		}
	}
	walk(root, "")
	return idx
}

var pointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

func escapePointer(token string) string { return pointerEscaper.Replace(token) }

// pointer builds an RFC 6901 JSON pointer from unescaped tokens.
func pointer(tokens []string) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString("/")
		b.WriteString(escapePointer(t))
	}
	return b.String()
}

// line returns the line of the deepest indexed ancestor of the location, so a
// missing property points at the object that should contain it.
func (d *document) line(tokens []string) int {
	for n := len(tokens); n >= 0; n-- {
		if l, ok := d.lines[pointer(tokens[:n])]; ok {
			return l
		}
	}
	return d.root.Line
}
