// Package cli verifies metadata-aware connection failure exit codes and bounded messages.
// input: Execute args for unreachable, authentication, and password-source failures
// output: exit-code, one-line bounded stderr, and no-leak coverage for issue #23
// pos: interface-layer contract tests for metadata connection error mapping
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
)

func TestAuditUnreachableMetadataServerExitsRuntime(t *testing.T) {
	t.Setenv("DELTASCOPE_TEST_PASSWORD", "testpass")

	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{
			"audit",
			"--sql", "alter table users drop column email",
			"--host", "127.0.0.1",
			"--port", "3999",
			"--user", "root",
			"--password-env", "DELTASCOPE_TEST_PASSWORD",
			"--schema", "app",
			"--metadata-connect-timeout", "2s",
		},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != exitInternal {
		t.Fatalf("expected exit %d for unreachable metadata server, got %d (stderr=%q)", exitInternal, code, stderr.String())
	}
	if stderr.String() != "connection failed\n" {
		t.Fatalf("expected bounded connection failed message, got %q", stderr.String())
	}
	assertNoConnectionInternals(t, stderr.String(), "127.0.0.1", "3999", "root", "testpass", "app")
}

func TestAuditAuthenticationFailureExitsRuntime(t *testing.T) {
	previous := newMetadataClient
	newMetadataClient = func(auditConnectionOptions) (metadataClient, error) {
		return nil, errors.New("Error 1045 (28000): Access denied for user 'root'@'localhost' (using password: YES)")
	}
	t.Cleanup(func() { newMetadataClient = previous })

	t.Setenv("DELTASCOPE_TEST_PASSWORD", "wrongpassword")
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{
			"audit",
			"--sql", "alter table users drop column email",
			"--host", "127.0.0.1",
			"--port", "3306",
			"--user", "root",
			"--password-env", "DELTASCOPE_TEST_PASSWORD",
			"--schema", "app",
		},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != exitInternal {
		t.Fatalf("expected exit %d for authentication failure, got %d (stderr=%q)", exitInternal, code, stderr.String())
	}
	if stderr.String() != "authentication failed\n" {
		t.Fatalf("expected bounded authentication failed message, got %q", stderr.String())
	}
	assertNoConnectionInternals(t, stderr.String(), "127.0.0.1", "3306", "root", "wrongpassword", "1045", "Access denied", "localhost")
}

