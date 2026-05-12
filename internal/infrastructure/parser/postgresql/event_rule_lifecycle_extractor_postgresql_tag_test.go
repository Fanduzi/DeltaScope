//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type eventRuleTestCase struct {
	Name          string
	SQL           string
	WantKind      spec.Kind
	WantOperation spec.DDLOperation
	WantObjName   string
	WantObjType   string
	WantOptions   map[string]string
	BodyValues    []string // values that must NOT appear in normalized output
}

var eventRuleExtractorCases = []eventRuleTestCase{
	{
		Name:          "create_event_trigger",
		SQL:           "CREATE EVENT TRIGGER trg_ddl ON ddl_command_end EXECUTE FUNCTION log_ddl()",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationCreateEventTrigger,
		WantObjName:   "trg_ddl",
		WantObjType:   "event_trigger",
		WantOptions: map[string]string{
			"event":    "ddl_command_end",
			"function": "log_ddl",
		},
	},
	{
		Name:          "alter_event_trigger_disable",
		SQL:           "ALTER EVENT TRIGGER trg_ddl DISABLE",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationAlterEventTrigger,
		WantObjName:   "trg_ddl",
		WantObjType:   "event_trigger",
		WantOptions: map[string]string{
			"action": "disable",
		},
	},
	{
		Name:          "alter_event_trigger_enable",
		SQL:           "ALTER EVENT TRIGGER trg_ddl ENABLE",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationAlterEventTrigger,
		WantObjName:   "trg_ddl",
		WantObjType:   "event_trigger",
		WantOptions: map[string]string{
			"action": "enable",
		},
	},
	{
		Name:          "alter_event_trigger_rename",
		SQL:           "ALTER EVENT TRIGGER trg_ddl RENAME TO trg_ddl_v2",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationAlterEventTrigger,
		WantObjName:   "trg_ddl",
		WantObjType:   "event_trigger",
		WantOptions: map[string]string{
			"action":   "rename",
			"new_name": "trg_ddl_v2",
		},
	},
	{
		Name:          "drop_event_trigger",
		SQL:           "DROP EVENT TRIGGER trg_ddl",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationDropEventTrigger,
		WantObjName:   "trg_ddl",
		WantObjType:   "event_trigger",
		WantOptions: map[string]string{
			"if_exists": "false",
		},
	},
	{
		Name:          "drop_event_trigger_if_exists",
		SQL:           "DROP EVENT TRIGGER IF EXISTS trg_ddl",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationDropEventTrigger,
		WantObjName:   "trg_ddl",
		WantObjType:   "event_trigger",
		WantOptions: map[string]string{
			"if_exists": "true",
		},
	},
	{
		Name:          "create_rule_insert",
		SQL:           "CREATE RULE users_insert AS ON INSERT TO users DO NOTHING",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationCreateRule,
		WantObjName:   "users_insert",
		WantObjType:   "rule",
		WantOptions: map[string]string{
			"table": "users",
			"event": "insert",
		},
		BodyValues: []string{"DO NOTHING"},
	},
	{
		Name:          "alter_rule_rename",
		SQL:           "ALTER RULE users_insert ON users RENAME TO users_insert_ignore",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationAlterRule,
		WantObjName:   "users_insert",
		WantObjType:   "rule",
		WantOptions: map[string]string{
			"action":   "rename",
			"new_name": "users_insert_ignore",
			"table":    "users",
		},
	},
	{
		Name:          "drop_rule",
		SQL:           "DROP RULE users_insert_ignore ON users",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationDropRule,
		WantObjName:   "users_insert_ignore",
		WantObjType:   "rule",
		WantOptions: map[string]string{
			"if_exists": "false",
		},
	},
	{
		Name:          "drop_rule_if_exists",
		SQL:           "DROP RULE IF EXISTS users_insert_ignore ON users",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationDropRule,
		WantObjName:   "users_insert_ignore",
		WantObjType:   "rule",
		WantOptions: map[string]string{
			"if_exists": "true",
		},
	},
}

func TestEventRuleLifecycleExtractor(t *testing.T) {
	t.Parallel()

	for _, tc := range eventRuleExtractorCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			p := New()
			result, parseErr := p.Parse(context.Background(), tc.SQL)
			if parseErr != nil {
				t.Fatalf("parse failed: %v", parseErr)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			es := result.Statements[0]

			if es.Kind != tc.WantKind {
				t.Fatalf("expected Kind %s, got %s", tc.WantKind, es.Kind)
			}

			stmt, extractErr := es.Extractor.Extract(spec.DialectPostgreSQL, es.RawSQL)
			if extractErr != nil {
				t.Fatalf("extract failed: %v", extractErr)
			}
			if stmt.Unsupported != nil {
				t.Fatalf("expected normalized DDL, got unsupported: %s: %s",
					stmt.Unsupported.Feature, stmt.Unsupported.Reason)
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL, got nil")
			}

			if stmt.DDL.Operation != tc.WantOperation {
				t.Errorf("expected operation %q, got %q", tc.WantOperation, stmt.DDL.Operation)
			}
			if stmt.DDL.ObjectName != tc.WantObjName {
				t.Errorf("expected object_name %q, got %q", tc.WantObjName, stmt.DDL.ObjectName)
			}
			if stmt.DDL.ObjectType != tc.WantObjType {
				t.Errorf("expected object_type %q, got %q", tc.WantObjType, stmt.DDL.ObjectType)
			}
			for k, wantV := range tc.WantOptions {
				gotV, ok := stmt.DDL.Options[k]
				if !ok {
					t.Errorf("missing option %q", k)
					continue
				}
				if gotV != wantV {
					t.Errorf("option %q: expected %q, got %q", k, wantV, gotV)
				}
			}

			// Verify body/payload values do not leak into normalized output.
			for _, body := range tc.BodyValues {
				for k, v := range stmt.DDL.Options {
					if v == body {
						t.Errorf("Options[%q] leaks body value %q", k, body)
					}
				}
			}
		})
	}
}
