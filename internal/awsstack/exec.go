package awsstack

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// fakeSecret is the secret access key floci receives: floci checks no
// signature, and its account is the 12-digit access key ID (ADR-0004).
const fakeSecret = "floci-does-not-check-it"

// passEnv names the variables of the job's environment that a tofu run keeps:
// what OpenTofu, git (which fetches the module sources) and the AWS provider
// need on a Linux runner. Everything else stays out, a token, a state key or a
// real AWS credential included, so a secret added to a floci job by mistake
// never reaches OpenTofu (ADR-0031). The names are exact.
var passEnv = map[string]bool{
	"PATH": true, "HOME": true, "TMPDIR": true,
	"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true,
	"http_proxy": true, "https_proxy": true, "no_proxy": true,
	"SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
}

// Env is the environment of a tofu run against floci: the passEnv variables of
// base, in their order, then the account, as the access key ID (ADR-0004), and
// no prompt. It never changes base.
func Env(base []string, account string) []string {
	var env []string
	for _, kv := range base {
		if name, _, ok := strings.Cut(kv, "="); ok && passEnv[name] {
			env = append(env, kv)
		}
	}
	return append(env,
		"AWS_ACCESS_KEY_ID="+account,
		"AWS_SECRET_ACCESS_KEY="+fakeSecret,
		"TF_IN_AUTOMATION=1",
		"TF_INPUT=0",
	)
}

// Exec runs the tofu binary at Path with the environment Env. Its output goes
// to Log, except a show's stdout, which is the plan's JSON and is returned.
type Exec struct {
	Path string
	Env  []string
	Log  io.Writer
}

// Tofu runs tofu with args in dir.
func (e Exec) Tofu(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(e.Path, args...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, e.Env, e.Log, e.Log
	var out bytes.Buffer
	if len(args) > 0 && args[0] == "show" {
		cmd.Stdout = &out
	}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("tofu %s in %s: %w", strings.Join(args, " "), dir, err)
	}
	return out.Bytes(), nil
}
