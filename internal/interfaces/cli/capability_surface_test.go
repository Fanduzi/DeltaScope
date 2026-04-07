//go:build !postgresql

package cli

import (
	"context"
	"strings"
	"testing"
)

func TestPureGoCapabilitiesAdvertiseUnifiedProductSurface(t *testing.T) {
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
	for _, expected := range []string{
		"available in this build:",
		"product dialects:",
		"mysql",
		"tidb",
		"postgresql",
		"postgresql requires a PG-capable build",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected pure-Go capabilities output to contain %q, got %q", expected, output)
		}
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
	for _, expected := range []string{"MySQL", "TiDB", "PostgreSQL", "postgresql requires a PG-capable build"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected pure-Go help output to contain %q, got %q", expected, output)
		}
	}
}
