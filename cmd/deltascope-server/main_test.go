// Package main verifies CLI flag parsing helpers for the HTTP server entrypoint.
// input: raw comma-separated flag strings
// output: normalized token slices used by auth flag parsing
// pos: lightweight unit tests for command bootstrap helpers
// note: if this file changes, update this header and module README.md.
package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/infrastructure/logger"
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
