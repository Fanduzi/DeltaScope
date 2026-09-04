// Package cli verifies the CLI version-string seam.
// input: version command and --version flag with ldflags override, empty override, and release-tag override
// output: assertions that an unset version uses ReportedVersion, release ldflags keep the tag, and untagged builds do not print DefaultVersion as the sole version
// pos: CLI adapter tests for the public version helper
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"strings"
	"testing"

	publicapi "github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestResolvedVersionPrefersLdflagsOverride(t *testing.T) {
	previous := Version
	Version = "v0.510.3"
	t.Cleanup(func() { Version = previous })

	if got := resolvedVersion(); got != "v0.510.3" {
		t.Fatalf("resolvedVersion() = %q, want ldflags release tag", got)
	}
}

func TestResolvedVersionUsesReportedVersionWhenUnset(t *testing.T) {
	previous := Version
	Version = ""
	t.Cleanup(func() { Version = previous })

	got := resolvedVersion()
	want := publicapi.ReportedVersion()
	if got != want {
		t.Fatalf("resolvedVersion() = %q, want ReportedVersion %q", got, want)
	}
	if got == "" {
		t.Fatal("resolvedVersion() must not be empty")
	}
}

func TestVersionCommandKeepsReleaseLdflagsVersion(t *testing.T) {
	previous := Version
	Version = publicapi.DefaultVersion
	t.Cleanup(func() { Version = previous })

	stdout := &strings.Builder{}
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
	if !strings.Contains(output, "deltascope "+publicapi.DefaultVersion+" (") {
		t.Fatalf("expected release ldflags version %q, got %q", publicapi.DefaultVersion, output)
	}
}

func TestVersionCommandDoesNotClaimDefaultVersionWhenUnset(t *testing.T) {
	previous := Version
	Version = ""
	t.Cleanup(func() { Version = previous })

	stdout := &strings.Builder{}
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

	want := resolvedVersion()
	output := stdout.String()
	if !strings.Contains(output, "deltascope "+want+" (") {
		t.Fatalf("expected version line to use %q, got %q", want, output)
	}
	if want != publicapi.DefaultVersion && strings.Contains(output, "deltascope "+publicapi.DefaultVersion+" (") {
		t.Fatalf("untagged build claimed DefaultVersion %q as the sole version, got %q", publicapi.DefaultVersion, output)
	}
}

func TestRootVersionFlagDoesNotClaimDefaultVersionWhenUnset(t *testing.T) {
	previous := Version
	Version = ""
	t.Cleanup(func() { Version = previous })

	stdout := &strings.Builder{}
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

	want := resolvedVersion()
	output := strings.TrimSpace(stdout.String())
	if !strings.Contains(output, "deltascope "+want+" (") {
		t.Fatalf("expected root version line to use %q, got %q", want, output)
	}
	if want != publicapi.DefaultVersion && strings.Contains(output, "deltascope "+publicapi.DefaultVersion+" (") {
		t.Fatalf("untagged build claimed DefaultVersion %q as the sole version, got %q", publicapi.DefaultVersion, output)
	}
}
