package ghapp

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// testKey is one App key for the whole package: generating it is slow.
var testKey = sync.OnceValue(func() *rsa.PrivateKey {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return k
})

var t0 = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

var testPermissions = map[string]string{"contents": "write", "pull_requests": "write"}

const (
	lookupPath = "/repos/acme/claims/installation"
	mintPath   = "/app/installations/42/access_tokens"
)

// fakeGitHub answers the installation lookup of acme/claims with id 42 and
// mints tokens ghs_1, ghs_2, … that expire an hour after its clock.
type fakeGitHub struct {
	mu       sync.Mutex
	now      time.Time
	lookup   int    // status of the lookup; 0 means 200
	answer   string // body of the access token answer; "" means a fresh token
	requests []string
	bearers  []string
	bodies   []string // bodies of the access token requests
	minted   int
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	f.bearers = append(f.bearers, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	switch {
	case r.Method == http.MethodGet && r.URL.Path == lookupPath:
		if f.lookup != 0 {
			w.WriteHeader(f.lookup)
			io.WriteString(w, `{"message":"A JSON web token could not be decoded"}`)
			return
		}
		io.WriteString(w, `{"id": 42}`)
	case r.Method == http.MethodPost && r.URL.Path == mintPath:
		f.bodies = append(f.bodies, string(body))
		w.WriteHeader(http.StatusCreated)
		if f.answer != "" {
			io.WriteString(w, f.answer)
			return
		}
		f.minted++
		fmt.Fprintf(w, `{"token":"ghs_%d","expires_at":%q}`, f.minted, f.now.Add(time.Hour).Format(time.RFC3339))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeGitHub) clock() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeGitHub) setNow(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t
}

func (f *fakeGitHub) seen() (requests, bearers, bodies []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.requests), slices.Clone(f.bearers), slices.Clone(f.bodies)
}

func newSource(t *testing.T, f *fakeGitHub) *Source {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return &Source{
		BaseURL: srv.URL, ClientID: "Iv23liTestClient", Key: testKey(),
		Owner: "acme", Repo: "claims", Permissions: testPermissions, Now: f.clock,
	}
}

func TestTokenAuthenticatesWithAnRS256JWT(t *testing.T) {
	f := &fakeGitHub{now: t0}
	if _, err := newSource(t, f).Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, bearers, _ := f.seen()
	if len(bearers) != 2 || bearers[0] == "" || bearers[1] != bearers[0] {
		t.Fatalf("got %d requests, want the lookup and the mint, both with the same JWT", len(bearers))
	}
	jwt := bearers[0]
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("the JWT has %d parts, want 3", len(parts))
	}
	if strings.ContainsAny(jwt, "=+/") {
		t.Error("the JWT is not base64url without padding")
	}
	decode := func(i int, out any) []byte {
		t.Helper()
		b, err := base64.RawURLEncoding.DecodeString(parts[i])
		if err != nil {
			t.Fatalf("JWT part %d: %v", i, err)
		}
		if out != nil {
			if err := json.Unmarshal(b, out); err != nil {
				t.Fatalf("JWT part %d: %v", i, err)
			}
		}
		return b
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	decode(0, &header)
	if header.Alg != "RS256" || header.Typ != "JWT" {
		t.Errorf("header = %+v, want alg RS256 and typ JWT", header)
	}
	var claims struct {
		IssuedAt  int64  `json:"iat"`
		ExpiresAt int64  `json:"exp"`
		Issuer    string `json:"iss"`
	}
	decode(1, &claims)
	if claims.Issuer != "Iv23liTestClient" {
		t.Errorf("iss = %q, want the client ID", claims.Issuer)
	}
	if d := claims.ExpiresAt - claims.IssuedAt; d > 600 {
		t.Errorf("exp - iat = %ds, want at most 10 minutes: GitHub refuses longer JWTs", d)
	}
	if claims.IssuedAt != t0.Add(-time.Minute).Unix() || claims.ExpiresAt != t0.Add(9*time.Minute).Unix() {
		t.Errorf("iat, exp = %d, %d, want now-60s and now+9m (%d, %d)", claims.IssuedAt, claims.ExpiresAt, t0.Add(-time.Minute).Unix(), t0.Add(9*time.Minute).Unix())
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&testKey().PublicKey, crypto.SHA256, sum[:], decode(2, nil)); err != nil {
		t.Errorf("the signature does not verify with the App's public key: %v", err)
	}
}

func TestTokenIsScopedToTheRepoAndPermissions(t *testing.T) {
	f := &fakeGitHub{now: t0}
	tok, err := newSource(t, f).Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tok != "ghs_1" {
		t.Errorf("token = %q, want ghs_1", tok)
	}
	requests, _, bodies := f.seen()
	if want := []string{"GET " + lookupPath, "POST " + mintPath}; !slices.Equal(requests, want) {
		t.Fatalf("requests = %q, want %q", requests, want)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(bodies[0]), &body); err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(body)); !slices.Equal(got, []string{"permissions", "repositories"}) {
		t.Errorf("access token request keys = %q, want exactly permissions and repositories", got)
	}
	var repos []string
	if err := json.Unmarshal(body["repositories"], &repos); err != nil || !slices.Equal(repos, []string{"claims"}) {
		t.Errorf("repositories = %s, want [\"claims\"]", body["repositories"])
	}
	var perms map[string]string
	if err := json.Unmarshal(body["permissions"], &perms); err != nil || !maps.Equal(perms, testPermissions) {
		t.Errorf("permissions = %s, want %v", body["permissions"], testPermissions)
	}
}

