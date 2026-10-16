package e2e

import (
	"fmt"
	"strings"
)

// Member is one Group membership.
type Member struct {
	User string
	Role string
}

// GroupClaim renders a Group claim.
func GroupClaim(name, description string, members []Member) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: idp/v1\nkind: Group\nname: %s\ndescription: %s\nmembers:\n", name, description)
	for _, m := range members {
		fmt.Fprintf(&b, "  - user: %s\n    role: %s\n", m.User, m.Role)
	}
	return []byte(b.String())
}

// ComponentClaim renders a Component claim in environment dev; no topics means
// no github block.
func ComponentClaim(name, description, group string, topics []string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: idp/v1\nkind: Component\nname: %s\ndescription: %s\nowner: group:%s\nenvironments: [dev]\n", name, description, group)
	if len(topics) > 0 {
		fmt.Fprintf(&b, "github:\n  topics: [%s]\n", strings.Join(topics, ", "))
	}
	return []byte(b.String())
}
