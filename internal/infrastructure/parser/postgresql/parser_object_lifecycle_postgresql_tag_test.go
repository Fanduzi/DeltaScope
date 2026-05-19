//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGExtractorExtractsObjectLifecycleDDL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name       string
		SQL        string
		Operation  spec.DDLOperation
		ObjectName string
		ObjectType string
		Options    map[string]string
		HasSelect  bool
	}{
		{
			Name:       "CREATE SCHEMA",
			SQL:        "CREATE SCHEMA staging",
			Operation:  spec.DDLOperationCreateSchema,
			ObjectName: "staging",
			ObjectType: "schema",
		},
		{
			Name:       "DROP SCHEMA IF EXISTS CASCADE",
			SQL:        "DROP SCHEMA IF EXISTS staging CASCADE",
			Operation:  spec.DDLOperationDropSchema,
			ObjectName: "staging",
			ObjectType: "schema",
			Options:    map[string]string{"if_exists": "true", "cascade": "true"},
		},
		{
			Name:       "CREATE SEQUENCE CYCLE",
			SQL:        "CREATE SEQUENCE seq_order_id CYCLE",
			Operation:  spec.DDLOperationCreateSequence,
			ObjectName: "seq_order_id",
			ObjectType: "sequence",
			Options:    map[string]string{"cycle": "true"},
		},
		{
			Name:       "ALTER SEQUENCE RESTART",
			SQL:        "ALTER SEQUENCE seq_order_id RESTART WITH 100",
			Operation:  spec.DDLOperationAlterSequence,
			ObjectName: "seq_order_id",
			ObjectType: "sequence",
			Options:    map[string]string{"restart": "true"},
		},
		{
			Name:       "DROP SEQUENCE IF EXISTS CASCADE",
			SQL:        "DROP SEQUENCE IF EXISTS seq_order_id CASCADE",
			Operation:  spec.DDLOperationDropSequence,
			ObjectName: "seq_order_id",
			ObjectType: "sequence",
			Options:    map[string]string{"if_exists": "true", "cascade": "true"},
		},
		{
			Name:       "CREATE MATERIALIZED VIEW WITH NO DATA",
			SQL:        "CREATE MATERIALIZED VIEW mv_stats AS SELECT COUNT(*) FROM users WITH NO DATA",
			Operation:  spec.DDLOperationCreateMaterializedView,
			ObjectName: "mv_stats",
			ObjectType: "materialized_view",
			Options:    map[string]string{"with_no_data": "true"},
			HasSelect:  true,
		},
		{
			Name:       "DROP MATERIALIZED VIEW IF EXISTS CASCADE",
			SQL:        "DROP MATERIALIZED VIEW IF EXISTS mv_stats CASCADE",
			Operation:  spec.DDLOperationDropMaterializedView,
			ObjectName: "mv_stats",
			ObjectType: "materialized_view",
			Options:    map[string]string{"if_exists": "true", "cascade": "true"},
		},
		{
			Name:       "REFRESH MATERIALIZED VIEW basic",
			SQL:        "REFRESH MATERIALIZED VIEW mv_stats",
			Operation:  spec.DDLOperationRefreshMaterializedView,
			ObjectName: "mv_stats",
			ObjectType: "materialized_view",
			Options:    map[string]string{"concurrently": "false", "with_no_data": "false"},
		},
		{
			Name:       "REFRESH MATERIALIZED VIEW CONCURRENTLY",
			SQL:        "REFRESH MATERIALIZED VIEW CONCURRENTLY mv_stats",
			Operation:  spec.DDLOperationRefreshMaterializedView,
			ObjectName: "mv_stats",
			ObjectType: "materialized_view",
			Options:    map[string]string{"concurrently": "true", "with_no_data": "false"},
		},
		{
			Name:       "REFRESH MATERIALIZED VIEW WITH DATA",
			SQL:        "REFRESH MATERIALIZED VIEW mv_stats WITH DATA",
			Operation:  spec.DDLOperationRefreshMaterializedView,
			ObjectName: "mv_stats",
			ObjectType: "materialized_view",
			Options:    map[string]string{"concurrently": "false", "with_no_data": "false"},
		},
		{
			Name:       "REFRESH MATERIALIZED VIEW WITH NO DATA",
			SQL:        "REFRESH MATERIALIZED VIEW mv_stats WITH NO DATA",
			Operation:  spec.DDLOperationRefreshMaterializedView,
			ObjectName: "mv_stats",
			ObjectType: "materialized_view",
			Options:    map[string]string{"concurrently": "false", "with_no_data": "true"},
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
			if stmt.DDL.HasSelect != tc.HasSelect {
				t.Fatalf("expected has_select %v, got %v", tc.HasSelect, stmt.DDL.HasSelect)
			}
			for k, v := range tc.Options {
				if got := stmt.DDL.Options[k]; got != v {
					t.Fatalf("expected options[%q] = %q, got %q", k, v, got)
				}
			}
		})
	}
}

func TestPGExtractorRefreshMaterializedViewNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "REFRESH MATERIALIZED VIEW mv_stats")
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
		t.Fatalf("expected normalized statement, got unsupported: %s: %s", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.Kind != spec.KindDDL {
		t.Fatalf("expected kind DDL, got %q", stmt.Kind)
	}
	if stmt.DDL == nil {
		t.Fatal("expected DDL metadata")
	}
	if stmt.DDL.Operation != spec.DDLOperationRefreshMaterializedView {
		t.Fatalf("expected operation %q, got %q", spec.DDLOperationRefreshMaterializedView, stmt.DDL.Operation)
	}
	if stmt.DDL.ObjectName != "mv_stats" {
		t.Fatalf("expected object_name %q, got %q", "mv_stats", stmt.DDL.ObjectName)
	}
	if stmt.DDL.ObjectType != "materialized_view" {
		t.Fatalf("expected object_type %q, got %q", "materialized_view", stmt.DDL.ObjectType)
	}
}

func TestPGExtractorAlterTableSetSchemaNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users SET SCHEMA archive")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	if result.Statements[0].Kind != spec.KindDDL {
		t.Fatalf("expected DDL kind, got %q", result.Statements[0].Kind)
	}

	stmt, extractErr := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if extractErr != nil {
		t.Fatalf("extract: %v", extractErr)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil {
		t.Fatal("expected DDL")
	}
	if stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got %q", stmt.DDL.Operation)
	}
	if stmt.DDL.Table == nil || stmt.DDL.Table.Name != "users" {
		t.Fatalf("expected table name 'users', got %v", stmt.DDL.Table)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "set_schema" {
		t.Fatalf("expected action 'set_schema', got %q", alter.Action)
	}
	if alter.Options["new_schema"] != "archive" {
		t.Fatalf("expected options['new_schema']='archive', got %q", alter.Options["new_schema"])
	}
}

func TestPGExtractorAlterTableOwnerToNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users OWNER TO app_owner")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if stmt.DDL.Table == nil || stmt.DDL.Table.Name != "users" {
		t.Fatalf("expected table name 'users', got %v", stmt.DDL.Table)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "change_owner" {
		t.Fatalf("expected action 'change_owner', got %q", alter.Action)
	}
	if alter.Options["owner"] != "app_owner" {
		t.Fatalf("expected options['owner']='app_owner', got %q", alter.Options["owner"])
	}
}

func TestPGExtractorAlterTableEnableTriggerNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users ENABLE TRIGGER trg_users_audit")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "enable_trigger" {
		t.Fatalf("expected action 'enable_trigger', got %q", alter.Action)
	}
	if alter.Name != "trg_users_audit" {
		t.Fatalf("expected name 'trg_users_audit', got %q", alter.Name)
	}
	if alter.Options["trigger"] != "trg_users_audit" {
		t.Fatalf("expected options['trigger']='trg_users_audit', got %q", alter.Options["trigger"])
	}
}

