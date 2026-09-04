// Package cli verifies CLI help versus host shorthand flags.
// input: Execute args for audit and query-access analyze -h/--help/-H/--host
// output: exit 0 help text for -h/--help, host binding for -H/--host, no flag-needs-argument on bare -h
// pos: interface-layer contract tests for issue #75
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/online"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestAuditBareHPrintsHelpAndExitsZero(t *testing.T) {
	t.Parallel()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "-h"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitOK {
		t.Fatalf("expected exit %d for audit -h, got %d (stderr=%q stdout=%q)", exitOK, code, stderr.String(), stdout.String())
	}
	assertHelpOutput(t, stdout.String(), stderr.String(), "audit", "-H, --host", "-h, --help")
}

func TestAuditLongHelpStillPrintsHelp(t *testing.T) {
	t.Parallel()

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--help"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitOK {
		t.Fatalf("expected exit %d for audit --help, got %d (stderr=%q stdout=%q)", exitOK, code, stderr.String(), stdout.String())
	}
	assertHelpOutput(t, stdout.String(), stderr.String(), "audit", "-H, --host", "-h, --help")
}

func TestAuditCapitalHBindsHost(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": {"app"}},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "-H", "127.0.0.1", "--user", "root"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected blocker exit %d for audit -H host, got %d (stderr=%q)", exitAudit, code, stderr.String())
	}
	if client.options.Host != "127.0.0.1" {
		t.Fatalf("expected -H to bind host 127.0.0.1, got %q", client.options.Host)
	}
	if strings.Contains(stdout.String()+stderr.String(), "flag needs an argument") {
		t.Fatalf("audit -H must bind host, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestAuditLongHostStillBindsHost(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectMySQL,
		schemasByTable: map[string][]string{"users": {"app"}},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from users", "--host", "10.0.0.8", "--user", "root"},
		strings.NewReader(""),
		&strings.Builder{},
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected blocker exit %d for audit --host, got %d (stderr=%q)", exitAudit, code, stderr.String())
	}
	if client.options.Host != "10.0.0.8" {
		t.Fatalf("expected --host to bind 10.0.0.8, got %q", client.options.Host)
	}
}

func TestQueryAccessAnalyzeBareHPrintsHelpAndExitsZero(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Execute(t.Context(), []string{"query-access", "analyze", "-h"}, &bytes.Buffer{}, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected exit %d for query-access analyze -h, got %d (stderr=%q stdout=%q)", exitOK, code, stderr.String(), stdout.String())
	}
	assertHelpOutput(t, stdout.String(), stderr.String(), "analyze", "-H, --host", "-h, --help")
}

func TestQueryAccessAnalyzeLongHelpStillPrintsHelp(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Execute(t.Context(), []string{"query-access", "analyze", "--help"}, &bytes.Buffer{}, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected exit %d for query-access analyze --help, got %d (stderr=%q stdout=%q)", exitOK, code, stderr.String(), stdout.String())
	}
	assertHelpOutput(t, stdout.String(), stderr.String(), "analyze", "-H, --host", "-h, --help")
}

