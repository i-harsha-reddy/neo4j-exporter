package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// expandSecrets replaces ${VAR} with os.Getenv("VAR") and ${file:/path}
// with the contents of the file (trimmed of trailing whitespace) inside
// any string scalar of the YAML document.
//
// We operate on the raw YAML text rather than per-field tags so the same
// rule applies uniformly to every string scalar without per-struct plumbing.
// The substitution is intentionally limited to ASCII identifiers (env)
// and absolute file paths (file:) to keep the surface tight.
var (
	envRE  = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)
	fileRE = regexp.MustCompile(`\$\{file:([^}]+)\}`)
)

func expandSecrets(s string) (string, error) {
	out := envRE.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-1]
		return os.Getenv(name)
	})
	var firstErr error
	out = fileRE.ReplaceAllStringFunc(out, func(match string) string {
		path := match[len("${file:") : len(match)-1]
		raw, err := os.ReadFile(path)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("read %s: %w", path, err)
			}
			return match
		}
		return strings.TrimRight(string(raw), "\r\n\t ")
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

// ResolveBoltPassword returns the effective password for a Bolt connection,
// reading from password_file if needed. Empty string + no error means
// auth: none was set explicitly.
func ResolveBoltPassword(b BoltConfig) (string, error) {
	if b.Auth == "none" {
		return "", nil
	}
	if b.Password != "" {
		return b.Password, nil
	}
	if b.PasswordFile != "" {
		raw, err := os.ReadFile(b.PasswordFile)
		if err != nil {
			return "", fmt.Errorf("read password_file %q: %w", b.PasswordFile, err)
		}
		return strings.TrimRight(string(raw), "\r\n\t "), nil
	}
	return "", fmt.Errorf("no password configured")
}

// ResolveJolokiaPassword returns the effective Jolokia password.
func ResolveJolokiaPassword(j *JolokiaAuth) (string, error) {
	if j == nil {
		return "", nil
	}
	if j.Password != "" {
		return j.Password, nil
	}
	if j.PasswordFile != "" {
		raw, err := os.ReadFile(j.PasswordFile)
		if err != nil {
			return "", fmt.Errorf("read jolokia password_file %q: %w", j.PasswordFile, err)
		}
		return strings.TrimRight(string(raw), "\r\n\t "), nil
	}
	return "", fmt.Errorf("no jolokia password configured")
}
