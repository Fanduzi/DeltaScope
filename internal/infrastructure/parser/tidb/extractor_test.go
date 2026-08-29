// Package tidbparser verifies TiDB AST extraction into parser-neutral specs.
// input: TiDB parser statements covering DDL and DML extraction paths
// output: coverage for TiDB extractor normalization into domain spec statements
// pos: infrastructure parser extractor test coverage
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

func TestExtractorCreateTableCapturesStructuralFacts(t *testing.T) {
	t.Parallel()
	stmt := extractSingleStatement(t, `
		create table app.users (
			id bigint unsigned not null auto_increment,
			role_id bigint not null,
			email varchar(255) collate utf8mb4_bin default 'anon@example.com',
			created_at timestamp default current_timestamp on update current_timestamp,
			note varchar(64) comment 'memo',
			primary key (id),
			unique key uniq_email (email),
			key idx_created_at (created_at),
			constraint fk_users_role foreign key (role_id) references roles(id)
		) engine=InnoDB charset=utf8mb4 row_format=dynamic comment='user table' auto_increment=42;
	`)

	if stmt.Kind != spec.KindDDL || stmt.DDL == nil {
		t.Fatalf("expected ddl statement, got %#v", stmt)
	}
	if stmt.DDL.Operation != spec.DDLOperationCreateTable {
		t.Fatalf("expected create_table operation, got %q", stmt.DDL.Operation)
	}
	if stmt.DDL.Table == nil || stmt.DDL.Table.Schema != "app" || stmt.DDL.Table.Name != "users" {
		t.Fatalf("expected app.users table, got %#v", stmt.DDL.Table)
	}
	if stmt.DDL.Table.Comment != "user table" {
		t.Fatalf("expected table comment to be extracted, got %#v", stmt.DDL.Table)
	}
	if len(stmt.DDL.Columns) != 5 {
		t.Fatalf("expected 5 columns, got %#v", stmt.DDL.Columns)
	}

	id := stmt.DDL.Columns[0]
	if id.Name != "id" || !strings.Contains(id.Type, "bigint") || !id.Unsigned || !id.NotNull || !id.AutoIncrement {
		t.Fatalf("expected id column facts, got %#v", id)
	}
	email := stmt.DDL.Columns[2]
	if !email.HasDefault || email.DefaultValue != "'anon@example.com'" || email.Collation != "utf8mb4_bin" {
		t.Fatalf("expected email default/collation facts, got %#v", email)
	}
	createdAt := stmt.DDL.Columns[3]
	if !createdAt.DefaultIsCurrentTimestamp || !createdAt.OnUpdateCurrentTimestamp {
		t.Fatalf("expected created_at timestamp facts, got %#v", createdAt)
	}
	note := stmt.DDL.Columns[4]
	if note.Comment != "'memo'" {
		t.Fatalf("expected note column comment, got %#v", note)
	}

	if stmt.DDL.PrimaryKey == nil || stmt.DDL.PrimaryKey.Name != "primary" || stmt.DDL.PrimaryKey.Kind != spec.IndexKindPrimary {
		t.Fatalf("expected extracted primary key, got %#v", stmt.DDL.PrimaryKey)
	}
	if len(stmt.DDL.Indexes) != 2 {
		t.Fatalf("expected 2 extracted indexes, got %#v", stmt.DDL.Indexes)
	}
	if stmt.DDL.Indexes[0].Name != "uniq_email" || stmt.DDL.Indexes[0].Kind != spec.IndexKindUnique {
		t.Fatalf("expected unique index, got %#v", stmt.DDL.Indexes[0])
	}
	if stmt.DDL.Indexes[1].Name != "idx_created_at" || stmt.DDL.Indexes[1].Kind != spec.IndexKindSecondary {
		t.Fatalf("expected secondary index, got %#v", stmt.DDL.Indexes[1])
	}
	if len(stmt.DDL.Constraints) != 1 || stmt.DDL.Constraints[0].Type != "foreign_key" || stmt.DDL.Constraints[0].Name != "fk_users_role" {
		t.Fatalf("expected foreign key constraint, got %#v", stmt.DDL.Constraints)
	}
	for key, want := range map[string]string{
		"engine":         "InnoDB",
		"charset":        "utf8mb4",
		"row_format":     "DYNAMIC",
		"auto_increment": "42",
		"comment":        "user table",
	} {
		if got := stmt.DDL.Options[key]; got != want {
			t.Fatalf("expected option %s=%q, got %q", key, want, got)
		}
	}
}

