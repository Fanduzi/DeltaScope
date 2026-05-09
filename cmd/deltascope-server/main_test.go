// Package main verifies CLI flag parsing helpers for the HTTP server entrypoint.
// input: raw comma-separated flag strings
// output: normalized token slices used by auth flag parsing
// pos: lightweight unit tests for command bootstrap helpers
// note: if this file changes, update this header and module README.md.
package main

import (
	"path/filepath"
	"reflect"
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
	cfg := loggerConfigFromFlags("info", "json", "stderr", "")
	l, err := logger.NewLogger(cfg, "server")
	if err != nil {
		t.Fatalf("default config should succeed: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLoggerConfigFromFlagsInvalidLevel(t *testing.T) {
	cfg := loggerConfigFromFlags("trace", "json", "stderr", "")
	_, err := logger.NewLogger(cfg, "server")
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestLoggerConfigFromFlagsFileOutput(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")
	cfg := loggerConfigFromFlags("info", "json", "file", tmpFile)
	l, err := logger.NewLogger(cfg, "server")
	if err != nil {
		t.Fatalf("file output should succeed: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLoggerConfigFromFlagsInvalidFormat(t *testing.T) {
	cfg := loggerConfigFromFlags("info", "xml", "stderr", "")
	_, err := logger.NewLogger(cfg, "server")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}
