package ghapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestListFollowsNextLinks(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("per_page"); got != "100" {
			t.Errorf("per_page = %q, want 100", got)
		}
		switch r.URL.Query().Get("page") {
		case "":
			w.Header().Set("Link", fmt.Sprintf(`<%s/items?per_page=100&page=2>; rel="next", <%s/items?per_page=100&page=3>; rel="last"`, srv.URL, srv.URL))
			fmt.Fprint(w, `[1,2]`)
		case "2":
			w.Header().Set("Link", fmt.Sprintf(`<%s/items?per_page=100&page=3>; rel="next"`, srv.URL))
			fmt.Fprint(w, `[3]`)
		default:
			fmt.Fprint(w, `[4]`)
		}
	}))
	defer srv.Close()
	got, err := List[int](context.Background(), New(srv.URL, "t"), "/items")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{1, 2, 3, 4}) {
		t.Errorf("items = %v", got)
	}
}

func TestListKeepsTheQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "state=open&per_page=100" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()
	got, err := List[int](context.Background(), New(srv.URL, ""), "/items?state=open")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("items = %#v, want empty and non-nil", got)
	}
}

func TestListRefusesNextLinksOffTheBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `<https://evil.example/items?page=2>; rel="next"`)
		fmt.Fprint(w, `[1]`)
	}))
	defer srv.Close()
	if _, err := List[int](context.Background(), New(srv.URL, "secret"), "/items"); err == nil || !strings.Contains(err.Error(), "not under") {
		t.Errorf("err = %v, want a refusal to follow the foreign link", err)
	}
}

func TestListBoundsThePageCount(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", fmt.Sprintf(`<%s/items?page=next>; rel="next"`, srv.URL))
		fmt.Fprint(w, `[1]`)
	}))
	defer srv.Close()
	if _, err := List[int](context.Background(), New(srv.URL, ""), "/items"); err == nil || !strings.Contains(err.Error(), "pages") {
		t.Errorf("err = %v, want a page-limit error", err)
	}
}