func TestExtractorAlterTableCapturesColumnAndIndexActions(t *testing.T) {
	t.Parallel()
	stmt := extractSingleStatement(t, `
		alter table app.users
			add column nickname varchar(20) not null default 'anon',
			drop index idx_email,
			rename index idx_old to idx_new,
			drop primary key,
			add unique key uniq_email (email),
			modify column nickname varchar(40) null default 'renamed';
	`)

	if stmt.Kind != spec.KindDDL || stmt.DDL == nil {
		t.Fatalf("expected ddl statement, got %#v", stmt)
	}
	if stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table operation, got %q", stmt.DDL.Operation)
	}
	if len(stmt.DDL.Alter) != 6 {
		t.Fatalf("expected 6 alter actions, got %#v", stmt.DDL.Alter)
	}

	addColumn := findAlter(t, stmt.DDL.Alter, "add_columns", "nickname")
	if addColumn.Column == nil || addColumn.Column.Definition == nil || addColumn.Column.Definition.DefaultValue != "'anon'" {
		t.Fatalf("expected add column definition with default, got %#v", addColumn)
	}

	dropIndex := findAlter(t, stmt.DDL.Alter, "drop_index", "idx_email")
	if dropIndex.Index == nil || dropIndex.Index.OldName != "idx_email" {
		t.Fatalf("expected drop index payload, got %#v", dropIndex)
	}

	renameIndex := findAlter(t, stmt.DDL.Alter, "rename_index", "idx_old")
	if renameIndex.Index == nil || renameIndex.Index.Definition == nil || renameIndex.Index.Definition.Name != "idx_new" {
		t.Fatalf("expected rename index payload, got %#v", renameIndex)
	}

	dropPrimaryKey := findAlter(t, stmt.DDL.Alter, "drop_primary_key", "primary")
	if dropPrimaryKey.Index == nil || dropPrimaryKey.Index.OldName != "primary" {
		t.Fatalf("expected drop primary key payload, got %#v", dropPrimaryKey)
	}

	addIndex := findAlter(t, stmt.DDL.Alter, "add_index", "uniq_email")
	if addIndex.Index == nil || addIndex.Index.Definition == nil || addIndex.Index.Definition.Kind != spec.IndexKindUnique {
		t.Fatalf("expected add unique index payload, got %#v", addIndex)
	}

	modifyColumn := findAlter(t, stmt.DDL.Alter, "modify_column", "nickname")
	if modifyColumn.Column == nil || modifyColumn.Column.Change == nil || !modifyColumn.Column.Change.TouchesNullability || !modifyColumn.Column.Change.TouchesDefault {
		t.Fatalf("expected modify column change facts, got %#v", modifyColumn)
	}
}

