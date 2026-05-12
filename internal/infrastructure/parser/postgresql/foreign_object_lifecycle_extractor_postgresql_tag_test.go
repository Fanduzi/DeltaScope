//go:build postgresql

package postgresql

import (
	"context"
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// foreignObjectTestCase defines one SQL statement to verify against the extractor.
type foreignObjectTestCase struct {
	Name          string
	SQL           string
	WantKind      spec.Kind
	WantOperation spec.DDLOperation
	WantObjName   string
	WantObjType   string
	WantOptions   map[string]string
	// SecretValues lists option values that must NOT appear anywhere in the
	// DDL.Options map. Used for security verification.
	SecretValues []string
}

// foreignObjectBaselineCensusCases covers the 12 canonical SQL forms plus edge cases.
var foreignObjectBaselineCensusCases = []foreignObjectTestCase{
	// ── Foreign Table lifecycle ──────────────────────────────────────────
	{
		Name:          "create_foreign_table",
		SQL:           "CREATE FOREIGN TABLE ft_users (id bigint) SERVER srv OPTIONS (table_name 'users')",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationCreateForeignTable,
		WantObjName:   "ft_users",
		WantObjType:   "foreign_table",
		WantOptions: map[string]string{
			"server":      "srv",
			"has_options": "true",
		},
		SecretValues: []string{"users"},
	},
	{
		Name:          "alter_foreign_table_options",
		SQL:           "ALTER FOREIGN TABLE ft_users OPTIONS (SET table_name 'users_v2')",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationAlterForeignTable,
		WantObjName:   "ft_users",
		WantObjType:   "foreign_table",
		WantOptions: map[string]string{
			"if_exists": "false",
			"action":    "alter_options",
		},
		SecretValues: []string{"users_v2"},
	},
	{
		Name:          "drop_foreign_table",
		SQL:           "DROP FOREIGN TABLE ft_users",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationDropForeignTable,
		WantObjName:   "ft_users",
		WantObjType:   "foreign_table",
		WantOptions: map[string]string{
			"if_exists": "false",
		},
	},
	{
		Name:          "drop_foreign_table_if_exists",
		SQL:           "DROP FOREIGN TABLE IF EXISTS ft_users",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationDropForeignTable,
		WantObjName:   "ft_users",
		WantObjType:   "foreign_table",
		WantOptions: map[string]string{
			"if_exists": "true",
		},
	},
	{
		Name:          "create_foreign_table_without_options",
		SQL:           "CREATE FOREIGN TABLE ft_logs (id bigint) SERVER srv",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationCreateForeignTable,
		WantObjName:   "ft_logs",
		WantObjType:   "foreign_table",
		WantOptions: map[string]string{
			"server": "srv",
		},
	},

	// ── Foreign Server lifecycle ─────────────────────────────────────────
	{
		Name:          "create_server",
		SQL:           "CREATE SERVER srv FOREIGN DATA WRAPPER postgres_fdw",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationCreateForeignServer,
		WantObjName:   "srv",
		WantObjType:   "foreign_server",
		WantOptions: map[string]string{
			"foreign_data_wrapper": "postgres_fdw",
		},
	},
	{
		Name:          "alter_server_options",
		SQL:           "ALTER SERVER srv OPTIONS (SET host 'db')",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationAlterForeignServer,
		WantObjName:   "srv",
		WantObjType:   "foreign_server",
		WantOptions: map[string]string{
			"has_options": "true",
			"action":      "alter_options",
		},
		SecretValues: []string{"db"},
	},
	{
		Name:          "drop_server",
		SQL:           "DROP SERVER srv",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationDropForeignServer,
		WantObjName:   "srv",
		WantObjType:   "foreign_server",
		WantOptions: map[string]string{
			"if_exists": "false",
		},
	},
	{
		Name:          "drop_server_if_exists_cascade",
		SQL:           "DROP SERVER IF EXISTS srv CASCADE",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationDropForeignServer,
		WantObjName:   "srv",
		WantObjType:   "foreign_server",
		WantOptions: map[string]string{
			"if_exists": "true",
			"cascade":   "true",
		},
	},
	{
		Name:          "create_server_with_options",
		SQL:           "CREATE SERVER srv_opts FOREIGN DATA WRAPPER postgres_fdw OPTIONS (host 'dbhost', port '5432')",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationCreateForeignServer,
		WantObjName:   "srv_opts",
		WantObjType:   "foreign_server",
		WantOptions: map[string]string{
			"foreign_data_wrapper": "postgres_fdw",
			"has_options":          "true",
		},
		SecretValues: []string{"dbhost", "5432"},
	},

	// ── User Mapping lifecycle ──────────────────────────────────────────
	{
		Name:          "create_user_mapping",
		SQL:           "CREATE USER MAPPING FOR app_role SERVER srv OPTIONS (user 'secret_user', password 's3cret')",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationCreateUserMapping,
		WantObjName:   "app_role@srv",
		WantObjType:   "user_mapping",
		WantOptions: map[string]string{
			"server":      "srv",
			"user":        "app_role",
			"has_options": "true",
		},
		// The OPTIONS values (secret_user, s3cret) are credential secrets
		// that must NOT leak. The 'user' option records the RoleSpec
		// rolename from FOR <role>, not the OPTIONS credential.
		SecretValues: []string{"secret_user", "s3cret"},
	},
	{
		Name:          "alter_user_mapping",
		SQL:           "ALTER USER MAPPING FOR app_role SERVER srv OPTIONS (SET user 'new_secret_user')",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationAlterUserMapping,
		WantObjName:   "app_role@srv",
		WantObjType:   "user_mapping",
		WantOptions: map[string]string{
			"server":      "srv",
			"user":        "app_role",
			"has_options": "true",
			"action":      "alter_options",
		},
		SecretValues: []string{"new_secret_user"},
	},
	{
		Name:          "drop_user_mapping",
		SQL:           "DROP USER MAPPING FOR app_role SERVER srv",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationDropUserMapping,
		WantObjName:   "app_role@srv",
		WantObjType:   "user_mapping",
		WantOptions: map[string]string{
			"if_exists": "false",
			"server":    "srv",
			"user":      "app_role",
		},
	},

	// ── Foreign Data Wrapper lifecycle ───────────────────────────────────
	{
		Name:          "create_fdw",
		SQL:           "CREATE FOREIGN DATA WRAPPER fdw HANDLER fdw_handler",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationCreateForeignDataWrapper,
		WantObjName:   "fdw",
		WantObjType:   "foreign_data_wrapper",
		WantOptions: map[string]string{
			"has_handler": "true",
		},
	},
	{
		Name:          "alter_fdw_options",
		SQL:           "ALTER FOREIGN DATA WRAPPER fdw OPTIONS (SET key 'value')",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationAlterForeignDataWrapper,
		WantObjName:   "fdw",
		WantObjType:   "foreign_data_wrapper",
		WantOptions: map[string]string{
			"has_options": "true",
			"action":      "alter_options",
		},
		SecretValues: []string{"value"},
	},
	{
		Name:          "drop_fdw",
		SQL:           "DROP FOREIGN DATA WRAPPER fdw",
		WantKind:      spec.KindDDL,
		WantOperation: spec.DDLOperationDropForeignDataWrapper,
		WantObjName:   "fdw",
		WantObjType:   "foreign_data_wrapper",
		WantOptions: map[string]string{
			"if_exists": "false",
		},
	},
}

func TestForeignObjectLifecycleExtractor(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== PostgreSQL Foreign Object Lifecycle Extractor Tests ===")
	t.Logf("%-40s | %-10s | %-28s | %-20s | %s",
		"Case", "Kind", "Operation", "ObjectName", "Detail")
	t.Log(string(make([]byte, 180)))

	for _, tc := range foreignObjectBaselineCensusCases {
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

			// Verify Kind classification.
			if es.Kind != tc.WantKind {
				t.Fatalf("expected Kind %s, got %s", tc.WantKind, es.Kind)
			}

			// Extract the normalized statement.
			stmt, extractErr := es.Extractor.Extract(spec.DialectPostgreSQL, es.RawSQL)
			if extractErr != nil {
				t.Fatalf("extract failed: %v", extractErr)
			}

			// Must not be unsupported.
			if stmt.Unsupported != nil {
				t.Fatalf("expected normalized DDL, got unsupported: %s: %s",
					stmt.Unsupported.Feature, stmt.Unsupported.Reason)
			}

			// DDL must be present.
			if stmt.DDL == nil {
				t.Fatal("expected DDL, got nil")
			}

			detail := fmt.Sprintf("op=%s obj=%q type=%q opts=%v",
				stmt.DDL.Operation, stmt.DDL.ObjectName, stmt.DDL.ObjectType, stmt.DDL.Options)
			t.Logf("%-40s | %-10s | %-28s | %-20s | %s",
				tc.Name, es.Kind, stmt.DDL.Operation, stmt.DDL.ObjectName, detail)

			// Verify operation.
			if stmt.DDL.Operation != tc.WantOperation {
				t.Errorf("expected operation %q, got %q", tc.WantOperation, stmt.DDL.Operation)
			}

			// Verify object name.
			if stmt.DDL.ObjectName != tc.WantObjName {
				t.Errorf("expected object_name %q, got %q", tc.WantObjName, stmt.DDL.ObjectName)
			}

			// Verify object type.
			if stmt.DDL.ObjectType != tc.WantObjType {
				t.Errorf("expected object_type %q, got %q", tc.WantObjType, stmt.DDL.ObjectType)
			}

			// Verify options.
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
			// Verify no extra options exist beyond what we expect.
			if len(stmt.DDL.Options) != len(tc.WantOptions) {
				t.Errorf("expected %d options, got %d: %v", len(tc.WantOptions), len(stmt.DDL.Options), stmt.DDL.Options)
			}

			// Security: verify that secret/credential values do NOT leak into options.
			for _, secret := range tc.SecretValues {
				for optKey, optVal := range stmt.DDL.Options {
					if optVal == secret {
						t.Errorf("SECURITY: option %q leaks credential value %q", optKey, secret)
					}
				}
			}
		})
	}
}
