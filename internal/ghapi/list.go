package ghapi

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// maxPages bounds List, so a misbehaving server cannot loop it forever.
const maxPages = 100

var linkNext = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// List GETs every page of a list endpoint, following GitHub's Link
// rel="next" headers, and returns all items. path must not set per_page.
func List[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	next := path + sep + "per_page=100"
	all := []T{}
	for pages := 0; next != ""; pages++ {
		if pages == maxPages {
			return nil, fmt.Errorf("list %s: more than %d pages", path, maxPages)
		}
		var items []T
		hdr, err := c.do(ctx, http.MethodGet, next, nil, &items)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if next, err = c.nextPage(hdr.Get("Link")); err != nil {
			return nil, fmt.Errorf("list %s: %w", path, err)
		}
	}
	return all, nil
}

// nextPage returns the path of the rel="next" link, or "". The link must point
// under this client's base URL: the token is never sent anywhere else.
func (c *Client) nextPage(link string) (string, error) {
	m := linkNext.FindStringSubmatch(link)
	if m == nil {
		return "", nil
	}
	p, ok := strings.CutPrefix(m[1], c.baseURL)
	if !ok || !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("next page %q is not under %s", m[1], c.baseURL)
	}
	return p, nil
}
