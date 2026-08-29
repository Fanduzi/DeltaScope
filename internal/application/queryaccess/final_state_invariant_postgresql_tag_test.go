//go:build postgresql

// Package queryaccess_test verifies the shared Query Access result invariant.
// input: dialect-specific SQL and optional relation metadata
// output: stable read classification, admission, and reason-code assertions
// pos: application seam regression coverage for cross-surface Query Access state
// note: if this file changes, update this header and module README.md.
package queryaccess_test

import (
	"context"
	"testing"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestServiceAnalyzeFinalStateInvariant(t *testing.T) {
	tests := []struct {
		name       string
		dialect    string
		sql        string
		metadata   bool
		wantClass  domain.ReadClassification
		wantAdmit  domain.Admission
	}{
		{"mysql/offline/read_only", "mysql", "SELECT id FROM app.users", false, domain.ReadOnly, domain.Admissible},
		{"mysql/metadata/read_only", "mysql", "SELECT id FROM app.users", true, domain.ReadOnly, domain.Admissible},
		{"tidb/offline/read_only", "tidb", "SELECT id FROM app.users", false, domain.ReadOnly, domain.Admissible},
		{"tidb/metadata/read_only", "tidb", "SELECT id FROM app.users", true, domain.ReadOnly, domain.Admissible},
		{"postgresql/offline/read_only", "postgresql", "SELECT id FROM app.users", false, domain.ReadOnly, domain.Admissible},
		{"postgresql/metadata/read_only", "postgresql", "SELECT id FROM app.users", true, domain.ReadOnly, domain.Admissible},
		{"mysql/offline/indeterminate", "mysql", "SELECT * FROM WHERE", false, domain.Indeterminate, domain.IndeterminateAdmission},
		{"mysql/metadata/indeterminate", "mysql", "SELECT * FROM WHERE", true, domain.Indeterminate, domain.IndeterminateAdmission},
		{"tidb/offline/indeterminate", "tidb", "SELECT * FROM WHERE", false, domain.Indeterminate, domain.IndeterminateAdmission},
		{"tidb/metadata/indeterminate", "tidb", "SELECT * FROM WHERE", true, domain.Indeterminate, domain.IndeterminateAdmission},
		{"postgresql/offline/indeterminate", "postgresql", "SELECT * FROM WHERE", false, domain.Indeterminate, domain.IndeterminateAdmission},
		{"postgresql/metadata/indeterminate", "postgresql", "SELECT * FROM WHERE", true, domain.Indeterminate, domain.IndeterminateAdmission},
		{"mysql/offline/not_read_only", "mysql", "DELETE FROM app.users", false, domain.NotReadOnly, domain.Rejected},
		{"mysql/metadata/not_read_only", "mysql", "DELETE FROM app.users", true, domain.NotReadOnly, domain.Rejected},
		{"tidb/offline/not_read_only", "tidb", "DELETE FROM app.users", false, domain.NotReadOnly, domain.Rejected},
		{"tidb/metadata/not_read_only", "tidb", "DELETE FROM app.users", true, domain.NotReadOnly, domain.Rejected},
		{"postgresql/offline/not_read_only", "postgresql", "DELETE FROM app.users", false, domain.NotReadOnly, domain.Rejected},
		{"postgresql/metadata/not_read_only", "postgresql", "DELETE FROM app.users", true, domain.NotReadOnly, domain.Rejected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resolver appqa.SchemaResolver
			if tt.metadata {
				resolver = newFakeResolver(map[string]appqa.RelationSchema{
					"app.users": {
						Schema: "app", Name: "users", Kind: "table",
						Columns: []appqa.ColumnSchema{{Name: "id", Ordinal: 1}},
					},
				})
			}

			result, err := (&appqa.Service{}).Analyze(context.Background(), appqa.QueryAccessRequest{
				SQL:            tt.sql,
				Dialect:        tt.dialect,
				Mode:           "strict",
				SchemaResolver: resolver,
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}

			dr := result.DomainResult
			if dr.ReadClassification != tt.wantClass {
				t.Errorf("classification: got %q, want %q", dr.ReadClassification, tt.wantClass)
			}
			if dr.Admission != tt.wantAdmit {
				t.Errorf("admission: got %q, want %q", dr.Admission, tt.wantAdmit)
			}
			if dr.ReadClassification == domain.ReadOnly && dr.Admission != domain.Admissible {
				t.Errorf("read_only result must be admissible, got %q", dr.Admission)
			}
			if dr.Admission == domain.IndeterminateAdmission {
				if dr.ReadClassification != domain.Indeterminate {
					t.Errorf("indeterminate admission must have indeterminate classification, got %q", dr.ReadClassification)
				}
				if len(dr.ReasonCodes) == 0 {
					t.Error("indeterminate admission must include a reason code")
				}
			}
		})
	}
}
