// Package ghapp mints GitHub App installation tokens and refreshes them before
// they expire (they live one hour; a harness run can last longer).
package ghapp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// refreshBefore is the life a cached token must have left to be handed out:
// enough for the request that asked for it, and a retry, to finish.
const refreshBefore = 10 * time.Minute

// jwtHeader is the base64url header of every App JWT.
var jwtHeader = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))

// Source mints installation tokens of one App for one repository.
type Source struct {
	BaseURL     string            // API base, e.g. ghapi.DefaultBaseURL
	ClientID    string            // the App's client ID, the JWT issuer
	Key         *rsa.PrivateKey   // the App's private key
	Owner, Repo string            // the installation is looked up through this repository, and every token is scoped to it
	Permissions map[string]string // every token asks for exactly these
	Now         func() time.Time  // nil means time.Now

	mu      sync.Mutex
	token   string
	expires time.Time
}

// ParseKey reads a PEM private key (PKCS#1 "RSA PRIVATE KEY" or PKCS#8 "PRIVATE KEY").
// Its errors never repeat the key's text.
func ParseKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse the PKCS#1 key: %w", err)
		}
		return k, nil
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse the PKCS#8 key: %w", err)
		}
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("the PKCS#8 key is a %T, want an RSA key", k)
		}
		return rk, nil
	}
	return nil, fmt.Errorf("the PEM block is %q, want \"RSA PRIVATE KEY\" or \"PRIVATE KEY\"", block.Type)
}

// Token returns the cached token while it has more than ten minutes left, and
// mints a new one otherwise. It is safe for concurrent use: callers that find
// the token stale wait for one mint instead of each minting their own.
func (s *Source) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.token != "" && s.expires.Sub(now) > refreshBefore {
		return s.token, nil
	}
	token, expires, err := s.mint(ctx, now)
	if err != nil {
		return "", fmt.Errorf("mint a token for %s/%s: %w", s.Owner, s.Repo, err)
	}
	s.token, s.expires = token, expires
	return token, nil
}

func (s *Source) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// mint looks up the App's installation through the repository and creates a
// token for that repository and the permissions only. No error it returns
// carries the JWT or the token.
func (s *Source) mint(ctx context.Context, now time.Time) (string, time.Time, error) {
	jwt, err := s.jwt(now)
	if err != nil {
		return "", time.Time{}, err
	}
	app := ghapi.New(s.BaseURL, jwt)
	var inst struct {
		ID int64 `json:"id"`
	}
	if err := app.Get(ctx, "/repos/"+s.Owner+"/"+s.Repo+"/installation", &inst); err != nil {
		return "", time.Time{}, fmt.Errorf("look up the installation: %w", err)
	}
	if inst.ID == 0 {
		return "", time.Time{}, errors.New("look up the installation: the answer has no id")
	}
	var answer struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	body := map[string]any{"repositories": []string{s.Repo}, "permissions": s.Permissions}
	if err := app.Post(ctx, fmt.Sprintf("/app/installations/%d/access_tokens", inst.ID), body, &answer); err != nil {
		return "", time.Time{}, fmt.Errorf("create the installation token: %w", err)
	}
	if answer.Token == "" {
		return "", time.Time{}, errors.New("create the installation token: the answer has no token")
	}
	expires, err := time.Parse(time.RFC3339, answer.ExpiresAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create the installation token: expires_at: %w", err)
	}
	return answer.Token, expires, nil
}

// jwt signs the App's JWT (RS256): issued a minute ago against clock drift,
// valid for nine minutes, within the ten GitHub allows.
func (s *Source) jwt(now time.Time) (string, error) {
	if s.Key == nil {
		return "", errors.New("no App private key")
	}
	claims, err := json.Marshal(struct {
		IssuedAt  int64  `json:"iat"`
		ExpiresAt int64  `json:"exp"`
		Issuer    string `json:"iss"`
	}{now.Add(-time.Minute).Unix(), now.Add(9 * time.Minute).Unix(), s.ClientID})
	if err != nil {
		return "", fmt.Errorf("encode the JWT claims: %w", err)
	}
	signed := jwtHeader + "." + base64.RawURLEncoding.EncodeToString(claims)
	sum := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.Key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("sign the JWT: %w", err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
