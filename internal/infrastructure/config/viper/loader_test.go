// Package viperconfig verifies YAML-backed policy loading behavior.
// input: temporary YAML config files and default policy loading scenarios
// output: test coverage for Viper-based policy loading
// pos: infrastructure config adapter test coverage
// note: if this file changes, update this header and module README.md.
package viperconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func TestLoaderUsesBuiltInDefaultsWhenConfigPathIsEmpty(t *testing.T) {
	cfg, err := LoadPolicy("")
	if err != nil {
		t.Fatalf("expected no error loading defaults, got %v", err)
	}

	ruleCfg, ok := cfg.Rules["dml.where.require"]
	if !ok {
		t.Fatalf("expected default rule dml.where.require to exist")
	}
	if !ruleCfg.Enabled {
		t.Fatalf("expected dml.where.require to be enabled by default")
	}
	if ruleCfg.Level != rule.LevelBlocker {
		t.Fatalf("expected dml.where.require level %q, got %q", rule.LevelBlocker, ruleCfg.Level)
	}

	param, ok := ruleCfg.Params["required"].(bool)
	if !ok || !param {
		t.Fatalf("expected dml.where.require.required default param to be true, got %#v", ruleCfg.Params["required"])
	}
}

func TestLoaderAppliesYAMLOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deltascope.yaml")
	content := []byte(`
rules:
  dml.where.require:
    enabled: false
    level: notice
    params:
      required: false
  ddl.table.name.max_length:
    enabled: true
    level: warning
    params:
      limit: 48
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("expected no error loading config, got %v", err)
	}

	whereRule := cfg.Rules["dml.where.require"]
	if whereRule.Enabled {
		t.Fatalf("expected dml.where.require to be disabled by override")
	}
	if whereRule.Level != rule.LevelNotice {
		t.Fatalf("expected overridden level %q, got %q", rule.LevelNotice, whereRule.Level)
	}
	required, ok := whereRule.Params["required"].(bool)
	if !ok || required {
		t.Fatalf("expected overridden required=false, got %#v", whereRule.Params["required"])
	}

	nameRule := cfg.Rules["ddl.table.name.max_length"]
	if nameRule.Level != rule.LevelWarning {
		t.Fatalf("expected overridden level %q, got %q", rule.LevelWarning, nameRule.Level)
	}
	limit, ok := nameRule.Params["limit"].(int)
	if !ok || limit != 48 {
		t.Fatalf("expected limit=48, got %#v", nameRule.Params["limit"])
	}
}
