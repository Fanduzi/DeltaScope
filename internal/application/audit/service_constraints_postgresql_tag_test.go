//go:build postgresql

package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLPostgreSQLPrimaryKeyRuleCoverage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "bigint_required_non_bigint_pk",
			sql:        "CREATE TABLE bad_pk_type (id integer PRIMARY KEY, name text);",
			wantRuleID: "ddl.table.primary_key.bigint.require",
		},
		{
			name:       "composite_pk_exceeds_max_columns",
			sql:        "CREATE TABLE composite_pk (tenant_id bigint, user_id bigint, PRIMARY KEY (tenant_id, user_id));",
			wantRuleID: "ddl.table.primary_key.columns.max_count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			// Must have the expected rule finding.
			found := false
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule %q, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}

			// Verify spec-level primary-key facts are populated.
			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}
			if stmt.DDL.PrimaryKey == nil {
				t.Fatal("expected DDL.PrimaryKey to be populated")
			}
			if stmt.DDL.PrimaryKey.Kind != spec.IndexKindPrimary {
				t.Fatalf("expected primary key kind, got %q", stmt.DDL.PrimaryKey.Kind)
			}
			if len(stmt.DDL.PrimaryKey.Columns) == 0 {
				t.Fatal("expected at least one primary key column")
			}

			// Verify PK columns are marked NotNull.
			for _, pkCol := range stmt.DDL.PrimaryKey.Columns {
				foundCol := false
				for _, col := range stmt.DDL.Columns {
					if col.Name == pkCol {
						foundCol = true
						if !col.NotNull {
							t.Errorf("PK column %q should be NotNull=true", pkCol)
						}
					}
				}
				if !foundCol {
					t.Errorf("PK column %q not found in DDL.Columns", pkCol)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// v0.38.0 Task 3: Service tests — standalone CREATE INDEX rule coverage
// ---------------------------------------------------------------------------

// TestAuditSQLPostgreSQLUniqueIndexRuleCoverage proves that standalone
// CREATE INDEX and CREATE UNIQUE INDEX on PostgreSQL trigger generic index
// rules through the full AuditSQL pipeline.
func TestAuditSQLPostgreSQLUniqueIndexRuleCoverage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "secondary_prefix_on_create_index",
			sql:        "CREATE INDEX bad_users_email ON users (email);",
			wantRuleID: "ddl.index.secondary.prefix.require",
		},
		{
			name:       "unique_prefix_on_create_unique_index",
			sql:        "CREATE UNIQUE INDEX bad_users_email_unique ON users (email);",
			wantRuleID: "ddl.index.unique.prefix.require",
		},
		{
			name:       "columns_max_count_on_create_index",
			sql:        "CREATE INDEX idx_wide ON users (c1, c2, c3, c4, c5, c6, c7, c8, c9);",
			wantRuleID: "ddl.index.columns.max_count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			found := false
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule %q, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}

			// Verify spec-level index facts are populated.
			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}
			if stmt.DDL.Operation != spec.DDLOperationCreateIndex {
				t.Fatalf("expected create_index operation, got %q", stmt.DDL.Operation)
			}
			if len(stmt.DDL.Indexes) == 0 {
				t.Fatal("expected at least one index")
			}
		})
	}
}