func TestAuditOmittedPasswordSourceOnAuthFailureExitsUser(t *testing.T) {
	previous := newMetadataClient
	opened := false
	newMetadataClient = func(auditConnectionOptions) (metadataClient, error) {
		opened = true
		return nil, errors.New("Error 1045 (28000): Access denied for user 'root'@'localhost' (using password: NO)")
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{
			"audit",
			"--sql", "alter table users drop column email",
			"--host", "127.0.0.1",
			"--port", "3306",
			"--user", "root",
			"--schema", "app",
		},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if !opened {
		t.Fatal("expected empty-password connect attempt so successful password-less audits stay possible")
	}
	if code != exitUser {
		t.Fatalf("expected exit %d for omitted password source, got %d (stderr=%q)", exitUser, code, stderr.String())
	}
	if stderr.String() != "password source required: use --password-env, --password-file, or --ask-password\n" {
		t.Fatalf("expected password-source required message, got %q", stderr.String())
	}
	assertNoConnectionInternals(t, stderr.String(), "127.0.0.1", "3306", "root", "1045", "Access denied", "localhost")
}

func TestAuditConnectionTimeoutExitsRuntime(t *testing.T) {
	previous := newMetadataClient
	newMetadataClient = func(auditConnectionOptions) (metadataClient, error) {
		return nil, errors.New("dial tcp 10.0.0.1:3306: i/o timeout")
	}
	t.Cleanup(func() { newMetadataClient = previous })

	t.Setenv("DELTASCOPE_TEST_PASSWORD", "testpass")
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{
			"audit",
			"--sql", "alter table users drop column email",
			"--host", "10.0.0.1",
			"--port", "3306",
			"--user", "root",
			"--password-env", "DELTASCOPE_TEST_PASSWORD",
			"--schema", "app",
		},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != exitInternal {
		t.Fatalf("expected exit %d for connection timeout, got %d (stderr=%q)", exitInternal, code, stderr.String())
	}
	if stderr.String() != "connection timed out\n" {
		t.Fatalf("expected bounded timeout message, got %q", stderr.String())
	}
	assertNoConnectionInternals(t, stderr.String(), "10.0.0.1", "3306", "root", "testpass")
}

func TestAuditTLSFailureExitsRuntime(t *testing.T) {
	previous := newMetadataClient
	newMetadataClient = func(auditConnectionOptions) (metadataClient, error) {
		return nil, errors.New("tls: failed to verify certificate: x509: certificate signed by unknown authority")
	}
	t.Cleanup(func() { newMetadataClient = previous })

	t.Setenv("DELTASCOPE_TEST_PASSWORD", "testpass")
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{
			"audit",
			"--sql", "alter table users drop column email",
			"--host", "db.example.com",
			"--port", "3306",
			"--user", "root",
			"--password-env", "DELTASCOPE_TEST_PASSWORD",
			"--schema", "app",
			"--tls-mode", "enabled",
		},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != exitInternal {
		t.Fatalf("expected exit %d for TLS failure, got %d (stderr=%q)", exitInternal, code, stderr.String())
	}
	if stderr.String() != "TLS certificate verification failed\n" {
		t.Fatalf("expected bounded TLS message, got %q", stderr.String())
	}
	assertNoConnectionInternals(t, stderr.String(), "db.example.com", "3306", "root", "x509")
}

func TestAuditMissingPasswordEnvStaysUserErrorWithoutConnect(t *testing.T) {
	previous := newMetadataClient
	opened := false
	newMetadataClient = func(auditConnectionOptions) (metadataClient, error) {
		opened = true
		return nil, errors.New("should not open metadata client")
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{
			"audit",
			"--sql", "alter table users drop column email",
			"--host", "127.0.0.1",
			"--port", "3306",
			"--user", "root",
			"--password-env", "DOES_NOT_EXIST",
			"--schema", "app",
		},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if opened {
		t.Fatal("missing password-env must fail before a connect attempt")
	}
	if code != exitUser {
		t.Fatalf("expected exit %d for missing password-env, got %d (stderr=%q)", exitUser, code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid password source") {
		t.Fatalf("expected invalid password source message, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "DOES_NOT_EXIST") {
		t.Fatalf("stderr leaked password-env name: %q", stderr.String())
	}
}

func TestMapAuditErrorConnectionRefusedExitsRuntime(t *testing.T) {
	code := 0
	got := mapAuditError(&code, errors.New("dial tcp 127.0.0.1:3306: connection refused"))
	if code != exitInternal {
		t.Fatalf("expected exit %d for remapped connection failure, got %d", exitInternal, code)
	}
	if got.Error() != "connection failed" {
		t.Fatalf("expected bounded connection failed message, got %q", got.Error())
	}
	assertNoConnectionInternals(t, got.Error(), "127.0.0.1", "3306", "dial tcp")
}

func TestMapAuditErrorAccessDeniedExitsRuntime(t *testing.T) {
	code := 0
	got := mapAuditError(&code, errors.New("Error 1045 (28000): Access denied for user 'root'@'localhost' (using password: YES)"))
	if code != exitInternal {
		t.Fatalf("expected exit %d for remapped authentication failure, got %d", exitInternal, code)
	}
	if got.Error() != "authentication failed" {
		t.Fatalf("expected bounded authentication failed message, got %q", got.Error())
	}
	assertNoConnectionInternals(t, got.Error(), "root", "localhost", "1045", "Access denied")
}

func TestAuditMetaConnectionOpenClassifiesAuthWithoutLeak(t *testing.T) {
	err := &auditmeta.Error{
		Kind:    auditmeta.ErrorConnectionOpen,
		Message: "open metadata connection: Error 1045 (28000): Access denied for user 'root'@'10.0.0.1' (using password: YES)",
	}
	bounded := mapAuditMetaErrorToBounded(err)
	if bounded.Error() != "authentication failed" {
		t.Fatalf("expected authentication failed, got %q", bounded.Error())
	}
	assertNoConnectionInternals(t, bounded.Error(), "root", "10.0.0.1", "1045", "Access denied", "password")
}

func assertNoConnectionInternals(t *testing.T, got string, tokens ...string) {
	t.Helper()
	lower := strings.ToLower(got)
	for _, token := range tokens {
		if strings.Contains(lower, strings.ToLower(token)) {
			t.Fatalf("bounded message %q leaked %q", got, token)
		}
	}
}
