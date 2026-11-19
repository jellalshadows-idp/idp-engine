package awsstack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

// TestEnv: a run against floci is the account's, by its access key ID
// (ADR-0004), and never prompts.
func TestEnv(t *testing.T) {
	got := Env([]string{"PATH=/bin"}, "000000000001")
	want := []string{"PATH=/bin", "AWS_ACCESS_KEY_ID=000000000001", "AWS_SECRET_ACCESS_KEY=floci-does-not-check-it", "TF_IN_AUTOMATION=1", "TF_INPUT=0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Env = %v, want %v", got, want)
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
