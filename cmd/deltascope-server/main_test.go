// Package main verifies CLI flag parsing helpers for the HTTP server entrypoint.
// input: raw comma-separated flag strings
// output: normalized token slices used by auth flag parsing
// pos: lightweight unit tests for command bootstrap helpers
// note: if this file changes, update this header and module README.md.
package main

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Fanduzi/DeltaScope/internal/infrastructure/logger"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
)

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  []string
	}{
		{name: "empty", in: "", out: []string{}},
		{name: "single", in: "abc", out: []string{"abc"}},
		{name: "trim and skip empty", in: " a, ,b ,, c ", out: []string{"a", "b", "c"}},
		{name: "whitespace only", in: "   ", out: []string{}},
		{name: "multiple commas", in: ",,,", out: []string{}},
		{name: "pathlike values", in: "/healthz,/readyz,/version", out: []string{"/healthz", "/readyz", "/version"}},
		{name: "mixed spacing", in: "key1 , key2,key3  ,  key4", out: []string{"key1", "key2", "key3", "key4"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCSV(tc.in)
			if !reflect.DeepEqual(got, tc.out) {
				t.Fatalf("parseCSV(%q) = %#v, want %#v", tc.in, got, tc.out)
			}
		})
	}
}

func TestVersionDefaultsToPublicAPIVersion(t *testing.T) {
	if Version == "" {
		t.Fatal("expected non-empty default version")
	}
}

