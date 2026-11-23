package claims

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"go.yaml.in/yaml/v4"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/jellalshadows-idp/idp-engine/schemas"
)

// schemaBaseURL identifies the embedded schemas inside the compiler. Nothing is
// fetched from it: every schema is added as an in-memory resource first.
const schemaBaseURL = "https://schemas.idp.invalid/"

// validator holds the compiled schema of every claim kind.
type validator struct {
	byKind  map[string]*jsonschema.Schema
	printer *message.Printer
}

func newValidator() (*validator, error) {
	c := jsonschema.NewCompiler()
	v := &validator{byKind: map[string]*jsonschema.Schema{}, printer: message.NewPrinter(language.English)}
	for _, k := range schemas.Kinds {
		raw, err := schemas.FS.ReadFile(schemas.File(k))
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w", k, err)
		}
		url := schemaBaseURL + schemas.File(k)
		if err := c.AddResource(url, doc); err != nil {
			return nil, fmt.Errorf("schema %s: %w", k, err)
		}
		sch, err := c.Compile(url)
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w", k, err)
		}
		v.byKind[k] = sch
	}
	return v, nil
}

// validate checks doc against the schema of kind and returns one diagnostic per
// failing leaf of the validation error tree, each located at its YAML line.
func (v *validator) validate(k string, doc *document) []Diagnostic {
	inst, err := jsonInstance(doc.root)
	if err != nil {
		line := doc.root.Line
		if l, _, _, ok := loadError(err); ok && l > 0 {
			line = l // the alias of a self-referential anchor, say
		}
		return []Diagnostic{{File: doc.file, Line: line, Message: err.Error()}}
	}
	err = v.byKind[k].Validate(inst)
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []Diagnostic{{File: doc.file, Line: doc.root.Line, Message: err.Error()}}
	}
	var out []Diagnostic
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		loc := e.InstanceLocation
		// An unknown property is reported on its parent object; point at the
		// offending key instead, which is where the fix goes.
		if ap, ok := e.ErrorKind.(*kind.AdditionalProperties); ok && len(ap.Properties) > 0 {
			loc = append(append([]string{}, loc...), ap.Properties[0])
		}
		out = append(out, Diagnostic{
			File:    doc.file,
			Line:    doc.line(loc),
			Message: fmt.Sprintf("%s: %s", locationLabel(e.InstanceLocation), e.ErrorKind.LocalizedString(v.printer)),
		})
	}
	walk(ve)
	return out
}

func locationLabel(tokens []string) string {
	if len(tokens) == 0 {
		return "(document)"
	}
	return pointer(tokens)
}

// jsonInstance converts a YAML mapping into the JSON value model the validator
// expects (json.Number numbers, string-keyed objects).
func jsonInstance(root *yaml.Node) (any, error) {
	var v any
	if err := root.Decode(&v); err != nil {
		return nil, fmt.Errorf("cannot read document: %w", wordAsV3(err))
	}
	raw, err := json.Marshal(oneZero(v))
	if err != nil {
		return nil, fmt.Errorf("unsupported YAML value: %w", err)
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

// oneZero turns every -0 in a decoded YAML value into 0, in place. The YAML
// library reads -0 as a negative-zero float, which encodes as "-0"; JSON has
// one zero, and claims never meant another.
func oneZero(v any) any {
	switch t := v.(type) {
	case float64:
		if t == 0 {
			return float64(0)
		}
	case map[string]any:
		for k, e := range t {
			t[k] = oneZero(e)
		}
	case []any:
		for i, e := range t {
			t[i] = oneZero(e)
		}
	}
	return v
}
