package ghapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GraphQL runs one query against the GraphQL endpoint and decodes its "data"
// into out. A response that carries "errors" is an error, even with data.
func (c *Client) GraphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	var resp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := c.Post(ctx, "/graphql", map[string]any{"query": query, "variables": variables}, &resp); err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		msgs := make([]string, len(resp.Errors))
		for i, e := range resp.Errors {
			msgs[i] = e.Message
		}
		return fmt.Errorf("graphql: %s", strings.Join(msgs, "; "))
	}
	if out == nil || len(resp.Data) == 0 || string(resp.Data) == "null" {
		return nil
	}
	return json.Unmarshal(resp.Data, out)
}
