//go:build postgresql

package cli

import (
	"context"
	"strings"
	"testing"
)

func TestTaggedCapabilitiesIncludePostgreSQL(t *testing.T) {
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
	for _, expected := range []string{"available in this build:", "product dialects:", "mysql", "tidb", "postgresql"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected tagged capabilities output to contain %q, got %q", expected, output)
		}
	}
	if strings.Contains(output, "requires a PG-capable build") {
		t.Fatalf("did not expect tagged capabilities output to mention PG-capable build, got %q", output)
	}
}

func TestTaggedVersionCommandShowsCompiledDialects(t *testing.T) {
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
	for _, expected := range []string{"test-build", "mysql", "tidb", "postgresql"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected tagged version output to contain %q, got %q", expected, output)
		}
	}
}

func TestTaggedRootVersionFlagShowsCompiledDialects(t *testing.T) {
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
	for _, expected := range []string{"test-build", "mysql", "tidb", "postgresql"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected tagged root version output to contain %q, got %q", expected, output)
		}
	}
}

func TestTaggedRootHelpWordingMatchesCapabilitySurface(t *testing.T) {
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
	for _, expected := range []string{"MySQL", "TiDB", "PostgreSQL", "postgresql"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected tagged help output to contain %q, got %q", expected, output)
		}
	}
	if strings.Contains(output, "requires a PG-capable build") {
		t.Fatalf("did not expect tagged help output to mention PG-capable build, got %q", output)
	}
}
