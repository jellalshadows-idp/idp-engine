package awsstack

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
)

// fakeSecret is the secret access key floci receives: floci checks no
// signature, and its account is the 12-digit access key ID (ADR-0004).
const fakeSecret = "floci-does-not-check-it"

// Env is base plus what a tofu run against floci needs: the account, as the
// access key ID (ADR-0004), and no prompt.
func Env(base []string, account string) []string {
	return append(slices.Clip(base),
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