func TestExtractorDMLCapturesInsertUpdateDeleteFacts(t *testing.T) {
	t.Parallel()
	statements := extractStatements(t, `
		insert into app.users (id, name) values (1, 'a'), (2, 'b') on duplicate key update name = values(name);
		update app.users set name = 'b' where id = 1;
		delete from app.users where id = 1 limit 1;
	`)

	insertStmt := statements[0]
	if insertStmt.Kind != spec.KindDML || insertStmt.DML == nil {
		t.Fatalf("expected dml insert statement, got %#v", insertStmt)
	}
	if insertStmt.DML.Operation != spec.DMLOperationInsert || insertStmt.DML.InsertRows != 2 || !insertStmt.DML.HasOnDuplicate || insertStmt.DML.IsInsertSelect {
		t.Fatalf("expected insert extraction facts, got %#v", insertStmt.DML)
	}
	if len(insertStmt.DML.Tables) != 1 || insertStmt.DML.Tables[0].Schema != "app" || insertStmt.DML.Tables[0].Name != "users" {
		t.Fatalf("expected insert target table, got %#v", insertStmt.DML.Tables)
	}

	updateStmt := statements[1]
	if updateStmt.DML == nil || updateStmt.DML.Operation != spec.DMLOperationUpdate {
		t.Fatalf("expected update statement, got %#v", updateStmt)
	}
	if !updateStmt.DML.HasWhere || updateStmt.DML.PredicateShape != spec.PredicateShapeUniqueEquality || updateStmt.DML.MatchedKeyName != "PRIMARY" || updateStmt.DML.MatchedKeyKind != spec.IndexKindPrimary {
		t.Fatalf("expected update predicate facts, got %#v", updateStmt.DML)
	}
	if len(updateStmt.DML.LookupColumns) != 1 || updateStmt.DML.LookupColumns[0] != "id" || !updateStmt.DML.IsSingleTable {
		t.Fatalf("expected update lookup facts, got %#v", updateStmt.DML)
	}

	deleteStmt := statements[2]
	if deleteStmt.DML == nil || deleteStmt.DML.Operation != spec.DMLOperationDelete {
		t.Fatalf("expected delete statement, got %#v", deleteStmt)
	}
	if !deleteStmt.DML.HasWhere || !deleteStmt.DML.HasLimit || deleteStmt.DML.PredicateShape != spec.PredicateShapeUniqueEquality {
		t.Fatalf("expected delete predicate facts, got %#v", deleteStmt.DML)
	}
}

func TestExtractorDMLCapturesJoinAndSubqueryShapes(t *testing.T) {
	t.Parallel()
	statements := extractStatements(t, `
		update users u join orgs o on o.id = u.org_id set u.name = 'x' where u.id = 1;
		delete from users where id in (select id from admins);
	`)

	updateStmt := statements[0]
	if updateStmt.DML == nil || !updateStmt.DML.HasJoin || !updateStmt.DML.HasJoinOn || updateStmt.DML.PredicateShape != spec.PredicateShapeJoin {
		t.Fatalf("expected join update facts, got %#v", updateStmt.DML)
	}
	if len(updateStmt.DML.Tables) != 2 {
		t.Fatalf("expected joined tables to be collected, got %#v", updateStmt.DML.Tables)
	}

	deleteStmt := statements[1]
	if deleteStmt.DML == nil || !deleteStmt.DML.HasSubquery || deleteStmt.DML.PredicateShape != spec.PredicateShapeSubquery {
		t.Fatalf("expected subquery delete facts, got %#v", deleteStmt.DML)
	}
}

