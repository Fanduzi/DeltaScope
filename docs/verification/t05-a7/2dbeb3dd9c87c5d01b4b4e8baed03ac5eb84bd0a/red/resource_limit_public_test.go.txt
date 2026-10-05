// Package audit_test verifies the public ordered audit resource boundary.
// input: short MySQL/TiDB batches through the existing AuditSQL entrypoint
// output: retained statement identity and default-budget unsupported assertions
// pos: public application regression for T05-A7 before and after implementation
// note: if this file changes, update this header and module README.md.
package audit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestT05A7AuditSQLDefaultBudget(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			sql := "CREATE TABLE t (id INT PRIMARY KEY);\nALTER TABLE t ADD COLUMN c INT;\n" + strings.Repeat("SELECT 1;\n", 1022) + "CREATE INDEX idx_c ON t(c);\n"
			result, err := audit.AuditSQL(context.Background(), audit.Request{SQL: sql, Dialect: dialect})
			if len(result.Statements) != 1025 {
				t.Fatalf("retained statements = %d, want 1025", len(result.Statements))
			}
			var resources []spec.UnsupportedDetail
			for _, item := range result.Unsupported {
				if item.Feature == "audit.resource_limit" {
					resources = append(resources, item)
				}
			}
			if len(resources) != 1 {
				t.Fatalf("resource entries = %d, want 1; final coverage=%s aggregate=%s err=%v", len(resources), result.Statements[1024].Coverage.Status, result.Coverage.Status, err)
			}
			if !errors.Is(err, audit.ErrUnsupportedStatement) {
				t.Fatalf("error = %v, want ErrUnsupportedStatement", err)
			}
			last := result.Statements[1024]
			if last.Index != 1024 || last.Kind != "ddl" || strings.TrimSpace(last.RawSQL) != "CREATE INDEX idx_c ON t(c);" || last.NormalizedSQL == "" || last.Coverage.Status != report.CoverageIncomplete || len(last.Findings) != 0 || len(last.EvidenceGaps) != 0 || last.Impact != nil {
				t.Fatalf("blocked statement = %+v", last)
			}
			item := resources[0]
			if item.Index != 1024 || item.Reason != "ordered-state statement budget exhausted" || item.SQL != last.RawSQL {
				t.Fatalf("resource identity = %+v", item)
			}
			want := map[string]any{"phase": "ordered_state", "resource": "statements", "limit": 1024, "consumed": 1024, "line": 1025, "column": 1}
			if len(item.Metadata) != len(want) {
				t.Fatalf("resource metadata = %+v, want %+v", item.Metadata, want)
			}
			for key, value := range want {
				if item.Metadata[key] != value {
					t.Errorf("resource metadata[%s] = %#v, want %#v", key, item.Metadata[key], value)
				}
			}
		})
	}
}
