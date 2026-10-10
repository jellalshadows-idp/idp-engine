// Package ghapi is a minimal GitHub REST client: JSON in, JSON out, typed errors.
package ghapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the public GitHub REST API.
const DefaultBaseURL = "https://api.github.com"

const apiVersion = "2022-11-28"

// maxBodyBytes bounds how much of a response is read (a variable so tests can shrink it).
var maxBodyBytes int64 = 10 << 20

// ErrNotFound is returned (wrapped) when the API answers 404.
var ErrNotFound = errors.New("not found")

// APIError is any non-2xx answer other than 404.
type APIError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github %s %s: %d %s", e.Method, e.Path, e.Status, e.Body)
}

// Client talks to the GitHub REST API. Build it with New.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New returns a client. An empty token sends unauthenticated requests.
func New(baseURL, token string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

// Get decodes the JSON answer of GET path into out (out may be nil).
func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.Do(ctx, http.MethodGet, path, nil, out)
}

// Post sends in as JSON (in may be nil) and decodes the answer into out (out may be nil).
func (c *Client) Post(ctx context.Context, path string, in, out any) error {
	return c.Do(ctx, http.MethodPost, path, in, out)
}

// Put sends in as JSON and decodes the answer into out (out may be nil).
func (c *Client) Put(ctx context.Context, path string, in, out any) error {
	return c.Do(ctx, http.MethodPut, path, in, out)
}

// Patch sends in as JSON and decodes the answer into out (out may be nil).
func (c *Client) Patch(ctx context.Context, path string, in, out any) error {
	return c.Do(ctx, http.MethodPatch, path, in, out)
}

// Delete sends DELETE path.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.Do(ctx, http.MethodDelete, path, nil, nil)
}

// Do performs one request. A 404 returns an error wrapping ErrNotFound;
// any other status >= 300 returns *APIError.
func (c *Client) Do(ctx context.Context, method, path string, in, out any) error {
	_, err := c.do(ctx, method, path, in, out)
	return err
}

// do is Do, and also returns the response headers.
func (c *Client) do(ctx context.Context, method, path string, in, out any) (http.Header, error) {
	var body io.Reader
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("encode %s %s: %w", method, path, err)
		}
		body = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s %s: %w", method, path, err)
	}
	oversized := int64(len(raw)) > maxBodyBytes
	if oversized {
		raw = raw[:maxBodyBytes] // keep the message bounded
	}
	// A 404 stays ErrNotFound even when its body is oversized: callers branch on it.
	if resp.StatusCode == http.StatusNotFound {
		if msg := strings.TrimSpace(string(raw)); msg != "" {
			return nil, fmt.Errorf("github %s %s: %w: %s", method, path, ErrNotFound, msg)
		}
		return nil, fmt.Errorf("github %s %s: %w", method, path, ErrNotFound)
	}
	if oversized {
		return nil, fmt.Errorf("read %s %s: response body exceeds %d bytes", method, path, maxBodyBytes)
	}
	if resp.StatusCode >= 300 {
		return nil, &APIError{Method: method, Path: path, Status: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
	}
	if out == nil || len(raw) == 0 {
		return resp.Header, nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return resp.Header, nil
}
