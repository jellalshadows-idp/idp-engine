package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/jellalshadows-idp/idp-engine/internal/ghapi"
)

// AppCredentials identify one GitHub App created by `idp bootstrap app`.
// The JSON file never contains the key; it lives in <slug>.pem next to it.
type AppCredentials struct {
	ID         int64  `json:"id"`
	ClientID   string `json:"client_id"`
	Slug       string `json:"slug"`
	PrivateKey string `json:"-"`
}

// Config is everything one org bootstrap needs.
type Config struct {
	Org        string
	ClaimsRepo string
	ApproverID int64 // GitHub user id of the platform admin who approves runs
	Reader     AppCredentials
	Writer     AppCredentials
	Passphrase string // OpenTofu state passphrase (apply only)
}

// ValidateCheck reports what `check` needs: identities, no key material.
func (c Config) ValidateCheck() error {
	return joinProblems(c.baseProblems())
}

// ValidateApply reports what `apply` needs: identities plus keys and passphrase.
func (c Config) ValidateApply() error {
	problems := c.baseProblems()
	if c.Reader.PrivateKey == "" {
		problems = append(problems, "reader private key is missing")
	}
	if c.Writer.PrivateKey == "" {
		problems = append(problems, "writer private key is missing")
	}
	if utf8.RuneCountInString(c.Passphrase) < 16 {
		problems = append(problems, "passphrase must be at least 16 characters")
	}
	return joinProblems(problems)
}

func (c Config) baseProblems() []string {
	var problems []string
	if c.Org == "" {
		problems = append(problems, "org is required")
	}
	if c.ClaimsRepo == "" {
		problems = append(problems, "claims repo is required")
	}
	if c.ApproverID <= 0 {
		problems = append(problems, "approver id must be positive")
	}
	for _, app := range []struct {
		role  string
		creds AppCredentials
	}{{"reader", c.Reader}, {"writer", c.Writer}} {
		if app.creds.ID <= 0 || app.creds.ClientID == "" || app.creds.Slug == "" {
			problems = append(problems, app.role+" app id, client id and slug are required")
		}
	}
	return problems
}

func joinProblems(problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; "))
}

// LoadAppCredentials reads the <slug>.json written by `idp bootstrap app` and,
// when withKey is set, the <slug>.pem next to it.
func LoadAppCredentials(jsonPath string, withKey bool) (AppCredentials, error) {
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		return AppCredentials{}, err
	}
	var c AppCredentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return AppCredentials{}, fmt.Errorf("%s: %w", jsonPath, err)
	}
	if withKey {
		pem, err := os.ReadFile(filepath.Join(filepath.Dir(jsonPath), c.Slug+".pem"))
		if err != nil {
			return AppCredentials{}, err
		}
		c.PrivateKey = string(pem)
	}
	return c, nil
}

// ReadPassphrase loads the state passphrase, trimming the line ending editors
// add (LF or CRLF), and enforces OpenTofu's PBKDF2 minimum of 16 characters.
func ReadPassphrase(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	p := strings.TrimRight(string(raw), "\r\n")
	if utf8.RuneCountInString(p) < 16 {
		return "", fmt.Errorf("passphrase in %s is shorter than 16 characters", path)
	}
	return p, nil
}

// UserID resolves a login to its numeric id; environment reviewers need ids.
func UserID(ctx context.Context, api *ghapi.Client, login string) (int64, error) {
	var u struct {
		ID int64 `json:"id"`
	}
	if err := api.Get(ctx, "/users/"+url.PathEscape(login), &u); err != nil {
		return 0, fmt.Errorf("resolve user %s: %w", login, err)
	}
	return u.ID, nil
}
