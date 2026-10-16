package ghapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGraphQLDecodesData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if r.Method != http.MethodPost || r.URL.Path != "/graphql" {
			t.Errorf("%s %s, want POST /graphql", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Variables["id"] != "C1" || !strings.Contains(in.Query, "node(") {
			t.Errorf("body %+v, %v", in, err)
		}
		_, _ = w.Write([]byte(`{"data":{"node":{"editor":null}}}`))
	}))
	defer srv.Close()
	var out struct {
		Node struct {
			Editor *struct{ Login string } `json:"editor"`
		} `json:"node"`
	}
	if err := New(srv.URL, "t").GraphQL(context.Background(), `query($id: ID!) { node(id: $id) { id } }`, map[string]any{"id": "C1"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Node.Editor != nil {
		t.Errorf("editor = %+v, want nil", out.Node.Editor)
	}
}

func TestGraphQLReportsErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":null,"errors":[{"message":"Could not resolve to a node"},{"message":"second"}]}`))
	}))
	defer srv.Close()
	err := New(srv.URL, "t").GraphQL(context.Background(), "query { x }", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "Could not resolve to a node; second") {
		t.Errorf("err = %v", err)
	}
}
