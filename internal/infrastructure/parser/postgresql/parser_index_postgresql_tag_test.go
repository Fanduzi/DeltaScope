//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractCreateIndexConcurrentFlag(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "create index concurrently idx_users_email on public.users (email);")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported create index, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateIndex {
		t.Fatalf("expected create_index operation, got %#v", statement.DDL)
	}
	if statement.DDL.Options["concurrently"] != "true" {
		t.Fatalf("expected concurrently=true, got %#v", statement.DDL.Options)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Schema != "public" || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table public.users, got %#v", statement.DDL.Table)
	}
	if len(statement.DDL.Indexes) != 1 || statement.DDL.Indexes[0].Name != "idx_users_email" {
		t.Fatalf("expected index name idx_users_email, got %#v", statement.DDL.Indexes)
	}
	if len(statement.DDL.Indexes[0].Columns) != 1 || statement.DDL.Indexes[0].Columns[0] != "email" {
		t.Fatalf("expected index column [email], got %#v", statement.DDL.Indexes[0].Columns)
	}
}

func TestExtractCreateIndexNonConcurrent(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "create index idx_users_email on public.users (email);")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported create index, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateIndex {
		t.Fatalf("expected create_index operation, got %#v", statement.DDL)
	}
	if statement.DDL.Options["concurrently"] != "false" {
		t.Fatalf("expected concurrently=false, got %#v", statement.DDL.Options)
	}
}

func TestExtractCreateUniqueIndex(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "create unique index idx_users_email on public.users (email);")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported create unique index, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateIndex {
		t.Fatalf("expected create_index operation, got %#v", statement.DDL)
	}
	if len(statement.DDL.Indexes) != 1 || statement.DDL.Indexes[0].Kind != spec.IndexKindUnique {
		t.Fatalf("expected unique index kind, got %#v", statement.DDL.Indexes)
	}
}

func TestExtractCreateIndexNormalizesPartialIndex(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "create index idx_active on public.users (email) where active = true;")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected DDL kind for partial index, got %q", statement.Kind)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %s", statement.Unsupported.Reason)
	}
	if statement.DDL == nil || len(statement.DDL.Indexes) == 0 {
		t.Fatal("expected DDL with index")
	}
	if !statement.DDL.Indexes[0].HasPredicate {
		t.Fatal("expected HasPredicate true")
	}
}

func TestExtractCreateIndexNormalizesExpressionIndex(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "create index idx_lower_email on public.users (lower(email));")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected DDL kind for expression index, got %q", statement.Kind)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %s", statement.Unsupported.Reason)
	}
	if statement.DDL == nil || len(statement.DDL.Indexes) == 0 {
		t.Fatal("expected DDL with index")
	}
	if !statement.DDL.Indexes[0].HasExpressionKeys {
		t.Fatal("expected HasExpressionKeys true")
	}
}

func TestExtractCreateIndexNormalizesIncludeClause(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "create index idx_users_email on public.users (email) include (name);")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected DDL kind for include clause, got %q", statement.Kind)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %s", statement.Unsupported.Reason)
	}
	if statement.DDL == nil || len(statement.DDL.Indexes) == 0 {
		t.Fatal("expected DDL with index")
	}
	if len(statement.DDL.Indexes[0].IncludedColumns) != 1 || statement.DDL.Indexes[0].IncludedColumns[0] != "name" {
		t.Fatalf("expected included [name], got %v", statement.DDL.Indexes[0].IncludedColumns)
	}
}

func TestExtractCreateIndexNormalizesNonBtreeAccessMethod(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "create index idx_users_email_hash on public.users using hash (email);")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected DDL kind for non-btree index, got %q", statement.Kind)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %s", statement.Unsupported.Reason)
	}
	if statement.DDL == nil || len(statement.DDL.Indexes) == 0 {
		t.Fatal("expected DDL with index")
	}
	if statement.DDL.Indexes[0].AccessMethod != "hash" {
		t.Fatalf("expected access method hash, got %q", statement.DDL.Indexes[0].AccessMethod)
	}
}

func TestExtractCreateIndexRejectsNullsNotDistinct(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "create unique index idx_users_email_unique on public.users (email) nulls not distinct;")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected unsupported kind unknown for nulls not distinct, got %q", statement.Kind)
	}
	if statement.Unsupported == nil || statement.Unsupported.Feature != "create_index" {
		t.Fatalf("expected unsupported create_index, got %#v", statement.Unsupported)
	}
	if statement.Unsupported.Reason == "" {
		t.Fatalf("expected non-empty reason for nulls not distinct, got %#v", statement.Unsupported)
	}
}

