package awsstack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestMain turns the test binary into a stub tofu when IDP_AWSSTACK_FAKE_TOFU
// is set: it prints its arguments to stdout and "stderr" to stderr, and exits
// with the variable's value, so TestExec runs offline.
func TestMain(m *testing.M) {
	if code := os.Getenv("IDP_AWSSTACK_FAKE_TOFU"); code != "" {
		fmt.Println(strings.Join(os.Args[1:], " "))
		fmt.Fprintln(os.Stderr, "stderr")
		if code != "0" {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestExec: a show's stdout is the plan JSON, returned and not logged; any
// other run logs both streams; a failure names the command and the directory.
func TestExec(t *testing.T) {
	dir := t.TempDir()
	var log bytes.Buffer
	e := Exec{Path: os.Args[0], Env: append(os.Environ(), "IDP_AWSSTACK_FAKE_TOFU=0"), Log: &log}
	out, err := e.Tofu(dir, "show", "-json", "x.tfplan")
	if err != nil || string(out) != "show -json x.tfplan\n" || log.String() != "stderr\n" {
		t.Errorf("show: out %q, log %q, err %v", out, log.String(), err)
	}
	log.Reset()
	out, err = e.Tofu(dir, "init", "-input=false")
	if err != nil || len(out) != 0 || log.String() != "init -input=false\nstderr\n" {
		t.Errorf("init: out %q, log %q, err %v", out, log.String(), err)
	}
	e.Env = append(os.Environ(), "IDP_AWSSTACK_FAKE_TOFU=1")
	if _, err := e.Tofu(dir, "apply", "x.tfplan"); err == nil || err.Error() != "tofu apply x.tfplan in "+dir+": exit status 1" {
		t.Errorf("failure: err = %v", err)
	}
}

// TestEnv: OpenTofu gets only the passEnv variables of the job's environment,
// then the account, the fixed secret access key and no prompt. A token, a
// state key or a real AWS credential in the job never reaches it (R1-001).
func TestEnv(t *testing.T) {
	base := []string{
		"PATH=/bin", "GITHUB_TOKEN=ghs_must_not_reach_tofu", "HOME=/home/runner", "TMPDIR=/tmp",
		"ACTIONS_RUNTIME_TOKEN=runtime", "ACTIONS_ID_TOKEN_REQUEST_URL=https://token.example",
		"HTTPS_PROXY=http://proxy:3128", "no_proxy=localhost", "SSL_CERT_FILE=/etc/ssl/cert.pem",
		"TF_ENCRYPTION=key_provider", "TF_PLUGIN_CACHE_DIR=/cache", "TF_CLI_CONFIG_FILE=/x",
		"AWS_ACCESS_KEY_ID=AKIAREAL", "AWS_SECRET_ACCESS_KEY=real", "AWS_SESSION_TOKEN=real", "AWS_PROFILE=prod",
		"PATHEXT=.EXE", "HOMEDRIVE=C:", "malformed",
	}
	kept := slices.Clone(base)
	tail := []string{"AWS_ACCESS_KEY_ID=000000000001", "AWS_SECRET_ACCESS_KEY=floci-does-not-check-it", "TF_IN_AUTOMATION=1", "TF_INPUT=0"}
	want := append([]string{"PATH=/bin", "HOME=/home/runner", "TMPDIR=/tmp", "HTTPS_PROXY=http://proxy:3128", "no_proxy=localhost", "SSL_CERT_FILE=/etc/ssl/cert.pem"}, tail...)
	if got := Env(base, "000000000001"); !reflect.DeepEqual(got, want) {
		t.Errorf("Env =\n%v\nwant\n%v", got, want)
	}
	if !reflect.DeepEqual(base, kept) {
		t.Errorf("Env changed its base: %v", base)
	}
	if got := Env(nil, "000000000001"); !reflect.DeepEqual(got, tail) {
		t.Errorf("Env(nil) = %v, want %v", got, tail)
	}
}

// TestWriteOverride pins what CI adds to a rendered AWS stack for floci (spec
// §5.6): the endpoint and the provider flags that OpenTofu merges into the
// stack's provider block. The pipelines and the floci tests write it with this
// one function.
func TestWriteOverride(t *testing.T) {
	dir := t.TempDir()
	if err := WriteOverride(dir, "http://localhost:4566"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "floci_override.tf.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"provider": map[string]any{"aws": map[string]any{
		"endpoints":                   map[string]any{"ecr": "http://localhost:4566", "iam": "http://localhost:4566", "sts": "http://localhost:4566"},
		"skip_credentials_validation": true,
		"skip_metadata_api_check":     "true",
		"skip_requesting_account_id":  true,
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("floci_override.tf.json = %v, want %v", got, want)
	}
}