func TestExtractorDMLProjectsReturningClause(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name         string
		SQL          string
		Operation    spec.DMLOperation
		HasReturning bool
	}{
		{Name: "INSERT RETURNING sets fact", SQL: "insert into app.users (id, name) values (1, 'a') returning id", Operation: spec.DMLOperationInsert, HasReturning: true},
		{Name: "UPDATE RETURNING sets fact", SQL: "update app.users set name = 'b' where id = 1 returning name", Operation: spec.DMLOperationUpdate, HasReturning: true},
		{Name: "single-table DELETE RETURNING sets fact", SQL: "delete from app.users where id = 1 returning id", Operation: spec.DMLOperationDelete, HasReturning: true},
		{Name: "INSERT without RETURNING omits fact", SQL: "insert into app.users (id, name) values (1, 'a')", Operation: spec.DMLOperationInsert, HasReturning: false},
		{Name: "UPDATE without RETURNING omits fact", SQL: "update app.users set name = 'b' where id = 1", Operation: spec.DMLOperationUpdate, HasReturning: false},
		{Name: "DELETE without RETURNING omits fact", SQL: "delete from app.users where id = 1", Operation: spec.DMLOperationDelete, HasReturning: false},
		{Name: "INSERT column named returning is not a clause", SQL: "insert into app.users (id, returning) values (1, 2)", Operation: spec.DMLOperationInsert, HasReturning: false},
		{Name: "UPDATE column named returning is not a clause", SQL: "update app.users set returning = 5 where id = 1", Operation: spec.DMLOperationUpdate, HasReturning: false},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			stmt := extractSingleStatement(t, tc.SQL)
			if stmt.DML == nil || stmt.DML.Operation != tc.Operation {
				t.Fatalf("expected %s dml statement, got %#v", tc.Operation, stmt.DML)
			}
			if stmt.DML.HasReturning != tc.HasReturning {
				t.Fatalf("expected HasReturning=%v for %q, got %#v", tc.HasReturning, tc.SQL, stmt.DML)
			}
		})
	}
}

func TestExtractorDMLReturningAliasDoesNotProjectClause(t *testing.T) {
	t.Parallel()
	// RETURNING is still a valid table alias outside the clause position. The
	// structural fact must come from a real RETURNING clause, not the alias
	// token, so a table aliased as "returning" must not set HasReturning.
	stmt := extractSingleStatement(t, "update app.users returning set returning.name = 'b' where returning.id = 1")
	if stmt.DML == nil || stmt.DML.Operation != spec.DMLOperationUpdate {
		t.Fatalf("expected update dml statement, got %#v", stmt.DML)
	}
	if stmt.DML.HasReturning {
		t.Fatalf("expected table alias 'returning' to not project a RETURNING clause, got %#v", stmt.DML)
	}
}

func TestExtractorCapturesCreateViewDropAndTruncateFacts(t *testing.T) {
	t.Parallel()
	statements := extractStatements(t, `
		create view app.active_users as select id from app.users;
		drop table if exists app.users, app.orgs;
		drop view if exists app.active_users;
		truncate table app.audit_log;
	`)

	createView := statements[0]
	if createView.DDL == nil || createView.DDL.Operation != spec.DDLOperationCreateView || !createView.DDL.HasSelect {
		t.Fatalf("expected create view facts, got %#v", createView.DDL)
	}

	dropTable := statements[1]
	if dropTable.DDL == nil || dropTable.DDL.Operation != spec.DDLOperationDropTable {
		t.Fatalf("expected drop table facts, got %#v", dropTable.DDL)
	}
	if dropTable.DDL.Options["if_exists"] != "true" || dropTable.DDL.Options["multiple_targets"] != "2" {
		t.Fatalf("expected drop table options, got %#v", dropTable.DDL.Options)
	}

	dropView := statements[2]
	if dropView.DDL == nil || dropView.DDL.Operation != spec.DDLOperationDropView || dropView.DDL.Options["if_exists"] != "true" {
		t.Fatalf("expected drop view facts, got %#v", dropView.DDL)
	}

	truncate := statements[3]
	if truncate.DDL == nil || truncate.DDL.Operation != spec.DDLOperationTruncateTable || truncate.DDL.Table == nil || truncate.DDL.Table.Name != "audit_log" {
		t.Fatalf("expected truncate table facts, got %#v", truncate.DDL)
	}
}

