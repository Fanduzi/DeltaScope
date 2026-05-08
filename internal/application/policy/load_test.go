package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func TestLoadReturnsDefaultPolicyWhenPathIsEmpty(t *testing.T) {
	t.Parallel()

	p, err := Load("")
	if err != nil {
		t.Fatalf("Load empty path: %v", err)
	}

	r, ok := p.Rules["dml.where.require"]
	if !ok {
		t.Fatal("expected dml.where.require in default policy")
	}
	if !r.Enabled {
		t.Fatal("expected dml.where.require enabled by default")
	}
	if r.Level != rule.LevelBlocker {
		t.Fatalf("expected blocker level, got %q", r.Level)
	}
}

func TestLoadAppliesValidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	content := []byte(`
rules:
  dml.where.require:
    enabled: false
    level: notice
    params:
      required: false
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load valid YAML: %v", err)
	}

	r := p.Rules["dml.where.require"]
	if r.Enabled {
		t.Fatal("expected dml.where.require disabled by override")
	}
	if r.Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", r.Level)
	}
}

func TestLoadReturnsErrorForInvalidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("::not valid yaml{{\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error loading invalid YAML")
	}
}

func TestLoadReturnsErrorForNonexistentFile(t *testing.T) {
	t.Parallel()

	_, err := Load("/no/such/path/deltascope.yaml")
	if err == nil {
		t.Fatal("expected error loading nonexistent file")
	}
}

func TestLoadReturnsErrorForUnknownFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "unknown.yaml")
	content := []byte(`
rules:
  dml.where.require:
    enabled: true
    made_up_field: oops
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown YAML fields")
	}
}