func TestLoggerConfigFromFlagsDefaultCreatesLogger(t *testing.T) {
	cfg := loggerConfigFromFlags("info", "json", "stderr", "", false, 100, 3, 30, true)
	l, err := logger.NewLogger(cfg, "server")
	if err != nil {
		t.Fatalf("default config should succeed: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLoggerConfigFromFlagsInvalidLevel(t *testing.T) {
	cfg := loggerConfigFromFlags("trace", "json", "stderr", "", false, 100, 3, 30, true)
	_, err := logger.NewLogger(cfg, "server")
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestLoggerConfigFromFlagsFileOutput(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")
	cfg := loggerConfigFromFlags("info", "json", "file", tmpFile, false, 100, 3, 30, true)
	l, err := logger.NewLogger(cfg, "server")
	if err != nil {
		t.Fatalf("file output should succeed: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLoggerConfigFromFlagsInvalidFormat(t *testing.T) {
	cfg := loggerConfigFromFlags("info", "xml", "stderr", "", false, 100, 3, 30, true)
	_, err := logger.NewLogger(cfg, "server")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestLoggerConfigFromFlagsDefaultRotateDisabled(t *testing.T) {
	cfg := loggerConfigFromFlags("info", "json", "stderr", "", false, 100, 3, 30, true)
	if cfg.Rotate != nil {
		t.Fatal("expected nil Rotate when rotate=false")
	}
}

func TestLoggerConfigFromFlagsFileWithRotateEnabled(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "rotated.log")
	cfg := loggerConfigFromFlags("info", "json", "file", tmpFile, true, 50, 5, 7, false)
	if cfg.Rotate == nil {
		t.Fatal("expected non-nil Rotate")
	}
	if !cfg.Rotate.Enabled {
		t.Fatal("expected Enabled=true")
	}
	if cfg.Rotate.MaxSizeMB != 50 {
		t.Fatalf("expected MaxSizeMB=50, got %d", cfg.Rotate.MaxSizeMB)
	}
	if *cfg.Rotate.Compress != false {
		t.Fatal("expected Compress=false")
	}

	l, err := logger.NewLogger(cfg, "server")
	if err != nil {
		t.Fatalf("rotated file should succeed: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLoggerConfigFromFlagsRotateWithStderr(t *testing.T) {
	cfg := loggerConfigFromFlags("info", "json", "stderr", "", true, 100, 3, 30, true)
	_, err := logger.NewLogger(cfg, "server")
	if err == nil {
		t.Fatal("expected error for rotate with stderr")
	}
}

func TestLoggerConfigFromFlagsInvalidMaxSize(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")
	cfg := loggerConfigFromFlags("info", "json", "file", tmpFile, true, -1, 3, 30, true)
	_, err := logger.NewLogger(cfg, "server")
	if err == nil {
		t.Fatal("expected error for negative max size")
	}
}

func TestLoggerConfigFromFlagsFileWithoutRotateStillPlainAppend(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "plain.log")
	cfg := loggerConfigFromFlags("info", "json", "file", tmpFile, false, 100, 3, 30, true)
	l, err := logger.NewLogger(cfg, "server")
	if err != nil {
		t.Fatalf("plain file should succeed: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestValidateRotateFlagsRejectsZeroMaxSize(t *testing.T) {
	err := validateRotateFlags(true, 0, 3, 30)
	if err == nil {
		t.Fatal("expected error for zero max-size-mb with rotation enabled")
	}
	if !strings.Contains(err.Error(), "log-max-size-mb must be > 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRotateFlagsAllowsZeroMaxSizeWhenDisabled(t *testing.T) {
	err := validateRotateFlags(false, 0, 3, 30)
	if err != nil {
		t.Fatalf("expected nil error when rotate disabled, got: %v", err)
	}
}

func TestValidateRotateFlagsAcceptsValidValues(t *testing.T) {
	err := validateRotateFlags(true, 100, 3, 30)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestValidateRotateFlagsRejectsNegativeMaxBackups(t *testing.T) {
	err := validateRotateFlags(true, 100, -1, 30)
	if err == nil {
		t.Fatal("expected error for negative max-backups")
	}
}

func TestValidateRotateFlagsRejectsNegativeMaxAge(t *testing.T) {
	err := validateRotateFlags(true, 100, 3, -1)
	if err == nil {
		t.Fatal("expected error for negative max-age-days")
	}
}

func TestLoggerConfigFromRuntimeConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte(`
logging:
  level: debug
  format: text
  output: file
  file: `+filepath.Join(dir, "test.log")+`
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var level, format, output, file string
	var rotate bool
	var maxMB, maxBackups, maxAge int
	var compress bool
	fs.StringVar(&level, "log-level", "info", "")
	fs.StringVar(&format, "log-format", "json", "")
	fs.StringVar(&output, "log-output", "stderr", "")
	fs.StringVar(&file, "log-file", "", "")
	fs.BoolVar(&rotate, "log-rotate", false, "")
	fs.IntVar(&maxMB, "log-max-size-mb", 100, "")
	fs.IntVar(&maxBackups, "log-max-backups", 3, "")
	fs.IntVar(&maxAge, "log-max-age-days", 30, "")
	fs.BoolVar(&compress, "log-compress", true, "")
	_ = fs.Parse([]string{})

	cfg := loggerConfigFromRuntimeAndFlags(runtimeCfg, loggingFlagSet{
		level: level, format: format, output: output, file: file,
		rotate: rotate, maxMB: maxMB, maxBackups: maxBackups,
		maxAge: maxAge, compress: compress,
	}, fs)

	if cfg.Level != "debug" {
		t.Fatalf("expected debug level from runtime, got %q", cfg.Level)
	}
	if cfg.Format != "text" {
		t.Fatalf("expected text format from runtime, got %q", cfg.Format)
	}
	if cfg.Output != "file" {
		t.Fatalf("expected file output from runtime, got %q", cfg.Output)
	}

	l, err := logger.NewLogger(cfg, "server")
	if err != nil {
		t.Fatalf("expected valid logger: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLoggerFlagsOverrideRuntimeConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte(`
logging:
  level: debug
  format: text
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var level, format, output, file string
	var rotate bool
	var maxMB, maxBackups, maxAge int
	var compress bool
	fs.StringVar(&level, "log-level", "info", "")
	fs.StringVar(&format, "log-format", "json", "")
	fs.StringVar(&output, "log-output", "stderr", "")
	fs.StringVar(&file, "log-file", "", "")
	fs.BoolVar(&rotate, "log-rotate", false, "")
	fs.IntVar(&maxMB, "log-max-size-mb", 100, "")
	fs.IntVar(&maxBackups, "log-max-backups", 3, "")
	fs.IntVar(&maxAge, "log-max-age-days", 30, "")
	fs.BoolVar(&compress, "log-compress", true, "")
	_ = fs.Parse([]string{"--log-level", "warn"})

	cfg := loggerConfigFromRuntimeAndFlags(runtimeCfg, loggingFlagSet{
		level: level, format: format, output: output, file: file,
		rotate: rotate, maxMB: maxMB, maxBackups: maxBackups,
		maxAge: maxAge, compress: compress,
	}, fs)

	if cfg.Level != "warn" {
		t.Fatalf("expected warn level from explicit flag overriding runtime debug, got %q", cfg.Level)
	}
	if cfg.Format != "text" {
		t.Fatalf("expected text format from runtime (no explicit flag), got %q", cfg.Format)
	}
}

func TestRuntimeConfigLoadRejectsInvalidYAML(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(tmpFile, []byte("logging: ["), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := runtimeconfig.Load(tmpFile)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if !strings.Contains(err.Error(), "parse runtime config") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestRuntimeConfigRotateEnabledMapsToLoggerConfig(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "server.log")
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte(`
logging:
  output: file
  file: `+logFile+`
  rotate:
    enabled: true
    max_size_mb: 10
    max_backups: 2
    max_age_days: 7
    compress: false
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var level, format, output, file string
	var rotate bool
	var maxMB, maxBackups, maxAge int
	var compress bool
	fs.StringVar(&level, "log-level", "info", "")
	fs.StringVar(&format, "log-format", "json", "")
	fs.StringVar(&output, "log-output", "stderr", "")
	fs.StringVar(&file, "log-file", "", "")
	fs.BoolVar(&rotate, "log-rotate", false, "")
	fs.IntVar(&maxMB, "log-max-size-mb", 100, "")
	fs.IntVar(&maxBackups, "log-max-backups", 3, "")
	fs.IntVar(&maxAge, "log-max-age-days", 30, "")
	fs.BoolVar(&compress, "log-compress", true, "")
	_ = fs.Parse([]string{})

	cfg := loggerConfigFromRuntimeAndFlags(runtimeCfg, loggingFlagSet{
		level: level, format: format, output: output, file: file,
		rotate: rotate, maxMB: maxMB, maxBackups: maxBackups,
		maxAge: maxAge, compress: compress,
	}, fs)

	if cfg.Rotate == nil || !cfg.Rotate.Enabled {
		t.Fatal("expected rotate enabled from runtime config")
	}
	if cfg.Rotate.MaxSizeMB != 10 {
		t.Fatalf("expected MaxSizeMB=10, got %d", cfg.Rotate.MaxSizeMB)
	}
	if cfg.Rotate.MaxBackups != 2 {
		t.Fatalf("expected MaxBackups=2, got %d", cfg.Rotate.MaxBackups)
	}
	if cfg.Rotate.MaxAgeDays != 7 {
		t.Fatalf("expected MaxAgeDays=7, got %d", cfg.Rotate.MaxAgeDays)
	}
	if cfg.Rotate.Compress == nil || *cfg.Rotate.Compress != false {
		t.Fatalf("expected Compress=false, got %v", cfg.Rotate.Compress)
	}
}

func TestRuntimeConfigRotateDisabledIgnoresRotateFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte(`
logging:
  rotate:
    enabled: false
    max_size_mb: 10
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var level, format, output, file string
	var rotate bool
	var maxMB, maxBackups, maxAge int
	var compress bool
	fs.StringVar(&level, "log-level", "info", "")
	fs.StringVar(&format, "log-format", "json", "")
	fs.StringVar(&output, "log-output", "stderr", "")
	fs.StringVar(&file, "log-file", "", "")
	fs.BoolVar(&rotate, "log-rotate", false, "")
	fs.IntVar(&maxMB, "log-max-size-mb", 100, "")
	fs.IntVar(&maxBackups, "log-max-backups", 3, "")
	fs.IntVar(&maxAge, "log-max-age-days", 30, "")
	fs.BoolVar(&compress, "log-compress", true, "")
	_ = fs.Parse([]string{})

	cfg := loggerConfigFromRuntimeAndFlags(runtimeCfg, loggingFlagSet{
		level: level, format: format, output: output, file: file,
		rotate: rotate, maxMB: maxMB, maxBackups: maxBackups,
		maxAge: maxAge, compress: compress,
	}, fs)

	if cfg.Rotate != nil {
		t.Fatalf("expected nil Rotate when enabled=false, got %+v", cfg.Rotate)
	}
}

func TestParseMetadataConnectTimeoutFromRuntime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte("metadata:\n  connect_timeout: 7s\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	d, err := parseMetadataConnectTimeout(runtimeCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 7*time.Second {
		t.Fatalf("expected 7s, got %v", d)
	}
}

func TestParseMetadataConnectTimeoutRejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte("metadata:\n  connect_timeout: not-a-duration\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runtimeCfg, err := runtimeconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	_, err = parseMetadataConnectTimeout(runtimeCfg)
	if err == nil {
		t.Fatal("expected error for invalid metadata.connect_timeout")
	}
	if !strings.Contains(err.Error(), "metadata.connect_timeout") {
		t.Fatalf("expected metadata.connect_timeout error, got: %v", err)
	}
}
