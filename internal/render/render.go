// Package render turns a validated claims model into OpenTofu stacks (spec §5).
// Render is pure: the same model and options always produce the same bytes.
package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/internal/version"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)

// Pinned versions the rendered stacks require (spec §7.5).
const (
	TofuVersion           = "1.13.1"
	GitHubProviderVersion = "6.13.0"
)

// stateFile is the GitHub stack's local backend path, relative to rendered/github
// in a checkout of the wet branch (spec §5.4).
const stateFile = "../../tfstate/github.tfstate"

// Options decides where module sources point.
type Options struct {
	// ModuleRef is the idp-engine git ref module sources pin: the renderer's own
	// release tag (spec §5.3), or a commit for pre-release testing.
	ModuleRef string
	// ModulesDir replaces git sources with local paths, relative to the github stack
	// directory and starting with ./ or ../. CI uses it to validate a render before
	// its ref exists.
	ModulesDir string
}

// Render returns the rendered files, keyed by slash-separated path relative to the
// output root. It fails, rather than render less, on a model that Load rejects in
// a way the render depends on, such as a registry Component in an environment
// without an aws block.
func Render(m *claims.Model, opts Options) (map[string][]byte, error) {
	if opts.ModuleRef == "" && opts.ModulesDir == "" {
		return nil, errors.New("render: set ModuleRef or ModulesDir")
	}
	doc, err := githubStack(m, opts)
	if err != nil {
		return nil, err
	}
	stack, err := marshal(doc)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		"github/main.tf.json":        stack,
		"github/.terraform.lock.hcl": lockfiles.GitHub,
	}, nil
}

func githubStack(m *claims.Model, opts Options) (map[string]any, error) {
	p := m.Platform
	modules := map[string]any{}
	for _, g := range m.Groups {
		members := make([]map[string]any, 0, len(g.Members))
		for _, mem := range g.Members {
			members = append(members, map[string]any{"user": literal(mem.User), "role": literal(mem.Role)})
		}
		modules["group_"+g.Name] = map[string]any{
			"source":      moduleSource(opts, "group"),
			"name":        literal(g.Name),
			"description": literal(g.Description),
			"members":     members,
		}
	}
	for _, c := range m.Components {
		envs := map[string]any{}
		for _, e := range c.Environments {
			env := map[string]any{"protected": p.Environments[e].Protected}
			if c.AWS.Registry {
				vars, err := envVariables(c.Name, e, p.Environments[e].AWS)
				if err != nil {
					return nil, err
				}
				env["variables"] = vars
			}
			envs[e] = env
		}
		topics := make([]string, 0, len(c.GitHub.Topics))
		for _, t := range c.GitHub.Topics {
			topics = append(topics, literal(t))
		}
		modules["component_"+c.Name] = map[string]any{
			"source":             moduleSource(opts, "component"),
			"name":               literal(c.Name),
			"description":        literal(c.Description),
			"topics":             topics,
			"owner_team_id":      fmt.Sprintf("${module.group_%s.team_id}", c.OwnerGroup()),
			"environments":       envs,
			"required_approvals": p.RequiredApprovals(),
			"archive_on_destroy": p.ArchiveOnDestroy(),
			"writer_app_id":      p.GitHub.WriterAppID,
		}
	}
	doc := map[string]any{
		"terraform": map[string]any{
			"required_version": TofuVersion,
			"required_providers": map[string]any{
				"github": map[string]any{"source": "integrations/github", "version": GitHubProviderVersion},
			},
			"backend": map[string]any{"local": map[string]any{"path": stateFile}},
			// The code says THAT state and plans are encrypted; TF_ENCRYPTION says HOW (spec §7.2).
			"encryption": map[string]any{
				"state": map[string]any{"enforced": true},
				"plan":  map[string]any{"enforced": true},
			},
		},
		"provider": map[string]any{"github": map[string]any{"owner": p.GitHub.Org}},
	}
	if len(modules) > 0 {
		doc["module"] = modules
	}
	return doc, nil
}

// envVariables are the GitHub environment variables of a registry Component in
// one environment (spec §4.4). internal/naming computes them from the names and
// the environment's aws block, never from AWS state (spec §5.5).
func envVariables(component, env string, aws *claims.AWS) (map[string]string, error) {
	if aws == nil {
		// Load reports this (spec §4.7). Rendering the Component without its
		// variables would plan their deletion, so fail instead.
		return nil, fmt.Errorf("render: component %q has aws.registry, but environment %q has no aws block", component, env)
	}
	return map[string]string{
		"AWS_ROLE_ARN":   literal(naming.RoleARN(aws.AccountID, component, env)),
		"ECR_REPOSITORY": literal(naming.RegistryURL(aws.AccountID, aws.Region, component)),
		"AWS_REGION":     literal(aws.Region),
	}, nil
}

// literal escapes s so OpenTofu renders it verbatim. OpenTofu evaluates ${ and %{
// inside JSON strings as template syntax, and claim text is data, never code.
func literal(s string) string {
	return strings.NewReplacer("${", "$${", "%{", "%%{").Replace(s)
}

func moduleSource(opts Options, module string) string {
	if opts.ModulesDir != "" {
		return strings.TrimSuffix(opts.ModulesDir, "/") + "/github/" + module
	}
	return fmt.Sprintf("git::https://%s.git//modules/github/%s?ref=%s", version.Repo, module, opts.ModuleRef)
}

// marshal encodes with sorted keys (encoding/json sorts map keys), two-space
// indentation, a trailing newline and no HTML escaping, so text stays literal.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
