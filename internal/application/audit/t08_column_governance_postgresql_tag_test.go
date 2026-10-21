//go:build postgresql

// Package audit verifies the T08 MySQL-family rule bindings stay inert on
// PostgreSQL statements.
// input: a PostgreSQL ALTER COLUMN setter against a present provider
// snapshot
// output: no mysql-family existence or position findings on the
// PostgreSQL action vocabulary (set_default), while the PG-side rules keep
// their own contract
// pos: T08 PostgreSQL boundary regression (issue #87)
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLT08PGRulesStayInert(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;",
		Dialect: spec.DialectPostgreSQL,
		Schema:  "public",
		MetadataProvider: &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"public.t": {
				Exists:  true,
				Schema:  "public",
				Table:   &spec.Table{Schema: "public", Name: "t"},
				Columns: []spec.Column{{Name: "c", Type: "int"}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.alter.set_default.exists.require" ||
			f.RuleID == "ddl.alter.drop_default.exists.require" ||
			f.RuleID == "ddl.alter.column_position.exists.require" {
			t.Fatalf("mysql-family rule fired on postgresql: %+v", f)
		}
	}
}
