// Package actions writes GitHub Actions workflow commands and environment
// files: step outputs, job environment, job summary and error annotations.
// Every writer is a no-op when its file path is empty (not running in Actions).
package actions

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

var (
	dataEscaper     = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	propertyEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

// ErrorAnnotation formats an ::error workflow command, escaped as the
// workflow-commands docs require. file, line and title are optional.
func ErrorAnnotation(file string, line int, title, message string) string {
	var props []string
	if file != "" {
		props = append(props, "file="+propertyEscaper.Replace(file))
	}
	if line > 0 {
		props = append(props, fmt.Sprintf("line=%d", line))
	}
	if title != "" {
		props = append(props, "title="+propertyEscaper.Replace(title))
	}
	cmd := "::error"
	if len(props) > 0 {
		cmd += " " + strings.Join(props, ",")
	}
	return cmd + "::" + dataEscaper.Replace(message)
}

// SetOutput appends a step output to the GITHUB_OUTPUT file at path.
func SetOutput(path, name, value string) error { return appendEntry(path, name, value) }

// SetEnv appends a variable for later steps to the GITHUB_ENV file at path.
func SetEnv(path, name, value string) error { return appendEntry(path, name, value) }

// AddSummary appends Markdown to the GITHUB_STEP_SUMMARY file at path.
func AddSummary(path, markdown string) error {
	if path == "" {
		return nil
	}
	return appendFile(path, markdown+"\n")
}

func appendEntry(path, name, value string) error {
	if path == "" {
		return nil
	}
	if name == "" || strings.ContainsAny(name, "=\r\n") {
		return fmt.Errorf("invalid name %q", name)
	}
	if !strings.ContainsAny(value, "\r\n") {
		return appendFile(path, name+"="+value+"\n")
	}
	delim, err := delimiter(value)
	if err != nil {
		return err
	}
	return appendFile(path, name+"<<"+delim+"\n"+value+"\n"+delim+"\n")
}

// delimiter returns a random heredoc delimiter that does not occur in value,
// so a value can never end the entry early.
func delimiter(value string) (string, error) {
	for {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		if d := "IDP_EOF_" + hex.EncodeToString(b); !strings.Contains(value, d) {
			return d, nil
		}
	}
}

func appendFile(path, s string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(s); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