func TestCharacterizePGUniqueIndexFacts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		sql                 string
		wantSupported       bool
		wantDDLOperation    spec.DDLOperation
		wantTable           string
		wantIndex           *spec.Index
		wantConcurrently    string
		wantUnsupportedFeat string
	}{
		{
			name:             "inline_column_unique_unnamed",
			sql:              `create table users (email text unique);`,
			wantSupported:    true,
			wantDDLOperation: spec.DDLOperationCreateTable,
			wantTable:        "users",
			wantIndex: &spec.Index{
				Name:    "",
				Kind:    spec.IndexKindUnique,
				Columns: []string{"email"},
			},
		},
		{
			name:             "inline_column_unique_named",
			sql:              `create table users (email text constraint users_email_key unique);`,
			wantSupported:    true,
			wantDDLOperation: spec.DDLOperationCreateTable,
			wantTable:        "users",
			wantIndex: &spec.Index{
				Name:    "users_email_key",
				Kind:    spec.IndexKindUnique,
				Columns: []string{"email"},
			},
		},
		{
			name:             "table_level_unique_unnamed",
			sql:              `create table users (email text, unique (email));`,
			wantSupported:    true,
			wantDDLOperation: spec.DDLOperationCreateTable,
			wantTable:        "users",
			wantIndex: &spec.Index{
				Name:    "",
				Kind:    spec.IndexKindUnique,
				Columns: []string{"email"},
			},
		},
		{
			name:             "table_level_unique_named",
			sql:              `create table users (email text, constraint users_email_key unique (email));`,
			wantSupported:    true,
			wantDDLOperation: spec.DDLOperationCreateTable,
			wantTable:        "users",
			wantIndex: &spec.Index{
				Name:    "users_email_key",
				Kind:    spec.IndexKindUnique,
				Columns: []string{"email"},
			},
		},
		{
			name:             "standalone_create_index",
			sql:              `create index idx_users_email on users (email);`,
			wantSupported:    true,
			wantDDLOperation: spec.DDLOperationCreateIndex,
			wantTable:        "users",
			wantIndex: &spec.Index{
				Name:    "idx_users_email",
				Kind:    spec.IndexKindSecondary,
				Columns: []string{"email"},
			},
			wantConcurrently: "false",
		},
		{
			name:             "standalone_create_unique_index",
			sql:              `create unique index uniq_users_email on users (email);`,
			wantSupported:    true,
			wantDDLOperation: spec.DDLOperationCreateIndex,
			wantTable:        "users",
			wantIndex: &spec.Index{
				Name:    "uniq_users_email",
				Kind:    spec.IndexKindUnique,
				Columns: []string{"email"},
			},
			wantConcurrently: "false",
		},
		{
			name:             "standalone_create_index_concurrently",
			sql:              `create index concurrently idx_users_email on users (email);`,
			wantSupported:    true,
			wantDDLOperation: spec.DDLOperationCreateIndex,
			wantTable:        "users",
			wantIndex: &spec.Index{
				Name:    "idx_users_email",
				Kind:    spec.IndexKindSecondary,
				Columns: []string{"email"},
			},
			wantConcurrently: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			statement := extractPostgreSQLStatement(t, tt.sql)

			if tt.wantSupported {
				if statement.Unsupported != nil {
					t.Fatalf("expected supported, got unsupported %#v", statement.Unsupported)
				}
				if statement.Kind != spec.KindDDL {
					t.Fatalf("expected kind DDL, got %q", statement.Kind)
				}
				if statement.DDL == nil || statement.DDL.Operation != tt.wantDDLOperation {
					t.Fatalf("expected DDL operation %q, got %#v", tt.wantDDLOperation, statement.DDL)
				}
				if statement.DDL.Table == nil || statement.DDL.Table.Name != tt.wantTable {
					t.Fatalf("expected table %q, got %#v", tt.wantTable, statement.DDL.Table)
				}
				if len(statement.DDL.Indexes) != 1 {
					t.Fatalf("expected 1 index, got %d: %#v", len(statement.DDL.Indexes), statement.DDL.Indexes)
				}
				got := statement.DDL.Indexes[0]
				if got.Name != tt.wantIndex.Name {
					t.Fatalf("expected index name %q, got %q", tt.wantIndex.Name, got.Name)
				}
				if got.Kind != tt.wantIndex.Kind {
					t.Fatalf("expected index kind %q, got %q", tt.wantIndex.Kind, got.Kind)
				}
				if len(got.Columns) != len(tt.wantIndex.Columns) {
					t.Fatalf("expected columns %#v, got %#v", tt.wantIndex.Columns, got.Columns)
				}
				for i, col := range tt.wantIndex.Columns {
					if got.Columns[i] != col {
						t.Fatalf("expected column[%d] = %q, got %q", i, col, got.Columns[i])
					}
				}
				if tt.wantConcurrently != "" {
					if statement.DDL.Options["concurrently"] != tt.wantConcurrently {
						t.Fatalf("expected concurrently=%q, got %q", tt.wantConcurrently, statement.DDL.Options["concurrently"])
					}
				}
			} else {
				if statement.Unsupported == nil {
					t.Fatalf("expected unsupported statement, got supported")
				}
				if tt.wantUnsupportedFeat != "" && statement.Unsupported.Feature != tt.wantUnsupportedFeat {
					t.Fatalf("expected unsupported feature %q, got %q", tt.wantUnsupportedFeat, statement.Unsupported.Feature)
				}
			}
		})
	}
}

func TestCharacterizePGUnsupportedIndexForms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		sql         string
		wantFeature string
	}{
		{
			name:        "nulls_not_distinct",
			sql:         `create unique index idx_users_email_nulls on users (email) nulls not distinct;`,
			wantFeature: "create_index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			statement := extractPostgreSQLStatement(t, tt.sql)

			if statement.Unsupported == nil {
				t.Fatalf("expected unsupported statement, got supported: %#v", statement)
			}
			if statement.Unsupported.Feature != tt.wantFeature {
				t.Fatalf("expected unsupported feature %q, got %q", tt.wantFeature, statement.Unsupported.Feature)
			}
		})
	}
}
