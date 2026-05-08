// Package main verifies CLI flag parsing helpers for the HTTP server entrypoint.
// input: raw comma-separated flag strings
// output: normalized token slices used by auth flag parsing
// pos: lightweight unit tests for command bootstrap helpers
// note: if this file changes, update this header and module README.md.
package main

import (
	"reflect"
	"testing"
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