func TestTokenIsCachedWhileMoreThanTenMinutesAreLeft(t *testing.T) {
	f := &fakeGitHub{now: t0}
	s := newSource(t, f)
	ctx := context.Background()
	first, err := s.Token(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []time.Duration{0, 49 * time.Minute, 49*time.Minute + 59*time.Second} {
		f.setNow(t0.Add(d))
		if got, err := s.Token(ctx); err != nil || got != first {
			t.Errorf("at +%v: Token = %q, %v, want the cached %q", d, got, err, first)
		}
	}
	if requests, _, _ := f.seen(); len(requests) != 2 {
		t.Errorf("requests = %q, want only the first mint's two", requests)
	}
}

func TestTokenIsRefreshedWithTenMinutesOrLessLeft(t *testing.T) {
	for _, d := range []time.Duration{50 * time.Minute, 51 * time.Minute} {
		t.Run(d.String(), func(t *testing.T) {
			f := &fakeGitHub{now: t0}
			s := newSource(t, f)
			ctx := context.Background()
			if _, err := s.Token(ctx); err != nil {
				t.Fatal(err)
			}
			f.setNow(t0.Add(d))
			if got, err := s.Token(ctx); err != nil || got != "ghs_2" {
				t.Errorf("Token = %q, %v, want a new token ghs_2", got, err)
			}
			if got, err := s.Token(ctx); err != nil || got != "ghs_2" {
				t.Errorf("Token = %q, %v, want the new token cached", got, err)
			}
			if requests, _, _ := f.seen(); len(requests) != 4 {
				t.Errorf("requests = %q, want two mints of two requests", requests)
			}
		})
	}
}

func TestTokenIsSafeForConcurrentUse(t *testing.T) {
	f := &fakeGitHub{now: t0}
	s := newSource(t, f)
	tokens := make([]string, 8)
	errs := make([]error, 8)
	var wg sync.WaitGroup
	for i := range tokens {
		wg.Go(func() { tokens[i], errs[i] = s.Token(context.Background()) })
	}
	wg.Wait()
	for i := range tokens {
		if errs[i] != nil || tokens[i] != "ghs_1" {
			t.Errorf("caller %d: Token = %q, %v, want ghs_1", i, tokens[i], errs[i])
		}
	}
	if requests, _, _ := f.seen(); len(requests) != 2 {
		t.Errorf("requests = %q, want one mint shared by every caller", requests)
	}
}

func TestLookupFailureHidesTheJWT(t *testing.T) {
	f := &fakeGitHub{now: t0, lookup: http.StatusUnauthorized}
	_, err := newSource(t, f).Token(context.Background())
	if err == nil {
		t.Fatal("Token succeeded after a 401 on the lookup")
	}
	requests, bearers, _ := f.seen()
	if len(bearers) == 0 || bearers[0] == "" {
		t.Fatal("the lookup carried no JWT")
	}
	if strings.Contains(err.Error(), bearers[0]) {
		t.Error("the error contains the JWT")
	}
	if !strings.HasPrefix(err.Error(), "mint a token for acme/claims: ") || !strings.Contains(err.Error(), "401") {
		t.Errorf("err = %q, want it to name acme/claims and the 401", err)
	}
	if want := []string{"GET " + lookupPath}; !slices.Equal(requests, want) {
		t.Errorf("requests = %q, want only the lookup", requests)
	}
}

func TestTokenRefusesAnIncompleteAnswer(t *testing.T) {
	tests := []struct {
		name, answer, want string
	}{
		{"no token", `{"expires_at":"2026-10-15T13:00:00Z"}`, "no token"},
		{"bad expiry", `{"token":"ghs_secret","expires_at":"in an hour"}`, "expires_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeGitHub{now: t0, answer: tt.answer}
			_, err := newSource(t, f).Token(context.Background())
			if err == nil || !strings.HasPrefix(err.Error(), "mint a token for acme/claims: ") || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want a mint error about %s", err, tt.want)
			}
			if strings.Contains(err.Error(), "ghs_secret") {
				t.Error("the error contains the token")
			}
		})
	}
}

func TestParseKey(t *testing.T) {
	k := testKey()
	pkcs8, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	for name, in := range map[string][]byte{
		"PKCS#1": pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}),
		"PKCS#8": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}),
	} {
		if got, err := ParseKey(in); err != nil || !got.Equal(k) {
			t.Errorf("%s: ParseKey = %v, want the key back", name, err)
		}
	}

	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecPKCS8, err := x509.MarshalPKCS8PrivateKey(ec)
	if err != nil {
		t.Fatal(err)
	}
	ecSEC1, err := x509.MarshalECPrivateKey(ec)
	if err != nil {
		t.Fatal(err)
	}
	for name, in := range map[string][]byte{
		"garbage":           []byte("this is not a key at all"),
		"corrupt PKCS#1":    pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("secret key material")}),
		"corrupt PKCS#8":    pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("secret key material")}),
		"ECDSA in PKCS#8":   pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecPKCS8}),
		"EC PRIVATE KEY":    pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecSEC1}),
		"public key, PKIX":  pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: mustPKIX(t, &k.PublicKey)}),
		"empty":             nil,
		"header and footer": []byte("-----BEGIN RSA PRIVATE KEY-----\n-----END RSA PRIVATE KEY-----\n"),
	} {
		_, err := ParseKey(in)
		if err == nil {
			t.Errorf("%s: ParseKey accepted it", name)
			continue
		}
		for _, line := range strings.Split(string(in), "\n") {
			if len(line) >= 8 && !strings.HasPrefix(line, "-----") && strings.Contains(err.Error(), line) {
				t.Errorf("%s: the error repeats the key's text", name)
			}
		}
	}
}

func mustPKIX(t *testing.T, pub any) []byte {
	t.Helper()
	b, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
