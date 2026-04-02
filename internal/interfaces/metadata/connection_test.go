// Package metadata verifies shared direct-connection validation and secret resolution behavior.
// input: connection inputs and secret lookup sources used by transport adapters
// output: regression coverage for shared metadata connection helpers
// pos: shared interface-layer tests for connection normalization helpers
// note: if this file changes, update this header and module README.md.
package metadata

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestValidateConnectionInputRejectsEmptyConnection(t *testing.T) {
	t.Parallel()

	err := ValidateConnectionInput(ConnectionInput{})
	if err == nil {
		t.Fatal("expected empty connection error")
	}
	if got := err.Error(); got != "connection must include at least one non-password field" {
		t.Fatalf("unexpected error: %q", got)
	}
	var inputErr *ConnectionInputError
	if !errors.As(err, &inputErr) || inputErr.Kind != ErrorKindValidation {
		t.Fatalf("expected typed validation error, got %#v", err)
	}
	if !IsConnectionInputError(err) {
		t.Fatalf("expected IsConnectionInputError to recognize typed validation error")
	}
}

func TestValidateConnectionInputRejectsHostWithoutUser(t *testing.T) {
	t.Parallel()

	err := ValidateConnectionInput(ConnectionInput{
		Host: "127.0.0.1",
	})
	if err == nil {
		t.Fatal("expected host/user validation error")
	}
	if got := err.Error(); got != "connection must include host/user, socket/user, or connection_ref" {
		t.Fatalf("unexpected error: %q", got)
	}
	var inputErr *ConnectionInputError
	if !errors.As(err, &inputErr) || inputErr.Kind != ErrorKindValidation {
		t.Fatalf("expected typed validation error, got %#v", err)
	}
}

func TestValidateConnectionInputRejectsSocketMixedWithHostOrPort(t *testing.T) {
	t.Parallel()

	err := ValidateConnectionInput(ConnectionInput{
		Host:   "127.0.0.1",
		Port:   3306,
		Socket: "/tmp/mysql.sock",
		User:   "root",
	})
	if err == nil {
		t.Fatal("expected socket/tcp conflict error")
	}
	if got := err.Error(); got != "connection socket cannot be combined with host/port TCP options" {
		t.Fatalf("unexpected error: %q", got)
	}
	var inputErr *ConnectionInputError
	if !errors.As(err, &inputErr) || inputErr.Kind != ErrorKindValidation {
		t.Fatalf("expected typed validation error, got %#v", err)
	}
}

func TestResolvePasswordRejectsMultipleSources(t *testing.T) {
	t.Parallel()

	_, err := ResolvePassword(ConnectionInput{
		Password:    "secret",
		PasswordEnv: "DB_PASSWORD",
	}, ResolveConnectionOptions{})
	if err == nil {
		t.Fatal("expected multiple source error")
	}
	if got := err.Error(); got != "connection password sources are mutually exclusive" {
		t.Fatalf("unexpected error: %q", got)
	}
	var inputErr *ConnectionInputError
	if !errors.As(err, &inputErr) || inputErr.Kind != ErrorKindPasswordSource {
		t.Fatalf("expected typed password-source error, got %#v", err)
	}
}

func TestResolvePasswordReadsEnvAndFileSources(t *testing.T) {
	t.Run("env", func(t *testing.T) {
		t.Parallel()

		got, err := ResolvePassword(ConnectionInput{
			PasswordEnv: "DB_PASSWORD",
		}, ResolveConnectionOptions{
			LookupEnv: func(key string) (string, bool) {
				if key == "DB_PASSWORD" {
					return "from-env", true
				}
				return "", false
			},
		})
		if err != nil {
			t.Fatalf("resolve env password: %v", err)
		}
		if got != "from-env" {
			t.Fatalf("unexpected env password: %q", got)
		}
	})

	t.Run("file", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		got, err := ResolvePassword(ConnectionInput{
			PasswordFile: "~/db/password.txt",
		}, ResolveConnectionOptions{
			ReadFile: func(path string) ([]byte, error) {
				if path != filepath.Join(home, "db/password.txt") {
					t.Fatalf("unexpected file path: %q", path)
				}
				return []byte("from-file\n"), nil
			},
		})
		if err != nil {
			t.Fatalf("resolve file password: %v", err)
		}
		if got != "from-file" {
			t.Fatalf("unexpected file password: %q", got)
		}
	})
}

func TestResolvePasswordReturnsMissingEnvError(t *testing.T) {
	t.Parallel()

	_, err := ResolvePassword(ConnectionInput{
		PasswordEnv: "DB_PASSWORD",
	}, ResolveConnectionOptions{
		LookupEnv: func(key string) (string, bool) {
			return "", false
		},
	})
	if err == nil {
		t.Fatal("expected missing env error")
	}
	if got := err.Error(); got != "password env \"DB_PASSWORD\" is not set" {
		t.Fatalf("unexpected error: %v", err)
	}
	var inputErr *ConnectionInputError
	if !errors.As(err, &inputErr) || inputErr.Kind != ErrorKindPasswordLookup {
		t.Fatalf("expected typed password-lookup error, got %#v", err)
	}
}

func TestIsConnectionInputErrorRejectsPlainTextMatch(t *testing.T) {
	t.Parallel()

	if IsConnectionInputError(errors.New("connection must include at least one non-password field")) {
		t.Fatalf("expected plain text error to be rejected without shared type")
	}
}
