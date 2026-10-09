package bootstrap

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"golang.org/x/crypto/nacl/box"
)

func TestSealRoundTrip(t *testing.T) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := Seal(base64.StdEncoding.EncodeToString(pub[:]), "s3cret-value")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		t.Fatal(err)
	}
	opened, ok := box.OpenAnonymous(nil, raw, pub, priv)
	if !ok || string(opened) != "s3cret-value" {
		t.Fatalf("OpenAnonymous = %q, %v", opened, ok)
	}
}

func TestSealRejectsBadKeys(t *testing.T) {
	for _, key := range []string{"not base64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := Seal(key, "x"); err == nil {
			t.Errorf("Seal(%q) succeeded, want error", key)
		}
	}
}
