//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// TestPGExtractorAdvancedViewLifecycleDDL verifies that the 8 advanced view
// lifecycle SQL forms are correctly normalized by the extractor. These tests
// exercise:
//   - CREATE OR REPLACE VIEW → operation=create_view, options.replace=true
//   - CREATE TEMP VIEW → operation=create_view, options.temporary=true
//   - CREATE VIEW ... WITH CHECK OPTION → operation=create_view, options.check_option=local|cascaded
//   - ALTER VIEW ... RENAME TO → operation=alter_view, action=rename_view
//   - ALTER VIEW ... SET SCHEMA → operation=alter_view, action=set_schema
//   - DROP VIEW ... CASCADE → operation=drop_view, options.cascade=true
func TestPGExtractorAdvancedViewLifecycleDDL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name       string
		SQL        string
		Operation  spec.DDLOperation
		ObjectName string
		ObjectType string
		Options    map[string]string
	}{
		{
			Name:       "CREATE OR REPLACE VIEW",
			SQL:        "CREATE OR REPLACE VIEW v_user_stats AS SELECT COUNT(*) FROM users",
			Operation:  spec.DDLOperationCreateView,
			ObjectName: "v_user_stats",
			ObjectType: "view",
			Options:    map[string]string{"replace": "true"},
		},
		{
			Name:       "CREATE TEMP VIEW",
			SQL:        "CREATE TEMP VIEW v_session AS SELECT 1",
			Operation:  spec.DDLOperationCreateView,
			ObjectName: "v_session",
			ObjectType: "view",
			Options:    map[string]string{"temporary": "true"},
		},
		{
			Name:       "CREATE TEMPORARY VIEW",
			SQL:        "CREATE TEMPORARY VIEW v_tmp AS SELECT 1",
			Operation:  spec.DDLOperationCreateView,
			ObjectName: "v_tmp",
			ObjectType: "view",
			Options:    map[string]string{"temporary": "true"},
		},
		{
			Name:       "CREATE VIEW WITH CHECK OPTION",
			SQL:        "CREATE VIEW v_checked AS SELECT * FROM users WITH CHECK OPTION",
			Operation:  spec.DDLOperationCreateView,
			ObjectName: "v_checked",
			ObjectType: "view",
			Options:    map[string]string{"check_option": "true", "check_option_scope": "cascaded"},
		},
		{
			Name:       "CREATE VIEW WITH LOCAL CHECK OPTION",
			SQL:        "CREATE VIEW v_local AS SELECT * FROM users WITH LOCAL CHECK OPTION",
			Operation:  spec.DDLOperationCreateView,
			ObjectName: "v_local",
			ObjectType: "view",
			Options:    map[string]string{"check_option": "true", "check_option_scope": "local"},
		},
		{
			Name:       "CREATE VIEW WITH CASCADED CHECK OPTION",
			SQL:        "CREATE VIEW v_cascaded AS SELECT * FROM users WITH CASCADED CHECK OPTION",
			Operation:  spec.DDLOperationCreateView,
			ObjectName: "v_cascaded",
			ObjectType: "view",
			Options:    map[string]string{"check_option": "true", "check_option_scope": "cascaded"},
		},
		{
			Name:       "ALTER VIEW RENAME TO",
			SQL:        "ALTER VIEW v_old RENAME TO v_new",
			Operation:  spec.DDLOperationAlterView,
			ObjectName: "v_old",
			ObjectType: "view",
			Options:    map[string]string{"action": "rename_view", "new_name": "v_new"},
		},
		{
			Name:       "ALTER VIEW SET SCHEMA",
			SQL:        "ALTER VIEW v_stats SET SCHEMA archive",
			Operation:  spec.DDLOperationAlterView,
			ObjectName: "v_stats",
			ObjectType: "view",
			Options:    map[string]string{"action": "set_schema", "new_schema": "archive"},
		},
		{
			Name:       "DROP VIEW CASCADE",
			SQL:        "DROP VIEW v_stats CASCADE",
			Operation:  spec.DDLOperationDropView,
			ObjectName: "v_stats",
			ObjectType: "view",
			Options:    map[string]string{"cascade": "true"},
		},
		{
			Name:       "DROP VIEW IF EXISTS CASCADE",
			SQL:        "DROP VIEW IF EXISTS v_stats CASCADE",
			Operation:  spec.DDLOperationDropView,
			ObjectName: "v_stats",
			ObjectType: "view",
			Options:    map[string]string{"if_exists": "true", "cascade": "true"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			p := New()
			result, err := p.Parse(context.Background(), tc.SQL)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			stmt, extractErr := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
			if extractErr != nil {
				t.Fatalf("extract: %v", extractErr)
			}

			if stmt.Unsupported != nil {
				t.Fatalf("expected supported statement, got unsupported: %s: %s", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
			}
			if stmt.Kind != spec.KindDDL {
				t.Fatalf("expected kind DDL, got %q", stmt.Kind)
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL metadata")
			}
			if stmt.DDL.Operation != tc.Operation {
				t.Fatalf("expected operation %q, got %q", tc.Operation, stmt.DDL.Operation)
			}
			if stmt.DDL.ObjectName != tc.ObjectName {
				t.Fatalf("expected object_name %q, got %q", tc.ObjectName, stmt.DDL.ObjectName)
			}
			if stmt.DDL.ObjectType != tc.ObjectType {
				t.Fatalf("expected object_type %q, got %q", tc.ObjectType, stmt.DDL.ObjectType)
			}
			for k, v := range tc.Options {
				if got := stmt.DDL.Options[k]; got != v {
					t.Fatalf("expected options[%q] = %q, got %q", k, v, got)
				}
			}
		})
	}
}
