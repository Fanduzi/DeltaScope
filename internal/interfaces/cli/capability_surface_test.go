//go:build !postgresql

package cli

import (
	"context"
	"strings"
	"testing"
)

func TestPureGoCapabilitiesOmitPostgreSQL(t *testing.T) {
	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"capabilities"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := stdout.String()
	for _, expected := range []string{"mysql", "tidb"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected pure-Go capabilities output to contain %q, got %q", expected, output)
		}
	}
	if strings.Contains(output, "postgresql") {
		t.Fatalf("expected pure-Go capabilities output to omit postgresql, got %q", output)
	}
}

func TestPureGoVersionCommandShowsCompiledDialects(t *testing.T) {
	stdout := &strings.Builder{}
	previous := Version
	Version = "test-build"
	t.Cleanup(func() { Version = previous })

	code := Execute(
		context.Background(),
		[]string{"version"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "____") {
		t.Fatalf("expected logo output, got %q", output)
	}
	for _, expected := range []string{"test-build", "mysql", "tidb"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected pure-Go version output to contain %q, got %q", expected, output)
		}
	}
	if strings.Contains(output, "postgresql") {
		t.Fatalf("expected pure-Go version output to omit postgresql, got %q", output)
	}
}

func TestPureGoRootVersionFlagShowsCompiledDialects(t *testing.T) {
	stdout := &strings.Builder{}
	previous := Version
	Version = "test-build"
	t.Cleanup(func() { Version = previous })

	code := Execute(
		context.Background(),
		[]string{"--version"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := strings.TrimSpace(stdout.String())
	for _, expected := range []string{"test-build", "mysql", "tidb"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected pure-Go root version output to contain %q, got %q", expected, output)
		}
	}
	if strings.Contains(output, "postgresql") {
		t.Fatalf("expected pure-Go root version output to omit postgresql, got %q", output)
	}
}

func TestPureGoRootHelpWordingMatchesCapabilitySurface(t *testing.T) {
	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"--help"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	output := stdout.String()
	for _, expected := range []string{"MySQL", "TiDB"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected pure-Go help output to contain %q, got %q", expected, output)
		}
	}
	if strings.Contains(output, "PostgreSQL") {
		t.Fatalf("expected pure-Go help output to omit PostgreSQL, got %q", output)
	}
	if strings.Contains(output, "postgresql") {
		t.Fatalf("expected pure-Go dialect flag help to omit postgresql, got %q", output)
	}
}