func TestQueryAccessAnalyzeCapitalHBindsHost(t *testing.T) {
	previousOpener := openOnlineSession
	previousConstructor := newOnlineQueryAccessSessionFromConn
	previousAnalyzer := analyzeOnlineQueryAccessWithSession
	t.Cleanup(func() {
		openOnlineSession = previousOpener
		newOnlineQueryAccessSessionFromConn = previousConstructor
		analyzeOnlineQueryAccessWithSession = previousAnalyzer
	})
	t.Setenv("CLI_HOST_HELP_PASSWORD", "secret")

	var sessionConfig online.SessionConfig
	openOnlineSession = func(_ context.Context, cfg online.SessionConfig) (*online.Session, error) {
		sessionConfig = cfg
		return &online.Session{Conn: &sql.Conn{}, Close: func() error { return nil }}, nil
	}
	newOnlineQueryAccessSessionFromConn = func(context.Context, *sql.Conn) (*deltascope.OnlineQueryAccessSession, error) {
		return nil, nil
	}
	analyzeOnlineQueryAccessWithSession = func(context.Context, *deltascope.OnlineQueryAccessSession, deltascope.QueryAccessRequest) (*deltascope.QueryAccessResult, error) {
		return &deltascope.QueryAccessResult{
			Dialect:            "mysql",
			Mode:               deltascope.QueryAccessModeStrict,
			ReadClassification: deltascope.QueryAccessReadOnly,
			Admission:          deltascope.QueryAccessAdmissible,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Execute(t.Context(), []string{
		"query-access", "analyze",
		"--sql", "SELECT 1",
		"--dialect", "mysql",
		"-H", "127.0.0.1",
		"--user", "root",
		"--password-env", "CLI_HOST_HELP_PASSWORD",
	}, &bytes.Buffer{}, &stdout, &stderr)

	if code != exitQueryAccessAdmissible {
		t.Fatalf("expected exit %d for query-access analyze -H host, got %d (stderr=%q)", exitQueryAccessAdmissible, code, stderr.String())
	}
	if sessionConfig.Host != "127.0.0.1" {
		t.Fatalf("expected -H to bind host 127.0.0.1, got %q", sessionConfig.Host)
	}
}

func TestQueryAccessAnalyzeLongHostStillBindsHost(t *testing.T) {
	previousOpener := openOnlineSession
	previousConstructor := newOnlineQueryAccessSessionFromConn
	previousAnalyzer := analyzeOnlineQueryAccessWithSession
	t.Cleanup(func() {
		openOnlineSession = previousOpener
		newOnlineQueryAccessSessionFromConn = previousConstructor
		analyzeOnlineQueryAccessWithSession = previousAnalyzer
	})
	t.Setenv("CLI_HOST_HELP_PASSWORD", "secret")

	var sessionConfig online.SessionConfig
	openOnlineSession = func(_ context.Context, cfg online.SessionConfig) (*online.Session, error) {
		sessionConfig = cfg
		return &online.Session{Conn: &sql.Conn{}, Close: func() error { return nil }}, nil
	}
	newOnlineQueryAccessSessionFromConn = func(context.Context, *sql.Conn) (*deltascope.OnlineQueryAccessSession, error) {
		return nil, nil
	}
	analyzeOnlineQueryAccessWithSession = func(context.Context, *deltascope.OnlineQueryAccessSession, deltascope.QueryAccessRequest) (*deltascope.QueryAccessResult, error) {
		return &deltascope.QueryAccessResult{
			Dialect:            "mysql",
			Mode:               deltascope.QueryAccessModeStrict,
			ReadClassification: deltascope.QueryAccessReadOnly,
			Admission:          deltascope.QueryAccessAdmissible,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Execute(t.Context(), []string{
		"query-access", "analyze",
		"--sql", "SELECT 1",
		"--dialect", "mysql",
		"--host", "10.0.0.8",
		"--user", "root",
		"--password-env", "CLI_HOST_HELP_PASSWORD",
	}, &bytes.Buffer{}, &stdout, &stderr)

	if code != exitQueryAccessAdmissible {
		t.Fatalf("expected exit %d for query-access analyze --host, got %d (stderr=%q)", exitQueryAccessAdmissible, code, stderr.String())
	}
	if sessionConfig.Host != "10.0.0.8" {
		t.Fatalf("expected --host to bind 10.0.0.8, got %q", sessionConfig.Host)
	}
}

func assertHelpOutput(t *testing.T, stdout string, stderr string, command string, required ...string) {
	t.Helper()
	combined := stdout + stderr
	if strings.Contains(combined, "flag needs an argument: -h") {
		t.Fatalf("bare -h must not be flag-needs-argument, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Fatalf("expected help usage on stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, command) {
		t.Fatalf("expected help for %q on stdout, got %q", command, stdout)
	}
	for _, want := range required {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected help to advertise %q, got %q", want, stdout)
		}
	}
}