func TestAuditSQLPostgreSQLAdvancedIndexFormsNormalizedAndCovered(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		sql                string
		wantAccessMethod   string
		wantHasPredicate   bool
		wantHasExprKeys    bool
		wantExprCount      int
		wantIncludedCols   []string
		wantConcurrently   string // "true", "false", or "" to skip
		wantFindingCovered bool
	}{
		{
			name:               "partial_index",
			sql:                "CREATE INDEX idx_users_active_email ON users (email) WHERE active = true",
			wantAccessMethod:   "btree",
			wantHasPredicate:   true,
			wantFindingCovered: true,
			wantConcurrently:   "false",
		},
		{
			name:               "expression_index",
			sql:                "CREATE INDEX idx_users_lower_email ON users (LOWER(email))",
			wantAccessMethod:   "btree",
			wantHasExprKeys:    true,
			wantExprCount:      1,
			wantFindingCovered: true,
			wantConcurrently:   "false",
		},
		{
			name:               "include_index",
			sql:                "CREATE INDEX idx_users_email_cover ON users (email) INCLUDE (name, active)",
			wantAccessMethod:   "btree",
			wantIncludedCols:   []string{"name", "active"},
			wantFindingCovered: true,
			wantConcurrently:   "false",
		},
		{
			name:               "gin_index",
			sql:                "CREATE INDEX idx_docs_body ON docs USING gin (body)",
			wantAccessMethod:   "gin",
			wantFindingCovered: true,
			wantConcurrently:   "false",
		},
		{
			name:             "concurrent_partial",
			sql:              "CREATE INDEX CONCURRENTLY idx_users_active_email ON users (email) WHERE active = true",
			wantAccessMethod: "btree",
			wantHasPredicate: true,
			wantConcurrently: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			// Service-level audit: verify finding coverage.
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			if tt.wantFindingCovered {
				found := false
				for _, f := range result.Statements[0].Findings {
					if f.RuleID == "ddl.pg.create_index.concurrently.require" {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected concurrently.require finding, got %v", collectAuditResultRuleIDs(result))
				}
			}

			// Spec-level: verify coarse index facts.
			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}
			if stmt.DDL.Operation != spec.DDLOperationCreateIndex {
				t.Fatalf("expected create_index operation, got %q", stmt.DDL.Operation)
			}
			if len(stmt.DDL.Indexes) == 0 {
				t.Fatal("expected at least one index")
			}
			idx := stmt.DDL.Indexes[0]
			if idx.AccessMethod != tt.wantAccessMethod {
				t.Errorf("access method = %q, want %q", idx.AccessMethod, tt.wantAccessMethod)
			}
			if idx.HasPredicate != tt.wantHasPredicate {
				t.Errorf("HasPredicate = %v, want %v", idx.HasPredicate, tt.wantHasPredicate)
			}
			if idx.HasExpressionKeys != tt.wantHasExprKeys {
				t.Errorf("HasExpressionKeys = %v, want %v", idx.HasExpressionKeys, tt.wantHasExprKeys)
			}
			if tt.wantExprCount > 0 && idx.ExpressionCount != tt.wantExprCount {
				t.Errorf("ExpressionCount = %d, want %d", idx.ExpressionCount, tt.wantExprCount)
			}
			if tt.wantIncludedCols != nil {
				if len(idx.IncludedColumns) != len(tt.wantIncludedCols) {
					t.Fatalf("IncludedColumns = %v, want %v", idx.IncludedColumns, tt.wantIncludedCols)
				}
				for i, col := range tt.wantIncludedCols {
					if idx.IncludedColumns[i] != col {
						t.Errorf("IncludedColumns[%d] = %q, want %q", i, idx.IncludedColumns[i], col)
					}
				}
			}
			if tt.wantConcurrently != "" {
				got := stmt.DDL.Options["concurrently"]
				if got != tt.wantConcurrently {
					t.Errorf("concurrently = %q, want %q", got, tt.wantConcurrently)
				}
			}
		})
	}
}