func TestExtractorHelperMappingsCoverSwitchVariants(t *testing.T) {
	t.Parallel()
	rowFormats := map[uint64]string{
		ast.RowFormatDefault:      "DEFAULT",
		ast.RowFormatCompact:      "COMPACT",
		ast.RowFormatDynamic:      "DYNAMIC",
		ast.RowFormatCompressed:   "COMPRESSED",
		ast.TokuDBRowFormatSnappy: "TOKUDB_SNAPPY",
		9999:                      "",
	}
	for input, want := range rowFormats {
		if got := rowFormatName(input); got != want {
			t.Fatalf("rowFormatName(%d) = %q, want %q", input, got, want)
		}
	}

	constraintTypes := map[ast.ConstraintType]string{
		ast.ConstraintPrimaryKey: "primary",
		ast.ConstraintKey:        "key",
		ast.ConstraintIndex:      "index",
		ast.ConstraintUniq:       "unique",
		ast.ConstraintForeignKey: "foreign_key",
		ast.ConstraintCheck:      "check",
	}
	for input, want := range constraintTypes {
		if got := constraintTypeName(input); got != want {
			t.Fatalf("constraintTypeName(%v) = %q, want %q", input, got, want)
		}
	}

	alterActions := map[ast.AlterTableType]string{
		ast.AlterTableDropColumn:     "drop_column",
		ast.AlterTableChangeColumn:   "change_column",
		ast.AlterTableRenameColumn:   "rename_column",
		ast.AlterTableRenameTable:    "rename_table",
		ast.AlterTableDropPrimaryKey: "drop_primary_key",
		ast.AlterTableDropIndex:      "drop_index",
		ast.AlterTableAddConstraint:  "add_constraint",
		ast.AlterTableRenameIndex:    "rename_index",
		ast.AlterTableOption:         "table_option",
	}
	for input, want := range alterActions {
		if got := alterActionName(input); got != want {
			t.Fatalf("alterActionName(%v) = %q, want %q", input, got, want)
		}
	}

	if constraintProducesIndex(ast.ConstraintForeignKey) {
		t.Fatal("expected foreign keys to not be treated as indexes")
	}
	if !constraintProducesIndex(ast.ConstraintPrimaryKey) {
		t.Fatal("expected primary key to be treated as an index-producing constraint")
	}
	if indexKindForConstraint(ast.ConstraintIndex) != spec.IndexKindSecondary {
		t.Fatalf("expected secondary index kind")
	}
	if indexKindForConstraint(ast.ConstraintFulltext) != spec.IndexKindFulltext {
		t.Fatalf("expected fulltext index kind")
	}
	if indexKindForConstraint(ast.ConstraintPrimaryKey) != spec.IndexKindPrimary {
		t.Fatalf("expected primary index kind")
	}

	if classify(nil) != spec.KindUnknown {
		t.Fatalf("expected nil statement classification to be unknown")
	}
}

