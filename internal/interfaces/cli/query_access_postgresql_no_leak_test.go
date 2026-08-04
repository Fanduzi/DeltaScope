//go:build postgresql && integration

// Package cli verifies that the PostgreSQL 17 query-access CLI surface does not
// leak SQL markers, credentials, catalog facts, server identity, backend
// details, or raw driver errors across online and default-offline paths.
// input: PostgreSQL 17 query-access CLI invocations with unique SQL markers
// output: bounded JSON on stdout, bounded errors on stderr, and no forbidden fields
// pos: CLI no-leak regression coverage for the PG17 COUNT(1) online contract
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestCLIOnlinePG17_CountIntegerOne_NoLeak(t *testing.T) {
	t.Setenv("DELTASCOPE_PG_PASSWORD", "root")
	marker := "PG17ONLINE_NOLEAK_CLI_COUNT_7F3A9C1D"
	sqlText := "SELECT COUNT(1) /* " + marker + " */ FROM app.orders"

	stdout, stderr, exitCode := executePG17CLI(t, sqlText, true)
	if exitCode != exitQueryAccessAdmissible {
		t.Fatalf("expected admissible exit code %d, got %d; stderr=%q", exitQueryAccessAdmissible, exitCode, stderr)
	}
	assertPG17CLINoLeak(t, marker, sqlText, stdout, stderr)

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode CLI JSON: %v; stdout=%q", err, stdout)
	}
	if result["read_classification"] != "read_only" || result["admission"] != "admissible" {
		t.Fatalf("unexpected COUNT(1) result: %#v", result)
	}
}

func TestCLIOnlinePG17_ExcludedShapes_NoLeak(t *testing.T) {
	t.Setenv("DELTASCOPE_PG_PASSWORD", "root")
	cases := []struct {
		name   string
		marker string
		sql    string
	}{
		{"count_two", "PG17ONLINE_NOLEAK_CLI_EXCLUDED_2_91B7E4A0", "SELECT COUNT(2) /* PG17ONLINE_NOLEAK_CLI_EXCLUDED_2_91B7E4A0 */ FROM app.orders"},
		{"count_filter", "PG17ONLINE_NOLEAK_CLI_EXCLUDED_FILTER_3C8D1A6E", "SELECT COUNT(1) FILTER (WHERE true) /* PG17ONLINE_NOLEAK_CLI_EXCLUDED_FILTER_3C8D1A6E */ FROM app.orders"},
		{"count_window", "PG17ONLINE_NOLEAK_CLI_EXCLUDED_WINDOW_6E2A4C9F", "SELECT COUNT(1) OVER () /* PG17ONLINE_NOLEAK_CLI_EXCLUDED_WINDOW_6E2A4C9F */ FROM app.orders"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, exitCode := executePG17CLI(t, tc.sql, true)
			if exitCode != exitQueryAccessIndeterminate {
				t.Fatalf("expected indeterminate exit code %d, got %d; stderr=%q", exitQueryAccessIndeterminate, exitCode, stderr)
			}
			assertPG17CLINoLeak(t, tc.marker, tc.sql, stdout, stderr)
		})
	}
}

func TestCLIOnlinePG17_DefaultOffline_NoLeak(t *testing.T) {
	marker := "PG17ONLINE_NOLEAK_CLI_OFFLINE_4B6D2E8A"
	sqlText := "SELECT COUNT(1) /* " + marker + " */ FROM app.orders"

	stdout, stderr, exitCode := executePG17CLI(t, sqlText, false)
	if exitCode != exitQueryAccessIndeterminate {
		t.Fatalf("expected default offline exit code %d, got %d; stderr=%q", exitQueryAccessIndeterminate, exitCode, stderr)
	}
	assertPG17CLINoLeak(t, marker, sqlText, stdout, stderr)
}

func executePG17CLI(t *testing.T, sqlText string, online bool) (string, string, int) {
	t.Helper()
	args := []string{
		"query-access", "analyze",
		"--sql", sqlText,
		"--dialect", "postgresql",
		"--mode", "strict",
	}
	if online {
		args = append(args,
			"--host", envOrPG17CLI("DELTASCOPE_PG_HOST", "127.0.0.1"),
			"--port", strconv.Itoa(envOrPG17CLIPort()),
			"--user", envOrPG17CLI("DELTASCOPE_PG_USER", "root"),
			"--database", envOrPG17CLI("DELTASCOPE_PG_DATABASE", "postgres"),
			"--schema", "app",
			"--password-env", "DELTASCOPE_PG_PASSWORD",
		)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(context.Background(), args, &bytes.Buffer{}, &stdout, &stderr)
	return stdout.String(), stderr.String(), exitCode
}

func envOrPG17CLI(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envOrPG17CLIPort() int {
	value := envOrPG17CLI("DELTASCOPE_PG_PORT", "5500")
	port, err := strconv.Atoi(value)
	if err != nil {
		return 5500
	}
	return port
}

func assertPG17CLINoLeak(t *testing.T, marker, sqlText, stdout, stderr string) {
	t.Helper()
	for _, forbidden := range []string{
		marker,
		sqlText,
		"PG17ONLINE_NOLEAK_CLI_USER_3D1A",
		"PG17ONLINE_NOLEAK_CLI_PASSWORD_8E4B",
		"PG17ONLINE_NOLEAK_CLI_CATALOG_5A7C",
		"PG17ONLINE_NOLEAK_CLI_VERSION_17000",
		"PG17ONLINE_NOLEAK_CLI_BACKEND_4F2B",
		"catalog lookup driver error",
		"postgres://",
		"pgx",
		"password",
		"server_version",
		"backend_pid",
	} {
		if strings.Contains(stdout, forbidden) {
			t.Errorf("CLI stdout leaked %q: %s", forbidden, stdout)
		}
		if strings.Contains(stderr, forbidden) {
			t.Errorf("CLI stderr leaked %q: %s", forbidden, stderr)
		}
	}

	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		var payload map[string]any
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("decode CLI JSON for forbidden-field check: %v", err)
		}
		for _, field := range []string{
			"raw_sql", "dsn", "credentials", "catalog", "server_version",
			"backend_pid", "backend_details", "driver_error", "identity",
			"manifest", "session", "context", "severity",
		} {
			if _, present := payload[field]; present {
				t.Errorf("CLI JSON carried forbidden field %q: %s", field, stdout)
			}
		}
	}
}
