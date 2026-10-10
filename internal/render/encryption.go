package render

import (
	"fmt"
	"strings"
)

// KeyProviderName names the PBKDF2 key provider and the AES-GCM method in
// TF_ENCRYPTION. OpenTofu records it in the encrypted state, so renaming it
// makes existing state unreadable without a fallback migration (ADR-0020).
const KeyProviderName = "idp"

var hclEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "${", "$${", "%{", "%%{")

// EscapeHCLString escapes s for use inside an HCL quoted string, so the string
// is read back literally. EncryptionConfig uses it for the passphrase.
func EscapeHCLString(s string) string { return hclEscaper.Replace(s) }

// EncryptionConfig returns the TF_ENCRYPTION value for passphrase: the key
// material behind the enforced encryption the render emits (spec §7.2). The
// passphrase is escaped for an HCL quoted string, so it is used literally.
func EncryptionConfig(passphrase string) string {
	return fmt.Sprintf(`key_provider "pbkdf2" %[1]q {
  passphrase = "%[2]s"
}
method "aes_gcm" %[1]q {
  keys = key_provider.pbkdf2.%[1]s
}
state {
  method = method.aes_gcm.%[1]s
}
plan {
  method = method.aes_gcm.%[1]s
}
`, KeyProviderName, EscapeHCLString(passphrase))
}