func TestExtractorHelperPredicatesAndAlterVariants(t *testing.T) {
	t.Parallel()
	updateMissingWhere := parsedNode(t, `update users set name = 'x'`).(*ast.UpdateStmt)
	shape, lookup, keyName, keyKind := extractMutationPredicateShape(updateMissingWhere.Where, tableRefsJoin(updateMissingWhere.TableRefs), true)
	if shape != spec.PredicateShapeMissingWhere || len(lookup) != 0 || keyName != "" || keyKind != spec.IndexKindUnknown {
		t.Fatalf("expected missing where shape, got %q %#v %q %q", shape, lookup, keyName, keyKind)
	}

	updateUnknown := parsedNode(t, `update users set name = 'x' where email = 'a@example.com'`).(*ast.UpdateStmt)
	shape, _, _, _ = extractMutationPredicateShape(updateUnknown.Where, tableRefsJoin(updateUnknown.TableRefs), true)
	if shape != spec.PredicateShapeUnknown {
		t.Fatalf("expected unknown predicate shape, got %q", shape)
	}

	updateJoin := parsedNode(t, `update users u join orgs o on o.id = u.org_id set u.name = 'x'`).(*ast.UpdateStmt)
	if !joinExists(tableRefsJoin(updateJoin.TableRefs)) || !joinHasOn(tableRefsJoin(updateJoin.TableRefs)) {
		t.Fatalf("expected join helpers to report join/on facts")
	}

	deleteWithParam := parsedNode(t, `delete from users where id = ?`).(*ast.DeleteStmt)
	shape, lookup, keyName, keyKind = extractMutationPredicateShape(deleteWithParam.Where, tableRefsJoin(deleteWithParam.TableRefs), true)
	if shape != spec.PredicateShapeUniqueEquality || len(lookup) != 1 || keyName != "PRIMARY" || keyKind != spec.IndexKindPrimary {
		t.Fatalf("expected parameterized unique equality shape, got %q %#v %q %q", shape, lookup, keyName, keyKind)
	}

	renameColumn := extractSingleStatement(t, `alter table users rename column old_name to new_name`)
	renameAlter := findAlter(t, renameColumn.DDL.Alter, "rename_column", "old_name")
	if renameAlter.Column == nil || renameAlter.Column.OldName != "old_name" || renameAlter.Column.Definition == nil || renameAlter.Column.Definition.Name != "new_name" {
		t.Fatalf("expected rename column payload, got %#v", renameAlter)
	}

	dropColumn := extractSingleStatement(t, `alter table users drop column old_name`)
	dropAlter := findAlter(t, dropColumn.DDL.Alter, "drop_column", "old_name")
	if dropAlter.Column == nil || dropAlter.Column.OldName != "old_name" {
		t.Fatalf("expected drop column payload, got %#v", dropAlter)
	}

	if got := extractAlterName(&ast.AlterTableSpec{FromKey: modelCIStr("from_idx")}); got != "from_idx" {
		t.Fatalf("expected from key name, got %q", got)
	}
	if got := extractAlterName(&ast.AlterTableSpec{ToKey: modelCIStr("to_idx")}); got != "to_idx" {
		t.Fatalf("expected to key name, got %q", got)
	}
	if got := extractAlterName(&ast.AlterTableSpec{IndexName: modelCIStr("idx_name")}); got != "idx_name" {
		t.Fatalf("expected index name, got %q", got)
	}
	if got := extractAlterName(&ast.AlterTableSpec{Name: "custom"}); got != "custom" {
		t.Fatalf("expected custom name, got %q", got)
	}
}

func extractStatements(t *testing.T, sql string) []spec.Statement {
	t.Helper()

	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse sql: %v", err)
	}

	wrapped := WrapStatements(result.Statements, result.Warnings)
	statements := make([]spec.Statement, 0, len(wrapped))
	for _, item := range wrapped {
		statement, err := item.Extractor.Extract(spec.DialectTiDB, item.RawSQL)
		if err != nil {
			t.Fatalf("extract statement: %v", err)
		}
		statements = append(statements, statement)
	}
	return statements
}

func parsedNode(t *testing.T, sql string) ast.StmtNode {
	t.Helper()

	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 parsed statement, got %d", len(result.Statements))
	}
	return result.Statements[0]
}

func extractSingleStatement(t *testing.T, sql string) spec.Statement {
	t.Helper()

	statements := extractStatements(t, sql)
	if len(statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(statements))
	}
	return statements[0]
}

func modelCIStr(value string) ast.CIStr {
	return ast.NewCIStr(value)
}

func findAlter(t *testing.T, alters []spec.Alter, action string, name string) spec.Alter {
	t.Helper()

	for _, alter := range alters {
		if alter.Action == action && alter.Name == name {
			return alter
		}
	}
	t.Fatalf("expected alter action %q with name %q in %#v", action, name, alters)
	return spec.Alter{}
}

