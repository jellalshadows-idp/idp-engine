package render

import (
	"strings"
	"testing"
)

func TestEncryptionConfig(t *testing.T) {
	got := EncryptionConfig("correct-horse-battery-staple")
	want := `key_provider "pbkdf2" "idp" {
  passphrase = "correct-horse-battery-staple"
}
method "aes_gcm" "idp" {
  keys = key_provider.pbkdf2.idp
}
state {
  method = method.aes_gcm.idp
}
plan {
  method = method.aes_gcm.idp
}
`
	if got != want {
		t.Errorf("EncryptionConfig =\n%s\nwant\n%s", got, want)
	}
}

func TestEncryptionConfigEscapesHCL(t *testing.T) {
	got := EncryptionConfig(`a"b\c${d}%{e}`)
	if !strings.Contains(got, `passphrase = "a\"b\\c$${d}%%{e}"`) {
		t.Errorf("passphrase line not escaped for HCL:\n%s", got)
	}
}
