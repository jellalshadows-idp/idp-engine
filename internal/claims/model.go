package claims

import "strings"

// Model is a loaded claims repo. It is only meaningful when Load returned no
// diagnostics.
type Model struct {
	Platform   Platform
	Groups     []Group     // sorted by name
	Components []Component // sorted by name
}

// Platform is config/platform.yaml (spec §4.6, amendment A1).
type Platform struct {
	GitHub struct {
		Org               string `yaml:"org"`
		WriterAppID       int64  `yaml:"writerAppId"`
		ArchiveOnDestroy  *bool  `yaml:"archiveOnDestroy"`
		RequiredApprovals *int   `yaml:"requiredApprovals"`
	} `yaml:"github"`
	Naming struct {
		RequiredPrefix   string   `yaml:"requiredPrefix"`
		ReservedPrefixes []string `yaml:"reservedPrefixes"`
	} `yaml:"naming"`
	Environments map[string]Environment `yaml:"environments"`
}

// Environment is one entry of Platform.Environments. Its aws block is used from Phase 2.
type Environment struct {
	Protected bool `yaml:"protected"`
}

// ArchiveOnDestroy reports github.archiveOnDestroy, which defaults to true.
func (p Platform) ArchiveOnDestroy() bool {
	if p.GitHub.ArchiveOnDestroy == nil {
		return true
	}
	return *p.GitHub.ArchiveOnDestroy
}

// RequiredApprovals reports github.requiredApprovals, which defaults to 1.
func (p Platform) RequiredApprovals() int {
	if p.GitHub.RequiredApprovals == nil {
		return 1
	}
	return *p.GitHub.RequiredApprovals
}

// Group is a GitHub team and its members (spec §4.3).
type Group struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Members     []Member `yaml:"members"`
}

// Member is one team member.
type Member struct {
	User string `yaml:"user"`
	Role string `yaml:"role"`
}

// Component is a service's GitHub part (spec §4.4); aws and features arrive later.
type Component struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Owner        string   `yaml:"owner"`
	Environments []string `yaml:"environments"`
	GitHub       struct {
		Topics []string `yaml:"topics"`
	} `yaml:"github"`
}

// OwnerGroup is the Group name in Owner ("group:platform" → "platform").
func (c Component) OwnerGroup() string { return strings.TrimPrefix(c.Owner, "group:") }
