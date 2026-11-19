package wetdiff

import (
	"regexp"
	"strings"
)

// reconcilable matches the stacks this engine version plans and applies (spec
// §5.4): github, each environment's AWS baseline and each registry Component's
// AWS stack. Workspaces (aws/<env>/workspaces/<name>) come in Phase 3. The name
// patterns are the claims schemas' (schemas/platform.json, component.json).
var reconcilable = regexp.MustCompile(`^(?:github|aws/[a-z][a-z0-9]{0,9}/(?:_baseline|components/[a-z][a-z0-9-]{0,38}[a-z0-9]))$`)

// Reconcilable reports whether this engine version plans and applies stack.
func Reconcilable(stack string) bool { return reconcilable.MatchString(stack) }

// Escape names a stack where a slash is not allowed, such as an artifact or a
// file name: each slash becomes a dot. No reconcilable stack name holds a dot,
// so two stacks never share an escaped name.
func Escape(stack string) string { return strings.ReplaceAll(stack, "/", ".") }