func TestPGExtractorAlterTableDisableTriggerNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users DISABLE TRIGGER trg_users_audit")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "disable_trigger" {
		t.Fatalf("expected action 'disable_trigger', got %q", alter.Action)
	}
	if alter.Name != "trg_users_audit" {
		t.Fatalf("expected name 'trg_users_audit', got %q", alter.Name)
	}
	if alter.Options["trigger"] != "trg_users_audit" {
		t.Fatalf("expected options['trigger']='trg_users_audit', got %q", alter.Options["trigger"])
	}
}

func TestPGExtractorAlterTableEnableTriggerAllNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users ENABLE TRIGGER ALL")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "enable_trigger" {
		t.Fatalf("expected action 'enable_trigger', got %q", alter.Action)
	}
	if alter.Options["trigger_scope"] != "all" {
		t.Fatalf("expected options['trigger_scope']='all', got %q", alter.Options["trigger_scope"])
	}
}

func TestPGExtractorAlterTableEnableTriggerUserNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users ENABLE TRIGGER USER")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "enable_trigger" {
		t.Fatalf("expected action 'enable_trigger', got %q", alter.Action)
	}
	if alter.Options["trigger_scope"] != "user" {
		t.Fatalf("expected options['trigger_scope']='user', got %q", alter.Options["trigger_scope"])
	}
}

func TestPGExtractorAlterTableDisableTriggerAllNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users DISABLE TRIGGER ALL")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "disable_trigger" {
		t.Fatalf("expected action 'disable_trigger', got %q", alter.Action)
	}
	if alter.Options["trigger_scope"] != "all" {
		t.Fatalf("expected options['trigger_scope']='all', got %q", alter.Options["trigger_scope"])
	}
}

func TestPGExtractorAlterTableDisableTriggerUserNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users DISABLE TRIGGER USER")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "disable_trigger" {
		t.Fatalf("expected action 'disable_trigger', got %q", alter.Action)
	}
	if alter.Options["trigger_scope"] != "user" {
		t.Fatalf("expected options['trigger_scope']='user', got %q", alter.Options["trigger_scope"])
	}
}

func TestPGExtractorAlterTableEnableReplicaTriggerNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users ENABLE REPLICA TRIGGER sync_trigger")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "enable_replica_trigger" {
		t.Fatalf("expected action 'enable_replica_trigger', got %q", alter.Action)
	}
	if alter.Name != "sync_trigger" {
		t.Fatalf("expected name 'sync_trigger', got %q", alter.Name)
	}
	if alter.Options["trigger"] != "sync_trigger" {
		t.Fatalf("expected options['trigger']='sync_trigger', got %q", alter.Options["trigger"])
	}
	if alter.Options["trigger_mode"] != "replica" {
		t.Fatalf("expected options['trigger_mode']='replica', got %q", alter.Options["trigger_mode"])
	}
}

func TestPGExtractorAlterTableEnableAlwaysTriggerNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users ENABLE ALWAYS TRIGGER audit_trigger")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "enable_always_trigger" {
		t.Fatalf("expected action 'enable_always_trigger', got %q", alter.Action)
	}
	if alter.Name != "audit_trigger" {
		t.Fatalf("expected name 'audit_trigger', got %q", alter.Name)
	}
	if alter.Options["trigger"] != "audit_trigger" {
		t.Fatalf("expected options['trigger']='audit_trigger', got %q", alter.Options["trigger"])
	}
	if alter.Options["trigger_mode"] != "always" {
		t.Fatalf("expected options['trigger_mode']='always', got %q", alter.Options["trigger_mode"])
	}
}

func TestPGExtractorAlterTableEnableRuleNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users ENABLE RULE route_rule")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "enable_rule" {
		t.Fatalf("expected action 'enable_rule', got %q", alter.Action)
	}
	if alter.Name != "route_rule" {
		t.Fatalf("expected name 'route_rule', got %q", alter.Name)
	}
	if alter.Options["rule"] != "route_rule" {
		t.Fatalf("expected options['rule']='route_rule', got %q", alter.Options["rule"])
	}
}

func TestPGExtractorAlterTableDisableRuleNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users DISABLE RULE route_rule")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "disable_rule" {
		t.Fatalf("expected action 'disable_rule', got %q", alter.Action)
	}
	if alter.Name != "route_rule" {
		t.Fatalf("expected name 'route_rule', got %q", alter.Name)
	}
	if alter.Options["rule"] != "route_rule" {
		t.Fatalf("expected options['rule']='route_rule', got %q", alter.Options["rule"])
	}
}

func TestPGExtractorAlterTableEnableReplicaRuleNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users ENABLE REPLICA RULE route_rule")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "enable_replica_rule" {
		t.Fatalf("expected action 'enable_replica_rule', got %q", alter.Action)
	}
	if alter.Name != "route_rule" {
		t.Fatalf("expected name 'route_rule', got %q", alter.Name)
	}
	if alter.Options["rule"] != "route_rule" {
		t.Fatalf("expected options['rule']='route_rule', got %q", alter.Options["rule"])
	}
	if alter.Options["rule_mode"] != "replica" {
		t.Fatalf("expected options['rule_mode']='replica', got %q", alter.Options["rule_mode"])
	}
}

func TestPGExtractorAlterTableEnableAlwaysRuleNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users ENABLE ALWAYS RULE route_rule")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "enable_always_rule" {
		t.Fatalf("expected action 'enable_always_rule', got %q", alter.Action)
	}
	if alter.Name != "route_rule" {
		t.Fatalf("expected name 'route_rule', got %q", alter.Name)
	}
	if alter.Options["rule"] != "route_rule" {
		t.Fatalf("expected options['rule']='route_rule', got %q", alter.Options["rule"])
	}
	if alter.Options["rule_mode"] != "always" {
		t.Fatalf("expected options['rule_mode']='always', got %q", alter.Options["rule_mode"])
	}
}

func TestPGExtractorAlterTableReplicaIdentityDefaultNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users REPLICA IDENTITY DEFAULT")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "replica_identity" {
		t.Fatalf("expected action 'replica_identity', got %q", alter.Action)
	}
	if alter.Options["identity"] != "default" {
		t.Fatalf("expected options['identity']='default', got %q", alter.Options["identity"])
	}
}

func TestPGExtractorAlterTableReplicaIdentityFullNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users REPLICA IDENTITY FULL")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "replica_identity" {
		t.Fatalf("expected action 'replica_identity', got %q", alter.Action)
	}
	if alter.Options["identity"] != "full" {
		t.Fatalf("expected options['identity']='full', got %q", alter.Options["identity"])
	}
}

func TestPGExtractorAlterTableReplicaIdentityNothingNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users REPLICA IDENTITY NOTHING")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "replica_identity" {
		t.Fatalf("expected action 'replica_identity', got %q", alter.Action)
	}
	if alter.Options["identity"] != "nothing" {
		t.Fatalf("expected options['identity']='nothing', got %q", alter.Options["identity"])
	}
}

func TestPGExtractorAlterTableReplicaIdentityUsingIndexNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE users REPLICA IDENTITY USING INDEX users_replica_identity_idx")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "replica_identity" {
		t.Fatalf("expected action 'replica_identity', got %q", alter.Action)
	}
	if alter.Options["identity"] != "using_index" {
		t.Fatalf("expected options['identity']='using_index', got %q", alter.Options["identity"])
	}
	if alter.Options["index"] != "users_replica_identity_idx" {
		t.Fatalf("expected options['index']='users_replica_identity_idx', got %q", alter.Options["index"])
	}
	if alter.Name != "users_replica_identity_idx" {
		t.Fatalf("expected name 'users_replica_identity_idx', got %q", alter.Name)
	}
}

