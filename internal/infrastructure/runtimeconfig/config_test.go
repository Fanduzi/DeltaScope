package runtimeconfig

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadEmptyPathReturnsZeroConfig(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load('') returned error: %v", err)
	}
	if cfg.Logging.Level != "" {
		t.Errorf("expected empty Logging.Level, got %q", cfg.Logging.Level)
	}
	if cfg.Metadata.ConnectTimeout != "" {
		t.Errorf("expected empty Metadata.ConnectTimeout, got %q", cfg.Metadata.ConnectTimeout)
	}
}

func TestLoadRuntimeConfigReadsLoggingAndMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	content := []byte(`
logging:
  level: debug
  format: json
  output: file
  file: /var/log/deltascope/server.log
  rotate:
    enabled: true
    max_size_mb: 50
    max_backups: 5
    max_age_days: 7
    compress: false
metadata:
  connect_timeout: 5s
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", path, err)
	}

	assertLogging(t, cfg)
	assertRotate(t, cfg)
	assertMetadata(t, cfg)
}

func assertLogging(t *testing.T, cfg Config) {
	t.Helper()
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "debug")
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, "json")
	}
	if cfg.Logging.Output != "file" {
		t.Errorf("Logging.Output = %q, want %q", cfg.Logging.Output, "file")
	}
	if cfg.Logging.File != "/var/log/deltascope/server.log" {
		t.Errorf("Logging.File = %q, want %q", cfg.Logging.File, "/var/log/deltascope/server.log")
	}
}

func assertRotate(t *testing.T, cfg Config) {
	t.Helper()
	if cfg.Logging.Rotate.Enabled == nil || !*cfg.Logging.Rotate.Enabled {
		t.Error("Rotate.Enabled = false, want true")
	}
	if cfg.Logging.Rotate.MaxSizeMB == nil || *cfg.Logging.Rotate.MaxSizeMB != 50 {
		t.Errorf("Rotate.MaxSizeMB = %v, want 50", cfg.Logging.Rotate.MaxSizeMB)
	}
	if cfg.Logging.Rotate.MaxBackups == nil || *cfg.Logging.Rotate.MaxBackups != 5 {
		t.Errorf("Rotate.MaxBackups = %v, want 5", cfg.Logging.Rotate.MaxBackups)
	}
	if cfg.Logging.Rotate.MaxAgeDays == nil || *cfg.Logging.Rotate.MaxAgeDays != 7 {
		t.Errorf("Rotate.MaxAgeDays = %v, want 7", cfg.Logging.Rotate.MaxAgeDays)
	}
	if cfg.Logging.Rotate.Compress == nil || *cfg.Logging.Rotate.Compress {
		t.Errorf("Rotate.Compress = true, want false")
	}
}

func assertMetadata(t *testing.T, cfg Config) {
	t.Helper()
	if cfg.Metadata.ConnectTimeout != "5s" {
		t.Errorf("Metadata.ConnectTimeout = %q, want %q", cfg.Metadata.ConnectTimeout, "5s")
	}
}

func TestLoadRuntimeConfigRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	content := []byte(`
logging:
  level: info
unknown_section:
  foo: bar
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown YAML fields, got nil")
	}
	if !errors.Is(err, os.ErrInvalid) {
		t.Errorf("error should wrap os.ErrInvalid, got: %v", err)
	}
}

func TestLoadRuntimeConfigWrapsReadErrorWithPath(t *testing.T) {
	path := "/nonexistent/path/runtime.yaml"
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error should wrap os.ErrNotExist, got: %v", err)
	}
}

func TestParseMetadataConnectTimeoutDefaultsWhenEmpty(t *testing.T) {
	d, ok, err := ParseConnectTimeout("")
	if err != nil {
		t.Fatalf("ParseConnectTimeout('') returned error: %v", err)
	}
	if ok {
		t.Error("ParseConnectTimeout('') ok = true, want false")
	}
	if d != 0 {
		t.Errorf("ParseConnectTimeout('') duration = %v, want 0", d)
	}
}

func TestParseMetadataConnectTimeoutTreatsZeroAsUnset(t *testing.T) {
	d, ok, err := ParseConnectTimeout("0s")
	if err != nil {
		t.Fatalf("ParseConnectTimeout('0s') returned error: %v", err)
	}
	if ok {
		t.Error("ParseConnectTimeout('0s') ok = true, want false")
	}
	if d != 0 {
		t.Errorf("ParseConnectTimeout('0s') duration = %v, want 0", d)
	}
}

func TestParseMetadataConnectTimeoutParsesPositiveDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"5s", 5 * time.Second},
		{"500ms", 500 * time.Millisecond},
		{"1m", time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			d, ok, err := ParseConnectTimeout(tt.input)
			if err != nil {
				t.Fatalf("ParseConnectTimeout(%q) returned error: %v", tt.input, err)
			}
			if !ok {
				t.Errorf("ParseConnectTimeout(%q) ok = false, want true", tt.input)
			}
			if d != tt.want {
				t.Errorf("ParseConnectTimeout(%q) = %v, want %v", tt.input, d, tt.want)
			}
		})
	}
}

func TestParseMetadataConnectTimeoutRejectsInvalidDuration(t *testing.T) {
	_, _, err := ParseConnectTimeout("not-a-duration")
	if err == nil {
		t.Fatal("expected error for invalid duration, got nil")
	}
}

func TestParseMetadataConnectTimeoutRejectsNegativeDuration(t *testing.T) {
	_, _, err := ParseConnectTimeout("-5s")
	if err == nil {
		t.Fatal("expected error for negative duration, got nil")
	}
}
