//go:build postgresql

package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func TestParserParsesMultiStatementSQL(t *testing.T) {
	parser := New()

	result, err := parser.Parse("create table t1 (id bigint); update t1 set id = 2 where id = 1;")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(result.Statements))
	}

	if result.Statements[0].Kind != spec.KindDDL {
		t.Fatalf("expected first statement to be DDL, got %q", result.Statements[0].Kind)
	}
	if result.Statements[1].Kind != spec.KindDML {
		t.Fatalf("expected second statement to be DML, got %q", result.Statements[1].Kind)
	}
	if result.Statements[0].RawSQL != "create table t1 (id bigint);" {
		t.Fatalf("expected first statement SQL to be sliced, got %q", result.Statements[0].RawSQL)
	}
	if result.Statements[1].RawSQL != "update t1 set id = 2 where id = 1;" {
		t.Fatalf("expected second statement SQL to be sliced, got %q", result.Statements[1].RawSQL)
	}
}

func TestPGExtractorExtractNormalizesStatementSQL(t *testing.T) {
	parser := New()

	result, err := parser.Parse(" create table t1 (id bigint); ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, " create table t1 (id bigint); ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind %q, got %q", spec.KindDDL, statement.Kind)
	}
	if statement.Dialect != spec.DialectPostgreSQL {
		t.Fatalf("expected dialect %q, got %q", spec.DialectPostgreSQL, statement.Dialect)
	}
	if statement.RawSQL != " create table t1 (id bigint); " {
		t.Fatalf("expected raw SQL preserved, got %q", statement.RawSQL)
	}
	if statement.NormalizedSQL != "create table t1 (id bigint)" {
		t.Fatalf("expected normalized SQL without outer whitespace or trailing semicolon, got %q", statement.NormalizedSQL)
	}
}

func TestParserMarksUnsupportedPostgreSQLStatementsStructurally(t *testing.T) {
	parser := New()

	result, err := parser.Parse("select 1;")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract unsupported statement: %v", err)
	}
	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected unsupported statement kind unknown, got %q", statement.Kind)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail to be populated")
	}
	if statement.Unsupported.Feature != "select" {
		t.Fatalf("expected unsupported feature select, got %#v", statement.Unsupported)
	}
	if statement.Unsupported.Reason == "" {
		t.Fatalf("expected unsupported reason, got %#v", statement.Unsupported)
	}
}

func TestParserSupportsApprovedPostgreSQLAlterTableWhitelist(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table users rename column old_name to new_name;")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	statement, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract supported alter table statement: %v", err)
	}
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected ddl statement kind, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %#v", statement.DDL)
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "rename_column" {
		t.Fatalf("expected rename_column action, got %#v", alter)
	}
	if alter.Column == nil || alter.Column.OldName != "old_name" || alter.Column.Definition == nil || alter.Column.Definition.Name != "new_name" {
		t.Fatalf("expected old/new column names, got %#v", alter.Column)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported alter table to omit unsupported detail, got %#v", statement.Unsupported)
	}
}

func TestParserPreservesDropConstraintActionForPostgreSQL(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table public.users drop constraint users_pkey;")
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
	if statement.DDL == nil || len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %#v", statement)
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "drop_constraint" {
		t.Fatalf("expected drop_constraint action, got %#v", alter)
	}
	if alter.Name != "users_pkey" {
		t.Fatalf("expected preserved constraint name, got %#v", alter)
	}
}

func TestParserPreservesSetDataTypeActionForPostgreSQL(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table users alter column status type bigint;")
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
		t.Fatalf("expected supported alter column type, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %#v", statement)
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "set_data_type" {
		t.Fatalf("expected set_data_type action, got %#v", alter)
	}
	if alter.Column == nil || alter.Column.OldName != "status" || alter.Column.Definition == nil || alter.Column.Definition.Type == "" {
		t.Fatalf("expected preserved alter-column type payload, got %#v", alter.Column)
	}
}

func TestParserSupportsPostgreSQLCreateView(t *testing.T) {
	parser := New()

	result, err := parser.Parse("create view public.active_users as select id from public.users;")
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
		t.Fatalf("expected supported create view, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil {
		t.Fatalf("expected ddl payload, got %#v", statement)
	}
	if statement.DDL.Operation != spec.DDLOperationCreateView {
		t.Fatalf("expected create view operation, got %#v", statement.DDL)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Schema != "public" || statement.DDL.Table.Name != "active_users" {
		t.Fatalf("expected qualified view target, got %#v", statement.DDL.Table)
	}
	if !statement.DDL.HasSelect {
		t.Fatalf("expected create view to preserve select shape, got %#v", statement.DDL)
	}
}

func TestParserRejectsPostgreSQLCreateOrReplaceView(t *testing.T) {
	parser := New()

	result, err := parser.Parse("create or replace view public.active_users as select id from public.users;")
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
		t.Fatalf("expected unsupported kind unknown, got %#v", statement)
	}
	if statement.Unsupported == nil || statement.Unsupported.Feature != "create_view" {
		t.Fatalf("expected unsupported create_view detail, got %#v", statement.Unsupported)
	}
}

func TestParserRejectsPostgreSQLCreateTemporaryView(t *testing.T) {
	parser := New()

	result, err := parser.Parse("create temporary view public.active_users as select id from public.users;")
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
		t.Fatalf("expected unsupported kind unknown, got %#v", statement)
	}
	if statement.Unsupported == nil || statement.Unsupported.Feature != "create_view" {
		t.Fatalf("expected unsupported create_view detail, got %#v", statement.Unsupported)
	}
}

func TestParserSupportsPostgreSQLDropView(t *testing.T) {
	parser := New()

	result, err := parser.Parse("drop view if exists public.active_users;")
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
		t.Fatalf("expected supported drop view, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil {
		t.Fatalf("expected ddl payload, got %#v", statement)
	}
	if statement.DDL.Operation != spec.DDLOperationDropView {
		t.Fatalf("expected drop view operation, got %#v", statement.DDL)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Schema != "public" || statement.DDL.Table.Name != "active_users" {
		t.Fatalf("expected qualified view target, got %#v", statement.DDL.Table)
	}
	if statement.DDL.Options["if_exists"] != "true" {
		t.Fatalf("expected if_exists option, got %#v", statement.DDL.Options)
	}
}

func TestParserRejectsPostgreSQLMultiTargetDropView(t *testing.T) {
	parser := New()

	result, err := parser.Parse("drop view public.active_users, public.disabled_users;")
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
	if statement.Unsupported == nil {
		t.Fatalf("expected multi-target drop view to stay unsupported, got %#v", statement)
	}
	if statement.Unsupported.Feature != "drop" {
		t.Fatalf("expected unsupported feature drop, got %#v", statement.Unsupported)
	}
}

func TestExtractCreateIndexConcurrentFlag(t *testing.T) {
	parser := New()

	result, err := parser.Parse("create index concurrently idx_users_email on public.users (email);")
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
	parser := New()

	result, err := parser.Parse("create index idx_users_email on public.users (email);")
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
	parser := New()

	result, err := parser.Parse("create unique index idx_users_email on public.users (email);")
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

func TestExtractCreateIndexRejectsPartialIndex(t *testing.T) {
	parser := New()

	result, err := parser.Parse("create index idx_active on public.users (email) where active = true;")
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
		t.Fatalf("expected unsupported kind unknown for partial index, got %q", statement.Kind)
	}
	if statement.Unsupported == nil || statement.Unsupported.Feature != "create_index" {
		t.Fatalf("expected unsupported create_index, got %#v", statement.Unsupported)
	}
}

func TestExtractCreateIndexRejectsExpressionIndex(t *testing.T) {
	parser := New()

	result, err := parser.Parse("create index idx_lower_email on public.users (lower(email));")
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
		t.Fatalf("expected unsupported kind unknown for expression index, got %q", statement.Kind)
	}
	if statement.Unsupported == nil || statement.Unsupported.Feature != "create_index" {
		t.Fatalf("expected unsupported create_index, got %#v", statement.Unsupported)
	}
}