func TestExtractorDatabaseLifecycleCreateDatabase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name        string
		SQL         string
		ObjectName  string
		IfNotExists bool
		Charset     string
		Collate     string
	}{
		{Name: "CREATE DATABASE", SQL: "CREATE DATABASE app", ObjectName: "app"},
		{Name: "CREATE DATABASE IF NOT EXISTS", SQL: "CREATE DATABASE IF NOT EXISTS app", ObjectName: "app", IfNotExists: true},
		{Name: "CREATE SCHEMA synonym", SQL: "CREATE SCHEMA app", ObjectName: "app"},
		{Name: "CREATE SCHEMA IF NOT EXISTS synonym", SQL: "CREATE SCHEMA IF NOT EXISTS app", ObjectName: "app", IfNotExists: true},
		{Name: "CREATE DATABASE charset", SQL: "CREATE DATABASE app CHARACTER SET utf8mb4", ObjectName: "app", Charset: "utf8mb4"},
		{Name: "CREATE DATABASE collate", SQL: "CREATE DATABASE app COLLATE utf8mb4_bin", ObjectName: "app", Collate: "utf8mb4_bin"},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			stmt := extractSingleStatement(t, tc.SQL)
			if stmt.Kind != spec.KindDDL {
				t.Fatalf("expected kind DDL, got %q", stmt.Kind)
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL metadata")
			}
			if stmt.DDL.Operation != spec.DDLOperationCreateSchema {
				t.Fatalf("expected operation create_schema, got %q", stmt.DDL.Operation)
			}
			if stmt.DDL.ObjectType != "database" {
				t.Fatalf("expected object_type database, got %q", stmt.DDL.ObjectType)
			}
			if stmt.DDL.ObjectName != tc.ObjectName {
				t.Fatalf("expected object_name %q, got %q", tc.ObjectName, stmt.DDL.ObjectName)
			}
			if tc.IfNotExists && stmt.DDL.Options["if_not_exists"] != "true" {
				t.Fatalf("expected if_not_exists=true, got %q", stmt.DDL.Options["if_not_exists"])
			}
			if !tc.IfNotExists && stmt.DDL.Options["if_not_exists"] == "true" {
				t.Fatal("did not expect if_not_exists=true")
			}
			if tc.Charset != "" && stmt.DDL.Options["charset"] != tc.Charset {
				t.Fatalf("expected charset=%q, got %q", tc.Charset, stmt.DDL.Options["charset"])
			}
			if tc.Collate != "" && stmt.DDL.Options["collate"] != tc.Collate {
				t.Fatalf("expected collate=%q, got %q", tc.Collate, stmt.DDL.Options["collate"])
			}
		})
	}
}

func TestExtractorDatabaseLifecycleDropDatabase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name       string
		SQL        string
		ObjectName string
		IfExists   bool
	}{
		{Name: "DROP DATABASE", SQL: "DROP DATABASE app", ObjectName: "app"},
		{Name: "DROP DATABASE IF EXISTS", SQL: "DROP DATABASE IF EXISTS app", ObjectName: "app", IfExists: true},
		{Name: "DROP SCHEMA synonym", SQL: "DROP SCHEMA app", ObjectName: "app"},
		{Name: "DROP SCHEMA IF EXISTS synonym", SQL: "DROP SCHEMA IF EXISTS app", ObjectName: "app", IfExists: true},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			stmt := extractSingleStatement(t, tc.SQL)
			if stmt.Kind != spec.KindDDL {
				t.Fatalf("expected kind DDL, got %q", stmt.Kind)
			}
			if stmt.DDL == nil {
				t.Fatal("expected DDL metadata")
			}
			if stmt.DDL.Operation != spec.DDLOperationDropSchema {
				t.Fatalf("expected operation drop_schema, got %q", stmt.DDL.Operation)
			}
			if stmt.DDL.ObjectType != "database" {
				t.Fatalf("expected object_type database, got %q", stmt.DDL.ObjectType)
			}
			if stmt.DDL.ObjectName != tc.ObjectName {
				t.Fatalf("expected object_name %q, got %q", tc.ObjectName, stmt.DDL.ObjectName)
			}
			if tc.IfExists && stmt.DDL.Options["if_exists"] != "true" {
				t.Fatalf("expected if_exists=true, got %q", stmt.DDL.Options["if_exists"])
			}
			if !tc.IfExists && stmt.DDL.Options["if_exists"] == "true" {
				t.Fatal("did not expect if_exists=true")
			}
		})
	}
}
