// Package metadata exposes shared direct-connection helpers for metadata-aware interface adapters.
// input: transport-layer connection inputs and secret lookup sources
// output: normalized validation and password resolution helpers for interface adapters
// pos: shared connection helper boundary used by MCP and future metadata-aware transports
// note: if this file changes, update this header and module README.md.
package metadata

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrorKind classifies shared direct-connection input failures.
type ErrorKind string

const (
	ErrorKindValidation     ErrorKind = "validation"
	ErrorKindPasswordSource ErrorKind = "password_source"
	ErrorKindPasswordLookup ErrorKind = "password_lookup"
)

// ConnectionInputError preserves the user-facing message while exposing adapter-safe classification.
type ConnectionInputError struct {
	Kind    ErrorKind
	Message string
	Err     error
}

func (e *ConnectionInputError) Error() string {
	return e.Message
}

func (e *ConnectionInputError) Unwrap() error {
	return e.Err
}

// IsConnectionInputError reports whether err is a shared direct-connection input failure.
func IsConnectionInputError(err error) bool {
	var inputErr *ConnectionInputError
	return errors.As(err, &inputErr)
}

// ConnectionInput describes one direct metadata-aware connection input.
type ConnectionInput struct {
	Host         string `json:"host,omitempty" yaml:"host"`
	Port         int    `json:"port,omitempty"`
	Socket       string `json:"socket,omitempty" yaml:"socket"`
	User         string `json:"user,omitempty" yaml:"user"`
	Schema       string `json:"schema,omitempty" yaml:"schema"`
	Dialect      string `json:"dialect,omitempty" yaml:"dialect"`
	Password     string `json:"password,omitempty" yaml:"password"`
	PasswordEnv  string `json:"password_env,omitempty" yaml:"password_env"`
	PasswordFile string `json:"password_file,omitempty" yaml:"password_file"`
}

// ResolveConnectionOptions configures shared connection helper dependencies.
type ResolveConnectionOptions struct {
	LookupEnv func(string) (string, bool)
	ReadFile  func(string) ([]byte, error)
}

// ValidateConnectionInput validates direct connection input shape and transport constraints.
func ValidateConnectionInput(input ConnectionInput) error {
	if strings.TrimSpace(input.Host) == "" &&
		input.Port == 0 &&
		strings.TrimSpace(input.Socket) == "" &&
		strings.TrimSpace(input.User) == "" &&
		strings.TrimSpace(input.Schema) == "" &&
		strings.TrimSpace(input.Dialect) == "" {
		return newConnectionInputError(ErrorKindValidation, "connection must include at least one non-password field", nil)
	}
	if strings.TrimSpace(input.Socket) == "" && (strings.TrimSpace(input.Host) == "" || strings.TrimSpace(input.User) == "") {
		return newConnectionInputError(ErrorKindValidation, "connection must include host/user, socket/user, or connection_ref", nil)
	}
	if strings.TrimSpace(input.Socket) != "" && strings.TrimSpace(input.User) == "" {
		return newConnectionInputError(ErrorKindValidation, "connection must include host/user, socket/user, or connection_ref", nil)
	}
	if strings.TrimSpace(input.Socket) != "" && (strings.TrimSpace(input.Host) != "" || input.Port != 0) {
		return newConnectionInputError(ErrorKindValidation, "connection socket cannot be combined with host/port TCP options", nil)
	}
	return nil
}

// ResolvePassword resolves a password from direct, env, or file sources.
func ResolvePassword(input ConnectionInput, options ResolveConnectionOptions) (string, error) {
	count := 0
	if strings.TrimSpace(input.Password) != "" {
		count++
	}
	if strings.TrimSpace(input.PasswordEnv) != "" {
		count++
	}
	if strings.TrimSpace(input.PasswordFile) != "" {
		count++
	}
	if count > 1 {
		return "", newConnectionInputError(ErrorKindPasswordSource, "connection password sources are mutually exclusive", nil)
	}

	switch {
	case strings.TrimSpace(input.Password) != "":
		return input.Password, nil
	case strings.TrimSpace(input.PasswordEnv) != "":
		lookup := options.LookupEnv
		if lookup == nil {
			lookup = os.LookupEnv
		}
		key := strings.TrimSpace(input.PasswordEnv)
		value, ok := lookup(key)
		if !ok {
			return "", newConnectionInputError(ErrorKindPasswordLookup, fmt.Sprintf("password env %q is not set", key), nil)
		}
		return value, nil
	case strings.TrimSpace(input.PasswordFile) != "":
		data, err := readFileWithHome(input.PasswordFile, options)
		if err != nil {
			return "", newConnectionInputError(ErrorKindPasswordLookup, fmt.Sprintf("read password file: %v", err), err)
		}
		return strings.TrimSpace(string(data)), nil
	default:
		return "", nil
	}
}

func newConnectionInputError(kind ErrorKind, message string, err error) error {
	return &ConnectionInputError{
		Kind:    kind,
		Message: message,
		Err:     err,
	}
}

// ExpandHome expands a leading ~ in path values.
func ExpandHome(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}

func readFileWithHome(path string, options ResolveConnectionOptions) ([]byte, error) {
	readFile := options.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	expanded, err := ExpandHome(path)
	if err != nil {
		return nil, err
	}
	data, err := readFile(expanded)
	if err != nil {
		return nil, err
	}
	return data, nil
}