func TestAuditSQLPostgreSQLAlterTableAddConstraintRuleCoverage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "primary_key_bigint_required",
			sql:        "ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (id);",
			wantRuleID: "ddl.table.primary_key.bigint.require",
		},
		{
			name:       "unique_prefix_required",
			sql:        "ALTER TABLE users ADD CONSTRAINT bad_email_key UNIQUE (email);",
			wantRuleID: "ddl.alter.add_index.unique.prefix.require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			found := false
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule %q, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}

			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}
			if stmt.DDL.Operation != spec.DDLOperationAlterTable {
				t.Fatalf("expected alter_table operation, got %q", stmt.DDL.Operation)
			}
			if len(stmt.DDL.Alter) != 1 {
				t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
			}
			alter := stmt.DDL.Alter[0]
			if alter.Action != "add_constraint" {
				t.Fatalf("expected add_constraint action, got %q", alter.Action)
			}
			if alter.Options["columns"] == "" {
				t.Fatal("expected columns option to be populated")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// v0.40.0 Task 3: Service tests — ALTER TABLE ADD CONSTRAINT FOREIGN KEY rule coverage
// ---------------------------------------------------------------------------

// TestAuditSQLPostgreSQLAlterTableForeignKeyRuleCoverage proves that ALTER
// TABLE ADD CONSTRAINT FOREIGN KEY triggers the FK forbid and cross-schema
// advisory rules through the full AuditSQL pipeline, with correct DDL/Alter
// facts preserved.
func TestAuditSQLPostgreSQLAlterTableForeignKeyRuleCoverage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		sql                 string
		wantRuleID          string
		wantColumns         []string
		wantRefTable        string
		wantRefColumns      []string
		wantRefSchema       string
		wantConstraintType  string
		wantCrossSchemaRule bool
	}{
		{
			name:                "fk_forbid_bare_reference",
			sql:                 "ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id);",
			wantRuleID:          "ddl.table.foreign_key.forbid",
			wantColumns:         []string{"user_id"},
			wantRefTable:        "users",
			wantRefColumns:      []string{"id"},
			wantConstraintType:  "foreign_key",
			wantCrossSchemaRule: false,
		},
		{
			name:                "fk_cross_schema_advisory",
			sql:                 "ALTER TABLE public.orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES auth.users(id);",
			wantRuleID:          "ddl.table.foreign_key.forbid",
			wantColumns:         []string{"user_id"},
			wantRefTable:        "users",
			wantRefColumns:      []string{"id"},
			wantRefSchema:       "auth",
			wantConstraintType:  "foreign_key",
			wantCrossSchemaRule: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			// Must have the FK forbid finding.
			found := false
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule %q, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}

			// Cross-schema advisory check.
			if tt.wantCrossSchemaRule {
				advisoryFound := false
				for _, f := range result.Statements[0].Findings {
					if f.RuleID == "ddl.pg.table.foreign_key.cross_schema.advisory" {
						advisoryFound = true
						break
					}
				}
				if !advisoryFound {
					t.Fatalf("expected cross-schema advisory finding, got %#v", result.Statements[0].Findings)
				}
			}

			// Verify spec-level DDL/Alter facts.
			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}
			if stmt.DDL.Operation != spec.DDLOperationAlterTable {
				t.Fatalf("expected alter_table operation, got %q", stmt.DDL.Operation)
			}
			if len(stmt.DDL.Alter) != 1 {
				t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
			}
			alter := stmt.DDL.Alter[0]
			if alter.Action != "add_constraint" {
				t.Fatalf("expected add_constraint action, got %q", alter.Action)
			}
			if alter.Options["constraint_type"] != tt.wantConstraintType {
				t.Fatalf("expected constraint_type %q, got %q", tt.wantConstraintType, alter.Options["constraint_type"])
			}
			if alter.Options["columns"] == "" {
				t.Fatal("expected columns option to be populated")
			}

			// Verify DDL.Constraints contains the FK constraint with correct facts.
			fkFound := false
			for _, c := range stmt.DDL.Constraints {
				if c.Type == "foreign_key" && c.Name == "fk_orders_user" {
					fkFound = true
					if len(c.Columns) != len(tt.wantColumns) || c.Columns[0] != tt.wantColumns[0] {
						t.Errorf("expected Columns %v, got %v", tt.wantColumns, c.Columns)
					}
					if c.ReferencedTable != tt.wantRefTable {
						t.Errorf("expected ReferencedTable %q, got %q", tt.wantRefTable, c.ReferencedTable)
					}
					if len(c.ReferencedColumns) != len(tt.wantRefColumns) || c.ReferencedColumns[0] != tt.wantRefColumns[0] {
						t.Errorf("expected ReferencedColumns %v, got %v", tt.wantRefColumns, c.ReferencedColumns)
					}
					if tt.wantRefSchema != "" && c.ReferencedSchema != tt.wantRefSchema {
						t.Errorf("expected ReferencedSchema %q, got %q", tt.wantRefSchema, c.ReferencedSchema)
					}
				}
			}
			if !fkFound {
				t.Fatalf("expected foreign_key constraint fk_orders_user in %+v", stmt.DDL.Constraints)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// v0.41.0 Task 3: Service tests — ALTER TABLE ADD CONSTRAINT CHECK rule coverage
// ---------------------------------------------------------------------------

// TestAuditSQLPostgreSQLAlterTableCheckRuleCoverage proves that ALTER TABLE ADD
// CONSTRAINT CHECK triggers check naming and not_valid rules through the full
// AuditSQL pipeline, with correct DDL/Alter/Constraint facts preserved.
func TestAuditSQLPostgreSQLAlterTableCheckRuleCoverage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                  string
		sql                   string
		enablePrefixRule      bool
		wantRuleIDs           []string
		wantNotAbsentRuleIDs  []string
		wantColumns           string
		wantConstraintName    string
		wantConstraintColumns []string
		wantNotValid          string
	}{
		{
			name:                  "naming_prefix_and_not_valid_warning",
			sql:                   "ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0);",
			enablePrefixRule:      true,
			wantRuleIDs:           []string{"ddl.constraint.check.name.prefix.require", "ddl.pg.alter.add_check.not_valid.require"},
			wantNotAbsentRuleIDs:  nil,
			wantColumns:           "amount",
			wantConstraintName:    "amount_positive",
			wantConstraintColumns: []string{"amount"},
			wantNotValid:          "false",
		},
		{
			name:                  "not_valid_suppresses_not_valid_rule",
			sql:                   "ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0) NOT VALID;",
			enablePrefixRule:      true,
			wantRuleIDs:           []string{"ddl.constraint.check.name.prefix.require"},
			wantNotAbsentRuleIDs:  []string{"ddl.pg.alter.add_check.not_valid.require"},
			wantColumns:           "amount",
			wantConstraintName:    "amount_positive",
			wantConstraintColumns: []string{"amount"},
			wantNotValid:          "true",
		},
		{
			name:                  "multi_column_check_preserves_columns",
			sql:                   "ALTER TABLE orders ADD CONSTRAINT amount_tax_positive CHECK (amount + tax >= 0);",
			enablePrefixRule:      true,
			wantRuleIDs:           []string{"ddl.constraint.check.name.prefix.require", "ddl.pg.alter.add_check.not_valid.require"},
			wantNotAbsentRuleIDs:  nil,
			wantColumns:           "amount,tax",
			wantConstraintName:    "amount_tax_positive",
			wantConstraintColumns: []string{"amount", "tax"},
			wantNotValid:          "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			req := Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			}
			if tt.enablePrefixRule {
				req.ConfigPath = corpusConfigPath(t, corpusExpected{
					Config: map[string]any{
						"rules": map[string]any{
							"ddl.constraint.check.name.prefix.require": map[string]any{
								"enabled": true,
								"params":  map[string]any{"prefix": "ck_"},
							},
						},
					},
				})
			}

			result, err := AuditSQL(context.Background(), req)
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			// Collect finding rule IDs.
			findingIDs := make(map[string]struct{})
			for _, f := range result.Statements[0].Findings {
				findingIDs[f.RuleID] = struct{}{}
			}

			// Assert expected findings present.
			for _, want := range tt.wantRuleIDs {
				if _, ok := findingIDs[want]; !ok {
					t.Fatalf("expected finding %q, got findings: %v", want, sortedKeys(findingIDs))
				}
			}

			// Assert absent findings.
			for _, absent := range tt.wantNotAbsentRuleIDs {
				if _, ok := findingIDs[absent]; ok {
					t.Fatalf("expected rule %q to be absent, but it was present", absent)
				}
			}

			// Verify spec-level facts.
			stmt, ok := corpusExtractStatement(t, tt.sql, spec.DialectPostgreSQL)
			if !ok {
				t.Fatal("expected supported statement")
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL payload")
			}
			if stmt.DDL.Operation != spec.DDLOperationAlterTable {
				t.Fatalf("expected alter_table operation, got %q", stmt.DDL.Operation)
			}
			if len(stmt.DDL.Alter) != 1 {
				t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
			}
			alter := stmt.DDL.Alter[0]
			if alter.Action != "add_constraint" {
				t.Fatalf("expected add_constraint action, got %q", alter.Action)
			}
			if alter.Options["constraint_type"] != "check" {
				t.Fatalf("expected constraint_type check, got %q", alter.Options["constraint_type"])
			}
			if alter.Options["columns"] != tt.wantColumns {
				t.Fatalf("expected columns %q, got %q", tt.wantColumns, alter.Options["columns"])
			}
			if alter.Options["not_valid"] != tt.wantNotValid {
				t.Fatalf("expected not_valid %q, got %q", tt.wantNotValid, alter.Options["not_valid"])
			}

			// Verify DDL.Constraints contains the check constraint.
			ckFound := false
			for _, c := range stmt.DDL.Constraints {
				if c.Type == "check" && c.Name == tt.wantConstraintName {
					ckFound = true
					if len(c.Columns) != len(tt.wantConstraintColumns) {
						t.Errorf("expected %d columns, got %d", len(tt.wantConstraintColumns), len(c.Columns))
					}
					for i, want := range tt.wantConstraintColumns {
						if i >= len(c.Columns) || c.Columns[i] != want {
							t.Errorf("expected column[%d]=%q, got %v", i, want, c.Columns)
						}
					}
				}
			}
			if !ckFound {
				t.Fatalf("expected check constraint %q in %+v", tt.wantConstraintName, stmt.DDL.Constraints)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// v0.42.0 Task 3: Service tests — NOT VALID constraint validation rule coverage
// ---------------------------------------------------------------------------

// TestAuditSQLPostgreSQLNotValidConstraintValidationRuleCoverage proves that
// the GlobalRule ddl.pg.alter.not_valid_constraint.validate.require fires
// through the full AuditSQL pipeline when a named NOT VALID CHECK/FK is not
// followed by a matching VALIDATE CONSTRAINT.
func TestAuditSQLPostgreSQLNotValidConstraintValidationRuleCoverage(t *testing.T) {
	t.Parallel()
	const ruleID = "ddl.pg.alter.not_valid_constraint.validate.require"

	tests := []struct {
		name              string
		sql               string
		wantGlobalFinding bool
		wantConstraint    string
		wantTable         string
	}{
		{
			name:              "check_not_valid_without_validate_fires",
			sql:               "ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;",
			wantGlobalFinding: true,
			wantConstraint:    "chk_orders_amount",
			wantTable:         "orders",
		},
		{
			name: "check_not_valid_with_later_validate_suppressed",
			sql: `ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;
ALTER TABLE orders VALIDATE CONSTRAINT chk_orders_amount;`,
			wantGlobalFinding: false,
		},
		{
			name:              "fk_not_valid_without_validate_fires",
			sql:               "ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;",
			wantGlobalFinding: true,
			wantConstraint:    "fk_orders_user",
			wantTable:         "orders",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}

			var found *rule.Finding
			for i := range result.GlobalFindings {
				if result.GlobalFindings[i].RuleID == ruleID {
					f := result.GlobalFindings[i]
					found = &f
					break
				}
			}

			if tt.wantGlobalFinding {
				if found == nil {
					t.Fatalf("expected global finding %q, got global findings: %+v", ruleID, result.GlobalFindings)
				}
				if found.Metadata["constraint"] != tt.wantConstraint {
					t.Errorf("expected constraint %q, got %v", tt.wantConstraint, found.Metadata["constraint"])
				}
				if found.Metadata["table"] != tt.wantTable {
					t.Errorf("expected table %q, got %v", tt.wantTable, found.Metadata["table"])
				}
				if found.Metadata["dialect"] != "postgresql" {
					t.Errorf("expected dialect postgresql, got %v", found.Metadata["dialect"])
				}
			} else {
				if found != nil {
					t.Fatalf("expected no global finding %q, got %+v", ruleID, found)
				}
			}
		})
	}
}