func TestExtractCreateIndexRejectsIncludeClause(t *testing.T) {
	parser := New()

	result, err := parser.Parse("create index idx_users_email on public.users (email) include (name);")
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
		t.Fatalf("expected unsupported kind unknown for include clause, got %q", statement.Kind)
	}
	if statement.Unsupported == nil || statement.Unsupported.Feature != "create_index" {
		t.Fatalf("expected unsupported create_index, got %#v", statement.Unsupported)
	}
}

func TestExtractCreateIndexRejectsNonBtreeAccessMethod(t *testing.T) {
	parser := New()

	result, err := parser.Parse("create index idx_users_email_hash on public.users using hash (email);")
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
		t.Fatalf("expected unsupported kind unknown for non-btree access method, got %q", statement.Kind)
	}
	if statement.Unsupported == nil || statement.Unsupported.Feature != "create_index" {
		t.Fatalf("expected unsupported create_index, got %#v", statement.Unsupported)
	}
	if statement.Unsupported.Reason == "" {
		t.Fatalf("expected non-empty reason for non-btree access method, got %#v", statement.Unsupported)
	}
}

func TestExtractCreateIndexRejectsNullsNotDistinct(t *testing.T) {
	parser := New()

	result, err := parser.Parse("create unique index idx_users_email_unique on public.users (email) nulls not distinct;")
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

func TestExtractAlterAddCheckNotValidFlag(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table public.orders add constraint chk_amount check (amount > 0) not valid;")
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
	if statement.DDL == nil || len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %#v", statement)
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "add_constraint" {
		t.Fatalf("expected add_constraint, got %#v", alter)
	}
	if alter.Options["constraint_type"] != "check" {
		t.Fatalf("expected constraint_type=check, got %#v", alter.Options)
	}
	if alter.Options["not_valid"] != "true" {
		t.Fatalf("expected not_valid=true, got %#v", alter.Options)
	}
}

func TestExtractAlterAddCheckWithoutNotValid(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table public.orders add constraint chk_amount check (amount > 0);")
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
	if statement.DDL == nil || len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %#v", statement)
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "add_constraint" {
		t.Fatalf("expected add_constraint, got %#v", alter)
	}
	if alter.Options["constraint_type"] != "check" {
		t.Fatalf("expected constraint_type=check, got %#v", alter.Options)
	}
	if alter.Options["not_valid"] != "false" {
		t.Fatalf("expected not_valid=false, got %#v", alter.Options)
	}
}

func TestExtractAlterColumnSetDefault(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table users alter column status set default 'active';")
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
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "set_default" {
		t.Fatalf("expected set_default action, got %q", alter.Action)
	}
	if alter.Name != "status" {
		t.Fatalf("expected column name status, got %q", alter.Name)
	}
	if alter.Column == nil || alter.Column.Change == nil || !alter.Column.Change.TouchesDefault {
		t.Fatalf("expected TouchesDefault=true, got %#v", alter.Column)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestExtractAlterColumnDropDefault(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table users alter column status drop default;")
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
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "drop_default" {
		t.Fatalf("expected drop_default action, got %q", alter.Action)
	}
	if alter.Name != "status" {
		t.Fatalf("expected column name status, got %q", alter.Name)
	}
	if alter.Column == nil || alter.Column.Change == nil || !alter.Column.Change.TouchesDefault {
		t.Fatalf("expected TouchesDefault=true, got %#v", alter.Column)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestExtractAlterColumnSetNotNull(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table users alter column status set not null;")
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
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "set_not_null" {
		t.Fatalf("expected set_not_null action, got %q", alter.Action)
	}
	if alter.Name != "status" {
		t.Fatalf("expected column name status, got %q", alter.Name)
	}
	if alter.Column == nil || alter.Column.Change == nil || !alter.Column.Change.TouchesNullability {
		t.Fatalf("expected TouchesNullability=true, got %#v", alter.Column)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestExtractAlterColumnDropNotNull(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table users alter column status drop not null;")
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
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "drop_not_null" {
		t.Fatalf("expected drop_not_null action, got %q", alter.Action)
	}
	if alter.Name != "status" {
		t.Fatalf("expected column name status, got %q", alter.Name)
	}
	if alter.Column == nil || alter.Column.Change == nil || !alter.Column.Change.TouchesNullability {
		t.Fatalf("expected TouchesNullability=true, got %#v", alter.Column)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestParserSupportsPostgreSQLRenameIndex(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter index idx_old rename to idx_new;")
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
		t.Fatalf("expected supported rename index, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %#v", statement)
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "rename_index" {
		t.Fatalf("expected rename_index action, got %#v", alter)
	}
	if alter.Name != "idx_old" {
		t.Fatalf("expected old index name idx_old, got %#v", alter)
	}
	if alter.Options["new_name"] != "idx_new" {
		t.Fatalf("expected new index name idx_new, got %#v", alter)
	}
}

func TestExtractValidateConstraint(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table users validate constraint chk_amount;")
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
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "validate_constraint" {
		t.Fatalf("expected validate_constraint action, got %q", alter.Action)
	}
	if alter.Name != "chk_amount" {
		t.Fatalf("expected constraint name chk_amount, got %q", alter.Name)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestExtractDropPrimaryKeyConstraint(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table users drop constraint users_pkey;")
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
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "drop_constraint" {
		t.Fatalf("expected drop_constraint action, got %q", alter.Action)
	}
	if alter.Name != "users_pkey" {
		t.Fatalf("expected constraint name users_pkey, got %q", alter.Name)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestExtractDropCheckConstraint(t *testing.T) {
	parser := New()

	result, err := parser.Parse("alter table users drop constraint chk_amount;")
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
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table ddl payload, got %#v", statement.DDL)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "drop_constraint" {
		t.Fatalf("expected drop_constraint action, got %q", alter.Action)
	}
	if alter.Name != "chk_amount" {
		t.Fatalf("expected constraint name chk_amount, got %q", alter.Name)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %#v", statement.DDL.Table)
	}
}

func TestExtractCreateTableConstraintNormalization(t *testing.T) {
	tests := []struct {
		name           string
		sql            string
		tableName      string
		wantColumns    int
		wantConstraint *spec.Constraint
		wantIndex      *spec.Index
	}{
		{
			name: "named_table_check",
			sql: `create table orders (
				id bigint primary key,
				amount bigint,
				constraint chk_orders_amount check (amount > 0)
			);`,
			tableName:   "orders",
			wantColumns: 2,
			wantConstraint: &spec.Constraint{
				Type:    "check",
				Name:    "chk_orders_amount",
				Columns: []string{"amount"},
			},
		},
		{
			name: "inline_column_check",
			sql: `create table users (
				age int check (age >= 0)
			);`,
			tableName:   "users",
			wantColumns: 1,
			wantConstraint: &spec.Constraint{
				Type:    "check",
				Columns: []string{"age"},
			},
		},
		{
			name: "named_table_unique",
			sql: `create table users (
				id bigint,
				email text,
				constraint uq_users_email unique (email)
			);`,
			tableName:   "users",
			wantColumns: 2,
			wantIndex: &spec.Index{
				Name:    "uq_users_email",
				Kind:    spec.IndexKindUnique,
				Columns: []string{"email"},
			},
		},
		{
			name: "inline_column_unique",
			sql: `create table users (
				email text unique
			);`,
			tableName:   "users",
			wantColumns: 1,
			wantIndex: &spec.Index{
				Kind:    spec.IndexKindUnique,
				Columns: []string{"email"},
			},
		},
		{
			name: "named_table_foreign_key",
			sql: `create table orders (
				user_id bigint,
				constraint fk_orders_user foreign key (user_id) references users(id)
			);`,
			tableName:   "orders",
			wantColumns: 1,
			wantConstraint: &spec.Constraint{
				Type:    "foreign_key",
				Name:    "fk_orders_user",
				Columns: []string{"user_id"},
			},
		},
		{
			name: "inline_column_references",
			sql: `create table orders (
				user_id bigint references users(id)
			);`,
			tableName:   "orders",
			wantColumns: 1,
			wantConstraint: &spec.Constraint{
				Type:    "foreign_key",
				Columns: []string{"user_id"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement := extractPostgreSQLStatement(t, tt.sql)

			if statement.Kind != spec.KindDDL {
				t.Fatalf("expected kind %q, got %q", spec.KindDDL, statement.Kind)
			}
			if statement.Unsupported != nil {
				t.Fatalf("expected supported create table, got unsupported %#v", statement.Unsupported)
			}
			if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateTable {
				t.Fatalf("expected create_table ddl payload, got %#v", statement.DDL)
			}
			if statement.DDL.Table == nil || statement.DDL.Table.Name != tt.tableName {
				t.Fatalf("expected table name %s, got %#v", tt.tableName, statement.DDL.Table)
			}
			if len(statement.DDL.Columns) != tt.wantColumns {
				t.Fatalf("expected %d columns, got %#v", tt.wantColumns, statement.DDL.Columns)
			}

			if tt.wantConstraint != nil {
				if len(statement.DDL.Constraints) != 1 {
					t.Fatalf("expected 1 constraint, got %#v", statement.DDL.Constraints)
				}
				constraint := statement.DDL.Constraints[0]
				if constraint.Type != tt.wantConstraint.Type || constraint.Name != tt.wantConstraint.Name {
					t.Fatalf("expected constraint %+v, got %+v", *tt.wantConstraint, constraint)
				}
				if len(constraint.Columns) != len(tt.wantConstraint.Columns) {
					t.Fatalf("expected constraint columns %#v, got %#v", tt.wantConstraint.Columns, constraint.Columns)
				}
				for i, wantColumn := range tt.wantConstraint.Columns {
					if constraint.Columns[i] != wantColumn {
						t.Fatalf("expected constraint columns %#v, got %#v", tt.wantConstraint.Columns, constraint.Columns)
					}
				}
			}

			if tt.wantIndex != nil {
				if len(statement.DDL.Indexes) != 1 {
					t.Fatalf("expected 1 index, got %#v", statement.DDL.Indexes)
				}
				index := statement.DDL.Indexes[0]
				if index.Name != tt.wantIndex.Name || index.Kind != tt.wantIndex.Kind {
					t.Fatalf("expected index %+v, got %+v", *tt.wantIndex, index)
				}
				if len(index.Columns) != len(tt.wantIndex.Columns) {
					t.Fatalf("expected index columns %#v, got %#v", tt.wantIndex.Columns, index.Columns)
				}
				for i, wantColumn := range tt.wantIndex.Columns {
					if index.Columns[i] != wantColumn {
						t.Fatalf("expected index columns %#v, got %#v", tt.wantIndex.Columns, index.Columns)
					}
				}
			}
		})
	}
}

func TestExtractCreateTableForeignKeyPreservesReferencedTableAndColumns(t *testing.T) {
	sql := `create table orders (
		user_id bigint,
		constraint fk_orders_user foreign key (user_id) references users(id)
	);`
	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind %q, got %q", spec.KindDDL, statement.Kind)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported create table, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateTable {
		t.Fatalf("expected create_table ddl payload, got %#v", statement.DDL)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "orders" {
		t.Fatalf("expected table name orders, got %#v", statement.DDL.Table)
	}
	if len(statement.DDL.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(statement.DDL.Constraints))
	}
	constraint := statement.DDL.Constraints[0]
	if constraint.Type != "foreign_key" || constraint.Name != "fk_orders_user" {
		t.Fatalf("expected named foreign_key constraint, got %+v", constraint)
	}
	if len(constraint.Columns) != 1 || constraint.Columns[0] != "user_id" {
		t.Fatalf("expected local columns [user_id], got %#v", constraint.Columns)
	}
	if constraint.ReferencedTable != "users" {
		t.Fatalf("expected referenced table users, got %q", constraint.ReferencedTable)
	}
	if len(constraint.ReferencedColumns) != 1 || constraint.ReferencedColumns[0] != "id" {
		t.Fatalf("expected referenced columns [id], got %#v", constraint.ReferencedColumns)
	}
}

func TestExtractCreateTableInlineReferencesPreservesReferencedTableAndColumns(t *testing.T) {
	sql := `create table orders (
		user_id bigint references users(id)
	);`
	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind %q, got %q", spec.KindDDL, statement.Kind)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported create table, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateTable {
		t.Fatalf("expected create_table ddl payload, got %#v", statement.DDL)
	}
	if statement.DDL.Table == nil || statement.DDL.Table.Name != "orders" {
		t.Fatalf("expected table name orders, got %#v", statement.DDL.Table)
	}
	if len(statement.DDL.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(statement.DDL.Constraints))
	}
	constraint := statement.DDL.Constraints[0]
	if constraint.Type != "foreign_key" {
		t.Fatalf("expected foreign_key constraint, got %+v", constraint)
	}
	if len(constraint.Columns) != 1 || constraint.Columns[0] != "user_id" {
		t.Fatalf("expected local columns [user_id], got %#v", constraint.Columns)
	}
	if constraint.ReferencedTable != "users" {
		t.Fatalf("expected referenced table users, got %q", constraint.ReferencedTable)
	}
	if len(constraint.ReferencedColumns) != 1 || constraint.ReferencedColumns[0] != "id" {
		t.Fatalf("expected referenced columns [id], got %#v", constraint.ReferencedColumns)
	}
}

func extractPostgreSQLStatement(t *testing.T, sql string) spec.Statement {
	t.Helper()

	parser := New()
	result, err := parser.Parse(sql)
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
	return statement
}

// parseCreateStmtAST parses SQL and returns the raw *pg_query.CreateStmt node
// for direct AST inspection. Used by characterization tests.
func parseCreateStmtAST(t *testing.T, sql string) *pg_query.CreateStmt {
	t.Helper()
	result, err := pg_query.Parse(sql)
	if err != nil {
		t.Fatalf("pg_query.Parse: %v", err)
	}
	stmts := result.GetStmts()
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	node := stmts[0].GetStmt()
	if node == nil {
		t.Fatal("stmt node is nil")
	}
	createStmt, ok := node.GetNode().(*pg_query.Node_CreateStmt)
	if !ok {
		t.Fatalf("expected *Node_CreateStmt, got %T", node.GetNode())
	}
	return createStmt.CreateStmt
}

func parseAlterTableStmtAST(t *testing.T, sql string) *pg_query.AlterTableStmt {
	t.Helper()
	result, err := pg_query.Parse(sql)
	if err != nil {
		t.Fatalf("pg_query.Parse: %v", err)
	}
	stmts := result.GetStmts()
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	node := stmts[0].GetStmt()
	if node == nil {
		t.Fatal("stmt node is nil")
	}
	alterStmt, ok := node.GetNode().(*pg_query.Node_AlterTableStmt)
	if !ok {
		t.Fatalf("expected *Node_AlterTableStmt, got %T", node.GetNode())
	}
	return alterStmt.AlterTableStmt
}

// ---------------------------------------------------------------------------
// v0.26.0 Task 2: Behavior tests — unsupported CREATE TABLE boundaries
// ---------------------------------------------------------------------------
// These tests assert that the extractor correctly rejects unsupported
// PostgreSQL CREATE TABLE features via the unsupportedStatement contract.
// ---------------------------------------------------------------------------

func TestExtractCreateTableIdentityColumnReturnsUnsupported(t *testing.T) {
	sql := `CREATE TABLE users (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email text
);`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected kind %q, got %q", spec.KindUnknown, statement.Kind)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail for GENERATED AS IDENTITY column")
	}
	if statement.Unsupported.Feature != "generated_as_identity" {
		t.Fatalf("expected unsupported feature 'generated_as_identity', got %q", statement.Unsupported.Feature)
	}
	if statement.Unsupported.Reason == "" {
		t.Fatal("expected non-empty unsupported reason")
	}
	t.Logf("unsupported: feature=%q reason=%q", statement.Unsupported.Feature, statement.Unsupported.Reason)
}

func TestExtractCreateTableGeneratedStoredColumnReturnsUnsupported(t *testing.T) {
	sql := `CREATE TABLE users (
  first_name text,
  last_name text,
  full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED
);`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected kind %q, got %q", spec.KindUnknown, statement.Kind)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail for GENERATED ALWAYS AS (...) STORED column")
	}
	if statement.Unsupported.Feature != "generated_column" {
		t.Fatalf("expected unsupported feature 'generated_column', got %q", statement.Unsupported.Feature)
	}
	if statement.Unsupported.Reason == "" {
		t.Fatal("expected non-empty unsupported reason")
	}
	t.Logf("unsupported: feature=%q reason=%q", statement.Unsupported.Feature, statement.Unsupported.Reason)
}

func TestExtractCreateTableExclusionConstraintReturnsUnsupported(t *testing.T) {
	sql := `CREATE TABLE bookings (
  room_id int,
  during tsrange,
  EXCLUDE USING gist (room_id WITH =, during WITH &&)
);`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected kind %q, got %q", spec.KindUnknown, statement.Kind)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail for EXCLUDE constraint")
	}
	if statement.Unsupported.Feature != "exclusion_constraint" {
		t.Fatalf("expected unsupported feature 'exclusion_constraint', got %q", statement.Unsupported.Feature)
	}
	if statement.Unsupported.Reason == "" {
		t.Fatal("expected non-empty unsupported reason")
	}
	t.Logf("unsupported: feature=%q reason=%q", statement.Unsupported.Feature, statement.Unsupported.Reason)
}

func TestExtractCreateTablePartitionByStillReturnsUnsupported(t *testing.T) {
	sql := `CREATE TABLE events (
  id bigint,
  created_at timestamptz NOT NULL
) PARTITION BY RANGE (created_at);`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected kind %q, got %q", spec.KindUnknown, statement.Kind)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail for PARTITION BY")
	}
	if statement.Unsupported.Feature != "partitioning" {
		t.Fatalf("expected unsupported feature 'partitioning', got %q", statement.Unsupported.Feature)
	}
	if statement.Unsupported.Reason == "" {
		t.Fatal("expected non-empty unsupported reason")
	}
	t.Logf("unsupported: feature=%q reason=%q", statement.Unsupported.Feature, statement.Unsupported.Reason)
}

// ---------------------------------------------------------------------------
// v0.26.0 Task 1: AST characterization tests for CREATE TABLE boundary cases
// ---------------------------------------------------------------------------
// These tests lock down AST facts about how pg_query_go/v6 represents
// PostgreSQL features that are currently unsupported by the extractor.
// They do NOT assert extractor behavior — only the raw AST shape so that
// Task 2+ can make informed changes.
// ---------------------------------------------------------------------------

func TestParseCreateTableIdentityColumnAST(t *testing.T) {
	sql := `CREATE TABLE users (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email text
);`

	stmt := parseCreateStmtAST(t, sql)

	// There should be 2 table elements (id column, email column).
	// PRIMARY KEY is an inline column constraint, not a separate table element.
	elts := stmt.GetTableElts()
	if len(elts) != 2 {
		t.Fatalf("expected 2 table elements, got %d", len(elts))
	}

	// First element must be a ColumnDef.
	colNode := elts[0].GetColumnDef()
	if colNode == nil {
		t.Fatal("first table element is not a ColumnDef")
	}

	// FACT 1: ColumnDef.Identity is EMPTY for GENERATED ALWAYS AS IDENTITY.
	// Despite the protobuf field existing, pg_query_go/v6 represents identity
	// through the column constraint list (CONSTR_IDENTITY), not via this field.
	identity := colNode.GetIdentity()
	t.Logf("ColumnDef.Identity = %q", identity)
	if identity != "" {
		t.Logf("ColumnDef.Identity is non-empty (%q) — the hypothesis that it is always empty was wrong", identity)
	}

	// FACT 2: Identity is represented as CONSTR_IDENTITY (4) in the column
	// constraint list, with GeneratedWhen="a" (ALWAYS).
	constraints := colNode.GetConstraints()
	t.Logf("ColumnDef has %d inline constraints", len(constraints))
	var identityConstraint *pg_query.Constraint
	for _, c := range constraints {
		con := c.GetConstraint()
		if con != nil && con.GetContype() == pg_query.ConstrType_CONSTR_IDENTITY {
			identityConstraint = con
			break
		}
	}
	if identityConstraint == nil {
		t.Fatal("expected CONSTR_IDENTITY in column constraint list for GENERATED ALWAYS AS IDENTITY")
	}
	t.Logf("CONSTR_IDENTITY.GeneratedWhen = %q", identityConstraint.GetGeneratedWhen())
	if identityConstraint.GetGeneratedWhen() != "a" {
		t.Fatalf("expected GeneratedWhen='a' for ALWAYS, got %q", identityConstraint.GetGeneratedWhen())
	}

	// FACT 3: PRIMARY KEY appears as CONSTR_PRIMARY in the column constraint list.
	foundPrimary := false
	for _, c := range constraints {
		con := c.GetConstraint()
		if con != nil && con.GetContype() == pg_query.ConstrType_CONSTR_PRIMARY {
			foundPrimary = true
			break
		}
	}
	if !foundPrimary {
		t.Fatal("expected CONSTR_PRIMARY in column constraint list for PRIMARY KEY")
	}
}

func TestParseCreateTableGeneratedStoredColumnAST(t *testing.T) {
	sql := `CREATE TABLE users (
  first_name text,
  last_name text,
  full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED
);`

	stmt := parseCreateStmtAST(t, sql)

	elts := stmt.GetTableElts()
	if len(elts) != 3 {
		t.Fatalf("expected 3 table elements, got %d", len(elts))
	}

	// Third element is the generated column.
	colNode := elts[2].GetColumnDef()
	if colNode == nil {
		t.Fatal("third table element is not a ColumnDef")
	}
	if colNode.GetColname() != "full_name" {
		t.Fatalf("expected column name 'full_name', got %q", colNode.GetColname())
	}

	// FACT 1: ColumnDef.Generated is EMPTY for GENERATED ALWAYS AS (...) STORED.
	// Despite the protobuf field existing, pg_query_go/v6 represents generated
	// columns through the column constraint list (CONSTR_GENERATED), not via this field.
	generated := colNode.GetGenerated()
	t.Logf("ColumnDef.Generated = %q", generated)
	if generated != "" {
		t.Logf("ColumnDef.Generated is non-empty (%q) — the hypothesis that it is always empty was wrong", generated)
	}

	// FACT 2: ColumnDef.Identity is also empty (this is a generated column, not identity).
	identity := colNode.GetIdentity()
	t.Logf("ColumnDef.Identity = %q", identity)
	if identity != "" {
		t.Fatalf("expected ColumnDef.Identity to be empty for GENERATED column, got %q", identity)
	}

	// FACT 3: The generated expression is represented as CONSTR_GENERATED (5) in
	// the column constraint list, with GeneratedWhen="a" (ALWAYS) and RawExpr populated.
	constraints := colNode.GetConstraints()
	t.Logf("ColumnDef has %d inline constraints", len(constraints))
	var generatedConstraint *pg_query.Constraint
	for _, c := range constraints {
		con := c.GetConstraint()
		if con != nil && con.GetContype() == pg_query.ConstrType_CONSTR_GENERATED {
			generatedConstraint = con
			break
		}
	}
	if generatedConstraint == nil {
		t.Fatal("expected CONSTR_GENERATED in column constraint list for GENERATED ALWAYS AS (...) STORED")
	}

	// FACT 4: GeneratedWhen is "a" for ALWAYS.
	gw := generatedConstraint.GetGeneratedWhen()
	t.Logf("CONSTR_GENERATED.GeneratedWhen = %q", gw)
	if gw != "a" {
		t.Fatalf("expected GeneratedWhen='a' for ALWAYS, got %q", gw)
	}

	// FACT 5: RawExpr is populated with the generation expression.
	if generatedConstraint.GetRawExpr() == nil {
		t.Fatal("expected CONSTR_GENERATED.RawExpr to be non-nil for generated column expression")
	}
}

func TestParseAlterTableAddGeneratedStoredColumnAST(t *testing.T) {
	sql := `ALTER TABLE users
  ADD COLUMN full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED;`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddColumn {
		t.Fatalf("expected subtype AT_AddColumn, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}
	column := cmd.GetDef().GetColumnDef()
	if column == nil {
		t.Fatal("expected add-column command def to be ColumnDef")
	}
	if column.GetColname() != "full_name" {
		t.Fatalf("expected column name full_name, got %q", column.GetColname())
	}
	if column.GetGenerated() != "" {
		t.Fatalf("expected ColumnDef.Generated to be empty, got %q", column.GetGenerated())
	}
	constraints := column.GetConstraints()
	if len(constraints) != 1 {
		t.Fatalf("expected 1 column constraint, got %d", len(constraints))
	}
	constraint := constraints[0].GetConstraint()
	if constraint == nil {
		t.Fatal("expected generated constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_GENERATED {
		t.Fatalf("expected CONSTR_GENERATED, got %s (%d)", constraint.GetContype().String(), constraint.GetContype())
	}
	if constraint.GetGeneratedWhen() != "a" {
		t.Fatalf("expected GeneratedWhen='a', got %q", constraint.GetGeneratedWhen())
	}
	if constraint.GetRawExpr() == nil {
		t.Fatal("expected CONSTR_GENERATED.RawExpr to be non-nil")
	}
}

func TestParseAlterTableAddIdentityColumnAST(t *testing.T) {
	sql := `ALTER TABLE users
  ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddColumn {
		t.Fatalf("expected subtype AT_AddColumn, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}
	column := cmd.GetDef().GetColumnDef()
	if column == nil {
		t.Fatal("expected add-column command def to be ColumnDef")
	}
	if column.GetColname() != "id" {
		t.Fatalf("expected column name id, got %q", column.GetColname())
	}
	if column.GetIdentity() != "" {
		t.Fatalf("expected ColumnDef.Identity to be empty, got %q", column.GetIdentity())
	}
	constraints := column.GetConstraints()
	if len(constraints) != 1 {
		t.Fatalf("expected 1 column constraint, got %d", len(constraints))
	}
	constraint := constraints[0].GetConstraint()
	if constraint == nil {
		t.Fatal("expected identity constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_IDENTITY {
		t.Fatalf("expected CONSTR_IDENTITY, got %s (%d)", constraint.GetContype().String(), constraint.GetContype())
	}
	if constraint.GetGeneratedWhen() != "a" {
		t.Fatalf("expected GeneratedWhen='a', got %q", constraint.GetGeneratedWhen())
	}
}

func TestParseAlterTableDropGeneratedExpressionAST(t *testing.T) {
	sql := `ALTER TABLE users
  ALTER COLUMN full_name DROP EXPRESSION;`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_DropExpression {
		t.Fatalf("expected subtype AT_DropExpression, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}
	if cmd.GetName() != "full_name" {
		t.Fatalf("expected column name full_name, got %q", cmd.GetName())
	}
	if cmd.GetDef() != nil {
		t.Fatalf("expected AT_DropExpression def to be nil, got %T", cmd.GetDef().GetNode())
	}
}

func TestParseAlterTableSetIdentityGeneratedAST(t *testing.T) {
	sql := `ALTER TABLE users
  ALTER COLUMN id SET GENERATED BY DEFAULT;`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_SetIdentity {
		t.Fatalf("expected subtype AT_SetIdentity, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}
	if cmd.GetName() != "id" {
		t.Fatalf("expected column name id, got %q", cmd.GetName())
	}
	listNode := cmd.GetDef().GetList()
	if listNode == nil {
		t.Fatal("expected AT_SetIdentity def to be a List")
	}
	items := listNode.GetItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 defelem item, got %d", len(items))
	}
	defElem := items[0].GetDefElem()
	if defElem == nil {
		t.Fatal("expected first list item to be DefElem")
	}
	if defElem.GetDefname() != "generated" {
		t.Fatalf("expected defname generated, got %q", defElem.GetDefname())
	}
	arg := defElem.GetArg()
	if arg == nil || arg.GetInteger() == nil {
		t.Fatal("expected defelem arg integer for generated setting")
	}
	if arg.GetInteger().GetIval() != 100 {
		t.Fatalf("expected integer 100 for BY DEFAULT, got %d", arg.GetInteger().GetIval())
	}
}

func TestParseAlterTableSetIdentityGeneratedAlwaysAST(t *testing.T) {
	sql := `ALTER TABLE users
  ALTER COLUMN id SET GENERATED ALWAYS;`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_SetIdentity {
		t.Fatalf("expected subtype AT_SetIdentity, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}
	if cmd.GetName() != "id" {
		t.Fatalf("expected column name id, got %q", cmd.GetName())
	}
	listNode := cmd.GetDef().GetList()
	if listNode == nil {
		t.Fatal("expected AT_SetIdentity def to be a List")
	}
	items := listNode.GetItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 defelem item, got %d", len(items))
	}
	defElem := items[0].GetDefElem()
	if defElem == nil {
		t.Fatal("expected first list item to be DefElem")
	}
	if defElem.GetDefname() != "generated" {
		t.Fatalf("expected defname generated, got %q", defElem.GetDefname())
	}
	arg := defElem.GetArg()
	if arg == nil || arg.GetInteger() == nil {
		t.Fatal("expected defelem arg integer for generated setting")
	}
}

func TestParseAlterTableDropIdentityAST(t *testing.T) {
	sql := `ALTER TABLE users
  ALTER COLUMN id DROP IDENTITY;`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_DropIdentity {
		t.Fatalf("expected subtype AT_DropIdentity, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}
	if cmd.GetName() != "id" {
		t.Fatalf("expected column name id, got %q", cmd.GetName())
	}
	if cmd.GetDef() != nil {
		t.Fatalf("expected AT_DropIdentity def to be nil, got %T", cmd.GetDef().GetNode())
	}
}

func TestExtractAlterTableAddGeneratedStoredColumnReturnsUnsupported(t *testing.T) {
	sql := `ALTER TABLE users
  ADD COLUMN full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED;`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected generated add-column to be unsupported, got kind=%q ddl=%#v", statement.Kind, statement.DDL)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail for generated add-column")
	}
	if statement.Unsupported.Feature != "generated_column" {
		t.Fatalf("expected unsupported feature generated_column, got %q", statement.Unsupported.Feature)
	}
	if statement.Unsupported.Reason == "" {
		t.Fatal("expected non-empty unsupported reason")
	}
}

func TestExtractAlterTableAddIdentityColumnReturnsUnsupported(t *testing.T) {
	sql := `ALTER TABLE users
  ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected identity add-column to be unsupported, got kind=%q ddl=%#v", statement.Kind, statement.DDL)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail for identity add-column")
	}
	if statement.Unsupported.Feature != "generated_as_identity" {
		t.Fatalf("expected unsupported feature generated_as_identity, got %q", statement.Unsupported.Feature)
	}
	if statement.Unsupported.Reason == "" {
		t.Fatal("expected non-empty unsupported reason")
	}
}

func TestExtractAlterTableDropGeneratedExpressionCurrentBehavior(t *testing.T) {
	sql := `ALTER TABLE users
  ALTER COLUMN full_name DROP EXPRESSION;`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected drop expression to remain unsupported today, got kind=%q ddl=%#v", statement.Kind, statement.DDL)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail for DROP EXPRESSION current behavior")
	}
	if statement.Unsupported.Feature != "dropexpression" {
		t.Fatalf("expected unsupported feature dropexpression, got %q", statement.Unsupported.Feature)
	}
}

func TestExtractAlterTableSetIdentityGeneratedCurrentBehavior(t *testing.T) {
	sql := `ALTER TABLE users
  ALTER COLUMN id SET GENERATED BY DEFAULT;`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected set identity generated to remain unsupported today, got kind=%q ddl=%#v", statement.Kind, statement.DDL)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail for SET GENERATED current behavior")
	}
	if statement.Unsupported.Feature != "setidentity" {
		t.Fatalf("expected unsupported feature setidentity, got %q", statement.Unsupported.Feature)
	}
}

func TestExtractAlterTableDropIdentityCurrentBehavior(t *testing.T) {
	sql := `ALTER TABLE users
  ALTER COLUMN id DROP IDENTITY;`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected drop identity to remain unsupported today, got kind=%q ddl=%#v", statement.Kind, statement.DDL)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail for DROP IDENTITY current behavior")
	}
	if statement.Unsupported.Feature != "dropidentity" {
		t.Fatalf("expected unsupported feature dropidentity, got %q", statement.Unsupported.Feature)
	}
}

func TestParseCreateTableExclusionConstraintAST(t *testing.T) {
	sql := `CREATE TABLE bookings (
  room_id int,
  during tsrange,
  EXCLUDE USING gist (room_id WITH =, during WITH &&)
);`

	stmt := parseCreateStmtAST(t, sql)

	elts := stmt.GetTableElts()
	if len(elts) != 3 {
		t.Fatalf("expected 3 table elements, got %d", len(elts))
	}

	// Third element should be the EXCLUDE constraint (a table-level constraint).
	constraintNode := elts[2].GetConstraint()
	if constraintNode == nil {
		t.Fatal("third table element is not a Constraint node")
	}

	// FACT 1: The exclusion constraint has contype CONSTR_EXCLUSION (9).
	contype := constraintNode.GetContype()
	t.Logf("Constraint.Contype = %s (%d)", contype.String(), contype)
	if contype != pg_query.ConstrType_CONSTR_EXCLUSION {
		t.Fatalf("expected CONSTR_EXCLUSION, got %s (%d)", contype.String(), contype)
	}

	// FACT 2: The access method ("gist") is stored in Constraint.AccessMethod.
	accessMethod := constraintNode.GetAccessMethod()
	t.Logf("Constraint.AccessMethod = %q", accessMethod)
	if accessMethod != "gist" {
		t.Fatalf("expected access method 'gist', got %q", accessMethod)
	}

	// FACT 3: The exclusion elements are stored in Constraint.Exclusions.
	exclusions := constraintNode.GetExclusions()
	t.Logf("Constraint.Exclusions count = %d", len(exclusions))
	if len(exclusions) == 0 {
		t.Fatal("expected non-empty Exclusions list for EXCLUDE constraint")
	}

	// FACT 4: The constraint name is empty (unnamed exclusion constraint).
	conname := constraintNode.GetConname()
	t.Logf("Constraint.Conname = %q", conname)

	// FACT 5: Verify the exclusion constraint has no FK/PK related fields populated.
	if constraintNode.GetFkAttrs() != nil {
		t.Log("Constraint.FkAttrs is populated (unexpected for EXCLUSION)")
	}
}

func TestParseCreateTablePartitionByAST(t *testing.T) {
	sql := `CREATE TABLE events (
  id bigint,
  created_at timestamptz NOT NULL
) PARTITION BY RANGE (created_at);`

	stmt := parseCreateStmtAST(t, sql)

	// FACT 1: CreateStmt.Partspec is non-nil for PARTITION BY.
	partspec := stmt.GetPartspec()
	t.Logf("CreateStmt.Partspec != nil: %v", partspec != nil)
	if partspec == nil {
		t.Fatal("expected CreateStmt.Partspec to be non-nil for PARTITION BY RANGE")
	}

	// FACT 2: The partition strategy is RANGE (PartitionStrategy_PARTITION_STRATEGY_RANGE = 2).
	strategy := partspec.GetStrategy()
	t.Logf("PartitionSpec.Strategy = %s (%d)", strategy.String(), strategy)
	if strategy != pg_query.PartitionStrategy_PARTITION_STRATEGY_RANGE {
		t.Fatalf("expected PARTITION_STRATEGY_RANGE, got %s (%d)", strategy.String(), strategy)
	}

	// FACT 3: PartitionSpec.PartParams contains the partition key columns.
	partParams := partspec.GetPartParams()
	t.Logf("PartitionSpec.PartParams count = %d", len(partParams))
	if len(partParams) == 0 {
		t.Fatal("expected non-empty PartParams for PARTITION BY RANGE")
	}

	// FACT 4: CreateStmt.Partbound should be nil for a top-level partitioned table
	// (Partbound is used when attaching a partition, not when declaring the parent).
	partbound := stmt.GetPartbound()
	t.Logf("CreateStmt.Partbound != nil: %v", partbound != nil)
	if partbound != nil {
		t.Log("CreateStmt.Partbound is non-nil — this is unexpected for a partitioned parent table")
	}

	// FACT 5: TableElts should still contain the regular column definitions.
	elts := stmt.GetTableElts()
	if len(elts) != 2 {
		t.Fatalf("expected 2 table elements, got %d", len(elts))
	}
}

// ---------------------------------------------------------------------------
// v0.27.0 Task 1: Schema-qualified REFERENCES characterization tests
// ---------------------------------------------------------------------------
// These tests prove three things:
//  1. pg_query.RangeVar carries both Schemaname and Relname for schema-qualified
//     REFERENCES (inline and table-level).
//  2. The current extractor drops schema because rangeVarName() only returns
//     Relname — this is an extractor design choice, not an AST limitation.
//  3. Adding ReferencedSchema to spec.Constraint would be a pure additive change.
//
// No production code is modified. These are decision-gate tests.
// ---------------------------------------------------------------------------

func TestParseCreateTableInlineSchemaQualifiedReferencesAST(t *testing.T) {
	sql := `CREATE TABLE orders (
  id bigint PRIMARY KEY,
  user_id bigint REFERENCES public.users(id)
);`

	stmt := parseCreateStmtAST(t, sql)

	elts := stmt.GetTableElts()
	if len(elts) != 2 {
		t.Fatalf("expected 2 table elements, got %d", len(elts))
	}

	// Second element is the user_id column with inline REFERENCES.
	colDef := elts[1].GetColumnDef()
	if colDef == nil {
		t.Fatal("expected second table element to be a ColumnDef")
	}
	if colDef.GetColname() != "user_id" {
		t.Fatalf("expected column name user_id, got %q", colDef.GetColname())
	}

	// ColumnDef should have exactly one constraint (the REFERENCES).
	colConstraints := colDef.GetConstraints()
	if len(colConstraints) != 1 {
		t.Fatalf("expected 1 column constraint, got %d", len(colConstraints))
	}
	constraint := colConstraints[0].GetConstraint()
	if constraint == nil {
		t.Fatal("expected Constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_FOREIGN {
		t.Fatalf("expected CONSTR_FOREIGN, got %s", constraint.GetContype())
	}

	// FACT 1: Pktable (the referenced table) is a RangeVar that carries
	// both Schemaname and Relname for schema-qualified references.
	pkTable := constraint.GetPktable()
	if pkTable == nil {
		t.Fatal("expected non-nil Pktable (RangeVar)")
	}
	schemaName := pkTable.GetSchemaname()
	relName := pkTable.GetRelname()

	t.Logf("Inline REFERENCES Pktable.Schemaname = %q", schemaName)
	t.Logf("Inline REFERENCES Pktable.Relname    = %q", relName)

	if schemaName != "public" {
		t.Fatalf("expected Pktable.Schemaname %q, got %q — AST does expose schema for inline REFERENCES", "public", schemaName)
	}
	if relName != "users" {
		t.Fatalf("expected Pktable.Relname %q, got %q", "users", relName)
	}

	// FACT 2: PkAttrs carries the referenced column name.
	pkAttrs := constraint.GetPkAttrs()
	if len(pkAttrs) != 1 {
		t.Fatalf("expected 1 pk_attr, got %d", len(pkAttrs))
	}
	if stringNodeValue(pkAttrs[0]) != "id" {
		t.Fatalf("expected pk_attr id, got %q", stringNodeValue(pkAttrs[0]))
	}
}

func TestParseCreateTableTableLevelSchemaQualifiedForeignKeyAST(t *testing.T) {
	sql := `CREATE TABLE orders (
  id bigint PRIMARY KEY,
  user_id bigint,
  CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES public.users(id)
);`

	stmt := parseCreateStmtAST(t, sql)

	elts := stmt.GetTableElts()
	if len(elts) != 3 {
		t.Fatalf("expected 3 table elements, got %d", len(elts))
	}

	// Third element is the named table-level CONSTRAINT.
	constraintNode := elts[2].GetConstraint()
	if constraintNode == nil {
		t.Fatal("expected third table element to be a Constraint")
	}
	if constraintNode.GetConname() != "fk_orders_user" {
		t.Fatalf("expected constraint name fk_orders_user, got %q", constraintNode.GetConname())
	}
	if constraintNode.GetContype() != pg_query.ConstrType_CONSTR_FOREIGN {
		t.Fatalf("expected CONSTR_FOREIGN, got %s", constraintNode.GetContype())
	}

	// FACT 1: Pktable carries schema for table-level FK too.
	pkTable := constraintNode.GetPktable()
	if pkTable == nil {
		t.Fatal("expected non-nil Pktable (RangeVar)")
	}
	schemaName := pkTable.GetSchemaname()
	relName := pkTable.GetRelname()

	t.Logf("Table-level FK Pktable.Schemaname = %q", schemaName)
	t.Logf("Table-level FK Pktable.Relname    = %q", relName)

	if schemaName != "public" {
		t.Fatalf("expected Pktable.Schemaname %q, got %q — AST exposes schema for table-level FK too", "public", schemaName)
	}
	if relName != "users" {
		t.Fatalf("expected Pktable.Relname %q, got %q", "users", relName)
	}

	// FACT 2: FkAttrs and PkAttrs are populated correctly.
	fkAttrs := constraintNode.GetFkAttrs()
	if len(fkAttrs) != 1 || stringNodeValue(fkAttrs[0]) != "user_id" {
		t.Fatalf("expected fk_attrs [user_id], got %#v", fkAttrs)
	}
	pkAttrs := constraintNode.GetPkAttrs()
	if len(pkAttrs) != 1 || stringNodeValue(pkAttrs[0]) != "id" {
		t.Fatalf("expected pk_attrs [id], got %#v", pkAttrs)
	}
}

func TestExtractCreateTableInlineSchemaQualifiedReferencesDropsSchemaToday(t *testing.T) {
	sql := `CREATE TABLE orders (
  id bigint PRIMARY KEY,
  user_id bigint REFERENCES public.users(id)
);`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind %q, got %q", spec.KindDDL, statement.Kind)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported create table, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(statement.DDL.Constraints))
	}

	constraint := statement.DDL.Constraints[0]

	// FACT: The extractor currently preserves the bare table name.
	if constraint.ReferencedTable != "users" {
		t.Fatalf("expected ReferencedTable %q, got %q", "users", constraint.ReferencedTable)
	}
	if len(constraint.ReferencedColumns) != 1 || constraint.ReferencedColumns[0] != "id" {
		t.Fatalf("expected ReferencedColumns [id], got %#v", constraint.ReferencedColumns)
	}

	// FACT: The spec.Constraint struct has no ReferencedSchema field today.
	// This test documents the current gap: schema "public" is available in the
	// AST (proven by TestParseCreateTableInlineSchemaQualifiedReferencesAST)
	// but is silently discarded by rangeVarName() which only returns Relname.
	//
	// Once ReferencedSchema is added, this test should be updated to assert:
	//   constraint.ReferencedSchema == "public"
	t.Logf("Current behavior: ReferencedTable=%q, schema info is lost", constraint.ReferencedTable)
	t.Log("Gap confirmed: pg_query.RangeVar has Schemaname=public but extractor drops it via rangeVarName()")
}

func TestExtractCreateTableTableLevelSchemaQualifiedForeignKeyDropsSchemaToday(t *testing.T) {
	sql := `CREATE TABLE orders (
  id bigint PRIMARY KEY,
  user_id bigint,
  CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES public.users(id)
);`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind %q, got %q", spec.KindDDL, statement.Kind)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported create table, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(statement.DDL.Constraints))
	}

	constraint := statement.DDL.Constraints[0]
	if constraint.Type != "foreign_key" || constraint.Name != "fk_orders_user" {
		t.Fatalf("expected named foreign_key constraint, got %+v", constraint)
	}

	// FACT: The extractor currently preserves the bare table name.
	if constraint.ReferencedTable != "users" {
		t.Fatalf("expected ReferencedTable %q, got %q", "users", constraint.ReferencedTable)
	}
	if len(constraint.ReferencedColumns) != 1 || constraint.ReferencedColumns[0] != "id" {
		t.Fatalf("expected ReferencedColumns [id], got %#v", constraint.ReferencedColumns)
	}

	// FACT: Same gap as inline case — schema is available in AST but discarded.
	// Both applyTableConstraint and applyColumnConstraints use rangeVarName()
	// which only extracts Relname, not Schemaname.
	t.Logf("Current behavior: ReferencedTable=%q, schema info is lost", constraint.ReferencedTable)
	t.Log("Gap confirmed: applies to both inline REFERENCES and table-level FOREIGN KEY")
}

