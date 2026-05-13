//go:build postgresql

package postgresql

import (
	"context"
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type annotationTestCase struct {
	Name          string
	SQL           string
	WantKind      spec.Kind
	WantOperation spec.DDLOperation
	WantObjName   string
	WantObjType   string
	WantOptions   map[string]string
	SecretValues  []string
}

var annotationBaselineCensusCases = []annotationTestCase{
	{
		Name:          "comment_on_table",
		SQL:           "COMMENT ON TABLE users IS 'user accounts'",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationCommentOn,
		WantObjName:   "users",
		WantObjType:   "comment",
		WantOptions: map[string]string{
			"target_type": "table",
			"target_name": "users",
			"is_null":     "false",
		},
		SecretValues: []string{"user accounts"},
	},
	{
		Name:          "comment_on_table_null",
		SQL:           "COMMENT ON TABLE users IS NULL",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationCommentOn,
		WantObjName:   "users",
		WantObjType:   "comment",
		WantOptions: map[string]string{
			"target_type": "table",
			"target_name": "users",
			"is_null":     "true",
		},
	},
	{
		Name:          "security_label_on_table",
		SQL:           "SECURITY LABEL FOR selinux ON TABLE users IS 'system_u:object_r:sepgsql_table_t:s0'",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationSecurityLabel,
		WantObjName:   "users",
		WantObjType:   "security_label",
		WantOptions: map[string]string{
			"target_type": "table",
			"target_name": "users",
			"provider":    "selinux",
			"is_null":     "false",
		},
		SecretValues: []string{"system_u:object_r:sepgsql_table_t:s0"},
	},
	{
		Name:          "security_label_on_table_null",
		SQL:           "SECURITY LABEL FOR selinux ON TABLE users IS NULL",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationSecurityLabel,
		WantObjName:   "users",
		WantObjType:   "security_label",
		WantOptions: map[string]string{
			"target_type": "table",
			"target_name": "users",
			"provider":    "selinux",
			"is_null":     "true",
		},
	},
}

func TestAnnotationLifecycleExtractor(t *testing.T) {
	t.Parallel()

	for _, tc := range annotationBaselineCensusCases {
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

			// Verify no secret values leak into options or fields.
			for _, secret := range tc.SecretValues {
				for k, v := range stmt.DDL.Options {
					if v == secret {
						t.Errorf("Options[%q] leaks secret value %q", k, secret)
					}
				}
				if stmt.DDL.ObjectName == secret {
					t.Errorf("ObjectName leaks secret value %q", secret)
				}
				if stmt.DDL.ObjectType == secret {
					t.Errorf("ObjectType leaks secret value %q", secret)
				}
				if fmt.Sprintf("%v", stmt.DDL.Options) == secret {
					t.Errorf("Options leak secret value %q", secret)
				}
			}
		})
	}
}

// --- Schema-qualified name regression tests (v0.90.0 Task 7) ---
// objectNameFromNode must return the last name part from qualified names.

func TestCommentOnSchemaQualifiedReturnsLastName(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "COMMENT ON TABLE app.users IS 'test comment'")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.DDL.ObjectName != "users" {
		t.Fatalf("expected object_name users, got %q", stmt.DDL.ObjectName)
	}
}

func TestCommentOnUnqualifiedNameUnchanged(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "COMMENT ON TABLE users IS 'test comment'")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.DDL.ObjectName != "users" {
		t.Fatalf("expected object_name users, got %q", stmt.DDL.ObjectName)
	}
}

func TestSecurityLabelSchemaQualifiedReturnsLastName(t *testing.T) {
	t.Parallel()
	parser := New()
	result, err := parser.Parse(context.Background(), "SECURITY LABEL FOR selinux ON TABLE app.users IS 'system_u:object_r:sepgsql_table_t:s0'")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.DDL.ObjectName != "users" {
		t.Fatalf("expected object_name users, got %q", stmt.DDL.ObjectName)
	}
}
