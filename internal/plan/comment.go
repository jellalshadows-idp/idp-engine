package plan

import (
	"fmt"
	"sort"
	"strings"
)

const (
	maxRowsPerStack = 100
	// maxCommentBytes keeps the comment under GitHub's 65536-character limit,
	// with room for the fingerprint marker.
	maxCommentBytes = 60000
)

// Comment renders the sticky PR plan comment: a summary table, one collapsible
// change list per stack, a warning for destructive plans, and the fingerprint
// marker the reconcile gate trusts (ADR-0018). Change lists are dropped when
// the comment would exceed GitHub's size limit; the marker never is.
func Comment(plans []*Plan, headSHA, runURL string) (string, error) {
	marker, err := EncodeMarker(headSHA, FingerprintOf(plans...))
	if err != nil {
		return "", err
	}
	sorted := append([]*Plan{}, plans...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Stack < sorted[j].Stack })
	body := renderComment(sorted, headSHA, runURL, true)
	if len(body)+len(marker) > maxCommentBytes {
		body = renderComment(sorted, headSHA, runURL, false)
		if len(body)+len(marker) > maxCommentBytes {
			return "", fmt.Errorf("plan comment is %d bytes even without change lists, over the %d-byte limit: too many stacks or changes for one PR comment", len(body)+len(marker), maxCommentBytes)
		}
	}
	return body + marker + "\n", nil
}

// codeEscaper makes text safe inside <code> in a Markdown table cell: no HTML,
// no cell break, no code-span break, one line. Plan text can therefore never
// form a marker or break the comment's structure.
var codeEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "|", "&#124;", "`", "&#96;", "\r", " ", "\n", " ")

func code(s string) string { return "<code>" + codeEscaper.Replace(s) + "</code>" }

func renderComment(plans []*Plan, headSHA, runURL string, details bool) string {
	var b strings.Builder
	b.WriteString(CommentMarker + "\n## IDP plan\n\n")
	total, destructive := 0, false
	for _, p := range plans {
		c := p.Counts()
		total += c.Total()
		destructive = destructive || c.Replace+c.Delete > 0
	}
	if total == 0 {
		b.WriteString("No infrastructure changes.\n\n")
	} else {
		b.WriteString("| Stack | Create | Update | Replace | Delete |\n|---|---:|---:|---:|---:|\n")
		for _, p := range plans {
			c := p.Counts()
			fmt.Fprintf(&b, "| %s | %d | %d | %s | %s |\n", code(p.Stack), c.Create, c.Update, flagged(c.Replace), flagged(c.Delete))
		}
		b.WriteString("\n")
		if details {
			for _, p := range plans {
				writeDetails(&b, p)
			}
		} else {
			b.WriteString("Change lists are omitted: this plan is too large for one comment. The workflow run has the full plan.\n\n")
		}
		if destructive {
			b.WriteString("> [!WARNING]\n> This plan deletes or replaces resources. The reconcile run after merge will wait for approval in the `idp-approval` environment.\n\n")
		}
	}
	fmt.Fprintf(&b, "Commit `%s`", headSHA[:7])
	if runURL != "" {
		fmt.Fprintf(&b, " · [workflow run](%s)", runURL)
	}
	b.WriteString("\n")
	return b.String()
}

func writeDetails(b *strings.Builder, p *Plan) {
	if len(p.Changes) == 0 {
		return
	}
	c := p.Counts()
	fmt.Fprintf(b, "<details><summary>%s: +%d ~%d -%d ±%d</summary>\n\n", code(p.Stack), c.Create, c.Update, c.Delete, c.Replace)
	b.WriteString("| Action | Address |\n|---|---|\n")
	for i, ch := range p.Changes {
		if i == maxRowsPerStack {
			fmt.Fprintf(b, "| … | and %d more changes |\n", len(p.Changes)-maxRowsPerStack)
			break
		}
		action := string(ch.Action)
		if ch.Action.Destructive() {
			action = "⚠️ " + action
		}
		fmt.Fprintf(b, "| %s | %s |\n", action, code(ch.Address))
	}
	b.WriteString("\n</details>\n\n")
}

func flagged(n int) string {
	if n == 0 {
		return "0"
	}
	return fmt.Sprintf("%d ⚠️", n)
}
