// Package render turns a validated claims model into OpenTofu stacks (spec §5).
// Render is pure: the same model and options always produce the same bytes.
package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/jellalshadows-idp/idp-engine/internal/claims"
	"github.com/jellalshadows-idp/idp-engine/internal/naming"
	"github.com/jellalshadows-idp/idp-engine/internal/version"
	"github.com/jellalshadows-idp/idp-engine/lockfiles"
)

// Pinned versions the rendered stacks require (spec §7.5). AWSProviderVersion
// equals the pin in modules/aws/*/versions.tf and the AWS lock file.
const (
	TofuVersion           = "1.13.2"
	GitHubProviderVersion = "6.13.0"
	AWSProviderVersion    = "6.71.0"
)

// stateFile is the GitHub stack's local backend path, relative to rendered/github
// in a checkout of the wet branch (spec §5.4).
const stateFile = "../../tfstate/github.tfstate"

// awsStateFile is every AWS stack's local backend path: in the stack's own
// directory, in the job's working copy, and never committed to wet. AWS state
// lives as long as the emulator it describes (spec §5.5, ADR-0030).
const awsStateFile = "terraform.tfstate"

// Options decides where module sources point.
type Options struct {
	// ModuleRef is the idp-engine git ref module sources pin: the renderer's own
	// release tag (spec §5.3), or a commit for pre-release testing.
	ModuleRef string
	// ModulesDir replaces git sources with local paths, relative to the github stack
	// directory and starting with ./ or ../. CI uses it to validate a render before
	// its ref exists. The AWS stacks, deeper in the tree, get the same directory
	// expressed from their own.
	ModulesDir string
}

// Render returns the rendered files, keyed by slash-separated path relative to the
// output root: the github stack, and the AWS stacks of spec §5.4. It fails, rather
// than render less, on a model that Load rejects in a way the render depends on,
// such as a registry Component in an environment without an aws block.
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
	files := map[string][]byte{
		"github/main.tf.json":        stack,
		"github/.terraform.lock.hcl": lockfiles.GitHub,
	}
	stacks, err := awsStacks(m, opts)
	if err != nil {
		return nil, err
	}
	for dir, doc := range stacks {
		b, err := marshal(doc)
		if err != nil {
			return nil, err
		}
		files[dir+"/main.tf.json"] = b
		files[dir+"/.terraform.lock.hcl"] = lockfiles.AWS
	}
	return files, nil
}

// awsStacks are the AWS stacks of spec §5.4, keyed by directory: the baseline of
// every environment with an aws block, which creates the account's GitHub OIDC
// provider whether or not a Component uses it, and the stack of each Component
// with aws.registry in each environment it lists. An environment without an aws
// block has no AWS stack.
func awsStacks(m *claims.Model, opts Options) (map[string]map[string]any, error) {
	p := m.Platform
	stacks := map[string]map[string]any{}
	for _, env := range slices.Sorted(maps.Keys(p.Environments)) {
		aws := p.Environments[env].AWS
		if aws == nil {
			continue
		}
		dir := "aws/" + env + "/_baseline"
		stacks[dir] = awsStack(aws, "baseline", map[string]any{
			"source": moduleSource(opts, dir, "aws/baseline"),
		})
	}
	for _, c := range m.Components {
		if !c.AWS.Registry {
			continue
		}
		for _, env := range c.Environments {
			aws := p.Environments[env].AWS
			if aws == nil {
				// Load reports this (spec §4.7), and githubStack fails on it first.
				return nil, fmt.Errorf("render: component %q has aws.registry, but environment %q has no aws block", c.Name, env)
			}
			dir := "aws/" + env + "/components/" + c.Name
			// The inputs come from internal/naming, as the github stack's
			// variables do (spec §5.5), so both stacks name the same role and
			// repository.
			stacks[dir] = awsStack(aws, "component", map[string]any{
				"source":            moduleSource(opts, dir, "aws/component"),
				"github_repository": literal(p.GitHub.Org + "/" + c.Name),
				"environment":       literal(env),
				"repository_name":   literal(naming.RepositoryName(c.Name)),
				"role_name":         literal(naming.RoleName(c.Name, env)),
				"oidc_provider_arn": literal(naming.OIDCProviderARN(aws.AccountID)),
			})
		}
	}
	return stacks, nil
}

// awsStack is one thin AWS stack (spec §5.3): the pinned versions, a local
// backend with no encryption block (ADR-0030), the environment's region and one
// call of the engine's module. The emulator's endpoint and provider flags are
// not here: CI adds them in floci_override.tf.json (spec §5.6).
func awsStack(aws *claims.AWS, name string, call map[string]any) map[string]any {
	return map[string]any{
		"terraform": map[string]any{
			"required_version": TofuVersion,
			"required_providers": map[string]any{
				"aws": map[string]any{"source": "hashicorp/aws", "version": AWSProviderVersion},
			},
			"backend": map[string]any{"local": map[string]any{"path": awsStateFile}},
		},
		"provider": map[string]any{"aws": map[string]any{"region": literal(aws.Region)}},
		"module":   map[string]any{name: call},
	}
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
			"source":      moduleSource(opts, "github", "github/group"),
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
			"source":             moduleSource(opts, "github", "github/component"),
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

// moduleSource is where the stack in stackDir, a slash path relative to the
// output root, finds the engine's module <provider>/<name>: the engine's git
// repository at ModuleRef, or ModulesDir. ModulesDir is relative to the github
// stack, so a deeper stack climbs to the output root and descends into github
// first. The github stack's sources are ModulesDir as given.
func moduleSource(opts Options, stackDir, module string) string {
	if opts.ModulesDir == "" {
		return fmt.Sprintf("git::https://%s.git//modules/%s?ref=%s", version.Repo, module, opts.ModuleRef)
	}
	dir := strings.TrimSuffix(opts.ModulesDir, "/")
	if stackDir != "github" {
		dir = path.Join(strings.Repeat("../", strings.Count(stackDir, "/")+1)+"github", dir)
	}
	return dir + "/" + module
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