// ---------------------------------------------------------------------------
// v0.27.0 Task 2: Behavior tests — schema-qualified REFERENCES preservation
// ---------------------------------------------------------------------------
// These tests assert that the extractor preserves schema-qualified reference
// semantics via the new ReferencedSchema field on spec.Constraint.
// ---------------------------------------------------------------------------

func TestExtractCreateTableInlineSchemaQualifiedReferencesPreservesReferencedSchema(t *testing.T) {
	sql := `CREATE TABLE orders (
  id bigint PRIMARY KEY,
  user_id bigint REFERENCES public.users(id)
);`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind %q, got %q", spec.KindDDL, statement.Kind)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported create table, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateTable {
		t.Fatalf("expected create_table ddl payload, got %#v", statement.DDL)
	}
	if len(statement.DDL.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(statement.DDL.Constraints))
	}

	constraint := statement.DDL.Constraints[0]
	if constraint.Type != "foreign_key" {
		t.Fatalf("expected foreign_key constraint, got %+v", constraint)
	}
	if constraint.ReferencedSchema != "public" {
		t.Fatalf("expected ReferencedSchema %q, got %q", "public", constraint.ReferencedSchema)
	}
	if constraint.ReferencedTable != "users" {
		t.Fatalf("expected ReferencedTable %q, got %q", "users", constraint.ReferencedTable)
	}
	if len(constraint.ReferencedColumns) != 1 || constraint.ReferencedColumns[0] != "id" {
		t.Fatalf("expected ReferencedColumns [id], got %#v", constraint.ReferencedColumns)
	}
}

