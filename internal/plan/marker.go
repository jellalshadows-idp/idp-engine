package plan

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
)

// CommentMarker identifies the sticky plan comment on a pull request.
const CommentMarker = "<!-- idp-plan -->"

var (
	shaRE    = regexp.MustCompile(`^[0-9a-f]{40}$`)
	markerRE = regexp.MustCompile(`<!-- idp-fingerprint:v1 sha=([0-9a-f]{40}) data=([A-Za-z0-9+/]+={0,2}) -->`)
)

// maxFingerprintBytes bounds the decompressed marker, so a hostile comment
// cannot make the gate inflate a gzip bomb.
const maxFingerprintBytes = 8 << 20

// ValidSHA reports whether s is a full lowercase commit SHA.
func ValidSHA(s string) bool { return shaRE.MatchString(s) }

// EncodeMarker returns the hidden fingerprint line of a plan comment:
// base64 of gzipped JSON, tagged with the head commit it was planned for.
func EncodeMarker(headSHA string, f Fingerprint) (string, error) {
	if !ValidSHA(headSHA) {
		return "", fmt.Errorf("head sha %q: want 40 lowercase hex characters", headSHA)
	}
	raw, err := json.Marshal(f)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	return fmt.Sprintf("<!-- idp-fingerprint:v1 sha=%s data=%s -->", headSHA, base64.StdEncoding.EncodeToString(buf.Bytes())), nil
}

// DecodeMarker extracts the fingerprint marker from a comment body. ok is
// false when there is no marker; err reports a marker that is present but
// unreadable. The last marker wins: the comment ends with the real one, and
// nothing rendered before it can shadow it.
func DecodeMarker(body string) (headSHA string, f Fingerprint, ok bool, err error) {
	all := markerRE.FindAllStringSubmatch(body, -1)
	if len(all) == 0 {
		return "", nil, false, nil
	}
	m := all[len(all)-1]
	data, err := base64.StdEncoding.DecodeString(m[2])
	if err != nil {
		return "", nil, true, fmt.Errorf("fingerprint marker: %w", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", nil, true, fmt.Errorf("fingerprint marker: %w", err)
	}
	raw, err := io.ReadAll(io.LimitReader(zr, maxFingerprintBytes+1))
	if err != nil {
		return "", nil, true, fmt.Errorf("fingerprint marker: %w", err)
	}
	if len(raw) > maxFingerprintBytes {
		return "", nil, true, fmt.Errorf("fingerprint marker: decompressed size exceeds %d bytes", maxFingerprintBytes)
	}
	f, err = ReadFingerprint(bytes.NewReader(raw))
	if err != nil {
		return "", nil, true, err
	}
	return m[1], f, true, nil
}
