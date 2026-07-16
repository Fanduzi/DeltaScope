//go:build postgresql

// Package deltascope verifies the public pure-effect admission matrix.
// input: default SDK analysis for PostgreSQL, MySQL, and TiDB function SQL
// output: trusted-only PostgreSQL promotion boundary and bounded public JSON
// pos: cross-surface SDK contract coverage
package deltascope

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestQueryAccessPureEffectSurfaceContract(t *testing.T) {
	cases := []struct {
		name       string
		dialect    Dialect
		wantReason string
	}{
		{name: "default postgresql remains indeterminate", dialect: DialectPostgreSQL, wantReason: "unproven_function_effect"},
		{name: "mysql remains deferred", dialect: DialectMySQL, wantReason: "unknown_function_effect"},
		{name: "tidb remains deferred", dialect: DialectTiDB, wantReason: "unknown_function_effect"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given: the default SDK path without a trusted PostgreSQL session.
			result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
				SQL:     "SELECT COUNT(*) FROM public.users WHERE id = 424242",
				Dialect: tc.dialect,
				Mode:    QueryAccessModeStrict,
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}

			// When: a function-bearing query is analyzed on each surface dialect.
			// Then: only the trusted SDK session can later promote PostgreSQL.
			if result.ReadClassification != QueryAccessIndeterminate || result.Admission != QueryAccessIndeterminateAdmission {
				t.Fatalf("classification/admission = %q/%q, want indeterminate/indeterminate", result.ReadClassification, result.Admission)
			}
			if !containsQueryAccessReason(result.ReasonCodes, tc.wantReason) {
				t.Fatalf("reason codes = %v, want %q", result.ReasonCodes, tc.wantReason)
			}

			data, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			output := string(data)
			for _, forbidden := range []string{"severity", "oid", "424242", "SELECT"} {
				if strings.Contains(output, forbidden) {
					t.Errorf("JSON leaked %q: %s", forbidden, output)
				}
			}
		})
	}
}

func containsQueryAccessReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