func TestExtractCreateTableTableLevelSchemaQualifiedForeignKeyPreservesReferencedSchema(t *testing.T) {
	sql := `CREATE TABLE orders (
  id bigint PRIMARY KEY,
  user_id bigint,
  CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES public.users(id)
);`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind %q, got %q", spec.KindDDL, statement.Kind)
	}
	if statement.Unsupported != nil {
		t.Fatalf("expected supported create table, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateTable {
		t.Fatalf("expected create_table ddl payload, got %#v", statement.DDL)
	}
	if len(statement.DDL.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(statement.DDL.Constraints))
	}

	constraint := statement.DDL.Constraints[0]
	if constraint.Type != "foreign_key" || constraint.Name != "fk_orders_user" {
		t.Fatalf("expected named foreign_key constraint, got %+v", constraint)
	}
	if constraint.ReferencedSchema != "public" {
		t.Fatalf("expected ReferencedSchema %q, got %q", "public", constraint.ReferencedSchema)
	}
	if constraint.ReferencedTable != "users" {
		t.Fatalf("expected ReferencedTable %q, got %q", "users", constraint.ReferencedTable)
	}
	if len(constraint.ReferencedColumns) != 1 || constraint.ReferencedColumns[0] != "id" {
		t.Fatalf("expected ReferencedColumns [id], got %#v", constraint.ReferencedColumns)
	}
}