func TestPGExtractorAlterTableAttachPartitionNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE measurement ATTACH PARTITION measurement_y2026m04 FOR VALUES FROM ('2026-04-01') TO ('2026-05-01')")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if stmt.DDL.Table == nil || stmt.DDL.Table.Name != "measurement" {
		t.Fatalf("expected table name 'measurement', got %v", stmt.DDL.Table)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "attach_partition" {
		t.Fatalf("expected action 'attach_partition', got %q", alter.Action)
	}
	if alter.Name != "measurement_y2026m04" {
		t.Fatalf("expected name 'measurement_y2026m04', got %q", alter.Name)
	}
	if alter.Options["has_bounds"] != "true" {
		t.Fatalf("expected options['has_bounds']='true', got %q", alter.Options["has_bounds"])
	}
}

func TestPGExtractorAlterTableDetachPartitionNormalized(t *testing.T) {
	t.Parallel()
	p := New()
	result, err := p.Parse(context.Background(), "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04")
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
		t.Fatalf("expected no unsupported, got feature=%q reason=%q", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got DDL=%v", stmt.DDL)
	}
	if stmt.DDL.Table == nil || stmt.DDL.Table.Name != "measurement" {
		t.Fatalf("expected table name 'measurement', got %v", stmt.DDL.Table)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "detach_partition" {
		t.Fatalf("expected action 'detach_partition', got %q", alter.Action)
	}
	if alter.Name != "measurement_y2026m04" {
		t.Fatalf("expected name 'measurement_y2026m04', got %q", alter.Name)
	}
}

func TestPGExtractorSchemaLifecycleCreateSchemaNormalized(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name        string
		SQL         string
		ObjectName  string
		ObjectType  string
		IfNotExists bool
	}{
		{Name: "CREATE SCHEMA", SQL: "CREATE SCHEMA app", ObjectName: "app", ObjectType: "schema"},
		{Name: "CREATE SCHEMA IF NOT EXISTS", SQL: "CREATE SCHEMA IF NOT EXISTS app", ObjectName: "app", ObjectType: "schema", IfNotExists: true},
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
				t.Fatalf("expected supported, got unsupported: %s: %s", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
			}
			if stmt.Kind != spec.KindDDL {
				t.Fatalf("expected kind DDL, got %q", stmt.Kind)
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL metadata")
			}
			if stmt.DDL.Operation != spec.DDLOperationCreateSchema {
				t.Fatalf("expected operation create_schema, got %q", stmt.DDL.Operation)
			}
			if stmt.DDL.ObjectName != tc.ObjectName {
				t.Fatalf("expected object_name %q, got %q", tc.ObjectName, stmt.DDL.ObjectName)
			}
			if stmt.DDL.ObjectType != tc.ObjectType {
				t.Fatalf("expected object_type %q, got %q", tc.ObjectType, stmt.DDL.ObjectType)
			}
			if tc.IfNotExists && stmt.DDL.Options["if_not_exists"] != "true" {
				t.Fatalf("expected if_not_exists=true, got %q", stmt.DDL.Options["if_not_exists"])
			}
			if !tc.IfNotExists && stmt.DDL.Options["if_not_exists"] == "true" {
				t.Fatal("did not expect if_not_exists=true")
			}
		})
	}
}

func TestPGExtractorSchemaLifecycleUnsupportedBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name            string
		SQL             string
		ExpectedFeature string
	}{
		{Name: "CREATE SCHEMA AUTHORIZATION", SQL: "CREATE SCHEMA AUTHORIZATION app_owner", ExpectedFeature: "create_schema_authorization"},
		{Name: "CREATE SCHEMA name AUTHORIZATION", SQL: "CREATE SCHEMA app AUTHORIZATION app_owner", ExpectedFeature: "create_schema_authorization"},
		{Name: "CREATE SCHEMA nested table", SQL: "CREATE SCHEMA app CREATE TABLE users (id bigint)", ExpectedFeature: "create_schema_nested_objects"},
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
			if stmt.Unsupported == nil {
				t.Fatal("expected unsupported statement")
			}
			if stmt.Unsupported.Feature != tc.ExpectedFeature {
				t.Fatalf("expected unsupported feature %q, got %q", tc.ExpectedFeature, stmt.Unsupported.Feature)
			}
		})
	}
}
