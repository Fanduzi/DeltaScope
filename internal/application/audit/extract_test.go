// Package audit verifies AST-to-StatementSpec extraction behavior.
// input: parsed SQL statements produced by the application parsing flow
// output: test coverage for first-pass StatementSpec extraction
// pos: application audit extraction test coverage
// note: if this file changes, update this header and module README.md.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractMapsCreateTable(t *testing.T) {
	parsed, err := Parse("create table users (id bigint unsigned not null auto_increment comment 'pk', name varchar(32) default 'guest' comment 'name', body text comment 'body', created_at datetime not null default current_timestamp comment 'created', updated_at datetime not null default current_timestamp on update current_timestamp comment 'updated', primary key (id), key idx_name (name), unique key uniq_name (name), fulltext key full_body (body)) comment='user table';", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(statements))
	}

	stmt := statements[0]
	if stmt.Kind != spec.KindDDL {
		t.Fatalf("expected kind %q, got %q", spec.KindDDL, stmt.Kind)
	}
	if stmt.RawSQL == "" || stmt.NormalizedSQL == "" {
		t.Fatalf("expected raw and normalized sql to be populated")
	}
	if stmt.DDL == nil || stmt.DDL.Table == nil {
		t.Fatalf("expected ddl table metadata to be populated")
	}
	if stmt.DDL.Table.Name != "users" {
		t.Fatalf("expected table name users, got %q", stmt.DDL.Table.Name)
	}
	if len(stmt.DDL.Columns) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(stmt.DDL.Columns))
	}
	idCol := stmt.DDL.Columns[0]
	if !idCol.Unsigned || !idCol.AutoIncrement {
		t.Fatalf("expected id column to be unsigned auto_increment, got %+v", idCol)
	}

	nameCol := stmt.DDL.Columns[1]
	if nameCol.Type != "varchar(32)" {
		t.Fatalf("expected normalized varchar type, got %q", nameCol.Type)
	}
	if nameCol.Length != 32 {
		t.Fatalf("expected varchar length 32, got %d", nameCol.Length)
	}
	if !nameCol.HasDefault || nameCol.DefaultValue != "'guest'" {
		t.Fatalf("expected default value 'guest', got has_default=%t default=%q", nameCol.HasDefault, nameCol.DefaultValue)
	}

	createdAt := stmt.DDL.Columns[3]
	if !createdAt.NotNull {
		t.Fatalf("expected created_at to be not null")
	}
	if !createdAt.DefaultIsCurrentTimestamp {
		t.Fatalf("expected created_at to use current_timestamp default")
	}
	if createdAt.OnUpdateCurrentTimestamp {
		t.Fatalf("expected created_at not to carry on update current_timestamp")
	}

	updatedAt := stmt.DDL.Columns[4]
	if !updatedAt.DefaultIsCurrentTimestamp || !updatedAt.OnUpdateCurrentTimestamp {
		t.Fatalf("expected updated_at audit timestamp metadata, got %+v", updatedAt)
	}
	if stmt.DDL.PrimaryKey == nil {
		t.Fatalf("expected primary key metadata to be populated")
	}
	if stmt.DDL.PrimaryKey.Name != "primary" {
		t.Fatalf("expected primary key name primary, got %q", stmt.DDL.PrimaryKey.Name)
	}
	if stmt.DDL.PrimaryKey.Kind != spec.IndexKindPrimary {
		t.Fatalf("expected primary key kind %q, got %q", spec.IndexKindPrimary, stmt.DDL.PrimaryKey.Kind)
	}
	if len(stmt.DDL.Indexes) != 3 {
		t.Fatalf("expected 3 secondary indexes, got %d", len(stmt.DDL.Indexes))
	}
	if stmt.DDL.Indexes[0].Kind != spec.IndexKindSecondary {
		t.Fatalf("expected first index kind %q, got %q", spec.IndexKindSecondary, stmt.DDL.Indexes[0].Kind)
	}
	if stmt.DDL.Indexes[1].Kind != spec.IndexKindUnique {
		t.Fatalf("expected second index kind %q, got %q", spec.IndexKindUnique, stmt.DDL.Indexes[1].Kind)
	}
	if stmt.DDL.Indexes[2].Kind != spec.IndexKindFulltext {
		t.Fatalf("expected third index kind %q, got %q", spec.IndexKindFulltext, stmt.DDL.Indexes[2].Kind)
	}
}

func TestExtractPreservesBacktickedKeywordsAndUnnamedIndexes(t *testing.T) {
	parsed, err := Parse("create table `select` (`from` bigint unsigned not null auto_increment comment 'pk', `group` varchar(32) comment 'group', primary key (`from`), key (`group`), key `order` (`group`)) comment='keyword table';", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if stmt.DDL == nil || stmt.DDL.Table == nil {
		t.Fatalf("expected ddl table metadata")
	}
	if stmt.DDL.Table.Name != "select" {
		t.Fatalf("expected backticked keyword table name to normalize to select, got %q", stmt.DDL.Table.Name)
	}
	if len(stmt.DDL.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(stmt.DDL.Columns))
	}
	if stmt.DDL.Columns[0].Name != "from" || stmt.DDL.Columns[1].Name != "group" {
		t.Fatalf("expected keyword column names to be preserved, got %+v", stmt.DDL.Columns)
	}
	if len(stmt.DDL.Indexes) != 2 {
		t.Fatalf("expected 2 non-primary indexes, got %d", len(stmt.DDL.Indexes))
	}
	if stmt.DDL.Indexes[0].Name != "" {
		t.Fatalf("expected unnamed secondary index to stay unnamed, got %q", stmt.DDL.Indexes[0].Name)
	}
	if stmt.DDL.Indexes[1].Name != "order" {
		t.Fatalf("expected named keyword index order, got %q", stmt.DDL.Indexes[1].Name)
	}
}

func TestExtractMapsColumnCharsetAndCollationFacts(t *testing.T) {
	parsed, err := Parse("create table users (name varchar(32) character set utf8mb4 collate utf8mb4_bin comment 'name', alias char(16) character set utf8 comment 'alias', payload json comment 'payload') comment='users';", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if len(stmt.DDL.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(stmt.DDL.Columns))
	}
	if stmt.DDL.Columns[0].Charset != "utf8mb4" || stmt.DDL.Columns[0].Collation != "utf8mb4_bin" {
		t.Fatalf("expected name column charset/collation to be extracted, got %+v", stmt.DDL.Columns[0])
	}
	if stmt.DDL.Columns[1].Charset != "utf8" || stmt.DDL.Columns[1].Collation != "" {
		t.Fatalf("expected alias column explicit charset only, got %+v", stmt.DDL.Columns[1])
	}
	if stmt.DDL.Columns[2].Charset != "" || stmt.DDL.Columns[2].Collation != "" {
		t.Fatalf("expected json column not to carry create-table charset metadata, got %+v", stmt.DDL.Columns[2])
	}
}

func TestExtractMapsAlterTable(t *testing.T) {
	t.Run("maps representative alter shapes", func(t *testing.T) {
		parsed, err := Parse("alter table users add column age int not null default 0 comment 'age', drop column old_age, modify column age bigint null default 1 comment 'age2', change column old_name new_name bigint unsigned not null auto_increment comment 'name', rename column old_email to email, add unique index uniq_email (email), drop index idx_old, rename index idx_old to idx_new, engine=InnoDB, comment='user table';", spec.DialectMySQL)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		statements, err := Extract(parsed)
		if err != nil {
			t.Fatalf("extract: %v", err)
		}

		stmt := statements[0]
		if stmt.DDL == nil || stmt.DDL.Table == nil {
			t.Fatalf("expected ddl metadata to be populated")
		}
		if stmt.DDL.Table.Name != "users" {
			t.Fatalf("expected table name users, got %q", stmt.DDL.Table.Name)
		}
		if len(stmt.DDL.Alter) != 10 {
			t.Fatalf("expected 10 alter actions, got %d", len(stmt.DDL.Alter))
		}

		addColumn := stmt.DDL.Alter[0]
		if addColumn.Action != "add_columns" {
			t.Fatalf("expected first alter action add_columns, got %q", addColumn.Action)
		}
		if addColumn.Name != "age" {
			t.Fatalf("expected canonical add column name age, got %q", addColumn.Name)
		}
		if addColumn.Column == nil {
			t.Fatalf("expected add column payload to be populated")
		}
		if addColumn.Column.OldName != "" {
			t.Fatalf("expected add column old name to be empty, got %q", addColumn.Column.OldName)
		}
		if addColumn.Column.Definition == nil {
			t.Fatalf("expected add column definition to be populated")
		}
		if addColumn.Column.Definition.Name != "age" {
			t.Fatalf("expected add column definition name age, got %q", addColumn.Column.Definition.Name)
		}
		if addColumn.Column.Definition.Type != "int(11)" {
			t.Fatalf("expected add column type int(11), got %q", addColumn.Column.Definition.Type)
		}
		if !addColumn.Column.Definition.NotNull || !addColumn.Column.Definition.HasDefault || addColumn.Column.Definition.DefaultValue != "0" {
			t.Fatalf("expected add column defaults/not-null metadata, got %+v", *addColumn.Column.Definition)
		}
		if addColumn.Column.Definition.Comment != "'age'" {
			t.Fatalf("expected add column comment 'age', got %q", addColumn.Column.Definition.Comment)
		}

		dropColumn := stmt.DDL.Alter[1]
		if dropColumn.Action != "drop_column" {
			t.Fatalf("expected second alter action drop_column, got %q", dropColumn.Action)
		}
		if dropColumn.Name != "old_age" {
			t.Fatalf("expected canonical drop column name old_age, got %q", dropColumn.Name)
		}
		if dropColumn.Column == nil || dropColumn.Column.OldName != "old_age" {
			t.Fatalf("expected drop column old_age payload, got %+v", dropColumn.Column)
		}
		if dropColumn.Column.Definition != nil {
			t.Fatalf("expected drop column definition to be empty, got %+v", dropColumn.Column.Definition)
		}

		modifyColumn := stmt.DDL.Alter[2]
		if modifyColumn.Action != "modify_column" {
			t.Fatalf("expected third alter action modify_column, got %q", modifyColumn.Action)
		}
		if modifyColumn.Name != "age" {
			t.Fatalf("expected canonical modify column name age, got %q", modifyColumn.Name)
		}
		if modifyColumn.Column == nil || modifyColumn.Column.Definition == nil {
			t.Fatalf("expected modify column payload to be populated")
		}
		if modifyColumn.Column.Change == nil {
			t.Fatalf("expected modify column change facts to be populated")
		}
		if !modifyColumn.Column.Change.TouchesNullability || !modifyColumn.Column.Change.TouchesDefault {
			t.Fatalf("expected modify column to mark nullability/default touches, got %+v", *modifyColumn.Column.Change)
		}
		if modifyColumn.Column.Change.TouchesAutoIncrement {
			t.Fatalf("expected modify column not to mark auto_increment touch, got %+v", *modifyColumn.Column.Change)
		}

		changeColumn := stmt.DDL.Alter[3]
		if changeColumn.Action != "change_column" {
			t.Fatalf("expected fourth alter action change_column, got %q", changeColumn.Action)
		}
		if changeColumn.Name != "old_name" {
			t.Fatalf("expected canonical change column name old_name, got %q", changeColumn.Name)
		}
		if changeColumn.Column == nil {
			t.Fatalf("expected change column payload to be populated")
		}
		if changeColumn.Column.OldName != "old_name" || changeColumn.Column.Definition == nil || changeColumn.Column.Definition.Name != "new_name" {
			t.Fatalf("expected change column rename old_name->new_name, got %+v", *changeColumn.Column)
		}
		if changeColumn.Column.Definition.Type != "bigint(20) unsigned" || !changeColumn.Column.Definition.Unsigned || !changeColumn.Column.Definition.AutoIncrement || !changeColumn.Column.Definition.NotNull {
			t.Fatalf("expected change column semantic payload, got %+v", *changeColumn.Column.Definition)
		}
		if changeColumn.Column.Change == nil {
			t.Fatalf("expected change column change facts to be populated")
		}
		if !changeColumn.Column.Change.TouchesNullability || !changeColumn.Column.Change.TouchesAutoIncrement {
			t.Fatalf("expected change column to mark nullability/auto_increment touches, got %+v", *changeColumn.Column.Change)
		}
		if changeColumn.Column.Change.TouchesDefault {
			t.Fatalf("expected change column without explicit default to avoid default touch, got %+v", *changeColumn.Column.Change)
		}

		renameColumn := stmt.DDL.Alter[4]
		if renameColumn.Action != "rename_column" {
			t.Fatalf("expected fifth alter action rename_column, got %q", renameColumn.Action)
		}
		if renameColumn.Name != "old_email" {
			t.Fatalf("expected canonical rename column name old_email, got %q", renameColumn.Name)
		}
		if renameColumn.Column == nil || renameColumn.Column.OldName != "old_email" || renameColumn.Column.Definition == nil || renameColumn.Column.Definition.Name != "email" {
			t.Fatalf("expected rename column payload old_email->email, got %+v", renameColumn.Column)
		}
		if renameColumn.Column.Change != nil {
			t.Fatalf("expected pure rename column to avoid separate change facts, got %+v", renameColumn.Column.Change)
		}

		addIndex := stmt.DDL.Alter[5]
		if addIndex.Action != "add_constraint" {
			t.Fatalf("expected sixth alter action add_constraint, got %q", addIndex.Action)
		}
		if addIndex.Name != "uniq_email" {
			t.Fatalf("expected canonical add index name uniq_email, got %q", addIndex.Name)
		}
		if addIndex.Index == nil {
			t.Fatalf("expected add index payload to be populated")
		}
		if addIndex.Index.OldName != "" {
			t.Fatalf("expected add index old name to be empty, got %q", addIndex.Index.OldName)
		}
		if addIndex.Index.Definition == nil || addIndex.Index.Definition.Kind != spec.IndexKindUnique || addIndex.Index.Definition.Name != "uniq_email" {
			t.Fatalf("expected unique index uniq_email, got %+v", addIndex.Index.Definition)
		}
		if len(addIndex.Index.Definition.Columns) != 1 || addIndex.Index.Definition.Columns[0] != "email" {
			t.Fatalf("expected add index columns [email], got %+v", addIndex.Index.Definition.Columns)
		}

		dropIndex := stmt.DDL.Alter[6]
		if dropIndex.Action != "drop_index" {
			t.Fatalf("expected seventh alter action drop_index, got %q", dropIndex.Action)
		}
		if dropIndex.Name != "idx_old" {
			t.Fatalf("expected canonical drop index name idx_old, got %q", dropIndex.Name)
		}
		if dropIndex.Index == nil || dropIndex.Index.OldName != "idx_old" {
			t.Fatalf("expected drop index idx_old payload, got %+v", dropIndex.Index)
		}
		if dropIndex.Index.Definition != nil {
			t.Fatalf("expected drop index definition to be empty, got %+v", dropIndex.Index.Definition)
		}

		renameIndex := stmt.DDL.Alter[7]
		if renameIndex.Action != "rename_index" {
			t.Fatalf("expected eighth alter action rename_index, got %q", renameIndex.Action)
		}
		if renameIndex.Name != "idx_old" {
			t.Fatalf("expected canonical rename index name idx_old, got %q", renameIndex.Name)
		}
		if renameIndex.Index == nil || renameIndex.Index.OldName != "idx_old" || renameIndex.Index.Definition == nil || renameIndex.Index.Definition.Name != "idx_new" {
			t.Fatalf("expected rename index idx_old->idx_new payload, got %+v", renameIndex.Index)
		}

		engineOption := stmt.DDL.Alter[8]
		if engineOption.Action != "table_option" {
			t.Fatalf("expected ninth alter action table_option, got %q", engineOption.Action)
		}
		if engineOption.Options["engine"] != "InnoDB" {
			t.Fatalf("expected engine option InnoDB, got %+v", engineOption.Options)
		}

		commentOption := stmt.DDL.Alter[9]
		if commentOption.Action != "table_option" {
			t.Fatalf("expected tenth alter action table_option, got %q", commentOption.Action)
		}
		if commentOption.Options["comment"] != "user table" {
			t.Fatalf("expected comment option user table, got %+v", commentOption.Options)
		}
	})

	t.Run("splits multi-column add into multiple alter records", func(t *testing.T) {
		parsed, err := Parse("alter table users add (city varchar(32), score bigint unsigned);", spec.DialectMySQL)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		statements, err := Extract(parsed)
		if err != nil {
			t.Fatalf("extract: %v", err)
		}

		alter := statements[0].DDL.Alter
		if len(alter) != 2 {
			t.Fatalf("expected 2 normalized add-column records, got %d", len(alter))
		}
		if alter[0].Name != "city" || alter[1].Name != "score" {
			t.Fatalf("expected add-column names [city score], got [%s %s]", alter[0].Name, alter[1].Name)
		}
		if alter[0].Column == nil || alter[0].Column.Definition == nil || alter[0].Column.Definition.Name != "city" {
			t.Fatalf("expected city definition to survive normalization, got %+v", alter[0].Column)
		}
		if alter[1].Column == nil || alter[1].Column.Definition == nil || !alter[1].Column.Definition.Unsigned {
			t.Fatalf("expected score definition to survive normalization, got %+v", alter[1].Column)
		}
	})

	t.Run("keeps non-index add constraint out of index payloads", func(t *testing.T) {
		parsed, err := Parse("alter table users add constraint fk_users_account foreign key (account_id) references accounts(id);", spec.DialectMySQL)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		statements, err := Extract(parsed)
		if err != nil {
			t.Fatalf("extract: %v", err)
		}

		alter := statements[0].DDL.Alter
		if len(alter) != 1 {
			t.Fatalf("expected 1 alter action, got %d", len(alter))
		}
		if alter[0].Action != "add_constraint" {
			t.Fatalf("expected add_constraint action, got %q", alter[0].Action)
		}
		if alter[0].Name != "fk_users_account" {
			t.Fatalf("expected canonical constraint name fk_users_account, got %q", alter[0].Name)
		}
		if alter[0].Index != nil {
			t.Fatalf("expected non-index add constraint to avoid AlterIndex payload, got %+v", alter[0].Index)
		}
	})
}

func TestExtractMapsInsert(t *testing.T) {
	parsed, err := Parse("insert into users(id, name) values (1, 'a'), (2, 'b') on duplicate key update name = values(name);", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if stmt.Kind != spec.KindDML || stmt.DML == nil {
		t.Fatalf("expected dml statement to be populated")
	}
	if stmt.DML.Operation != spec.DMLOperationInsert {
		t.Fatalf("expected insert operation, got %q", stmt.DML.Operation)
	}
	if stmt.DML.InsertRows != 2 {
		t.Fatalf("expected 2 insert rows, got %d", stmt.DML.InsertRows)
	}
	if !stmt.DML.HasOnDuplicate {
		t.Fatalf("expected insert to report has_on_duplicate=true")
	}
}

func TestExtractMapsUpdate(t *testing.T) {
	parsed, err := Parse("update users set name = 'c' where id = 1;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if stmt.DML == nil {
		t.Fatalf("expected dml metadata to be populated")
	}
	if stmt.DML.Operation != spec.DMLOperationUpdate {
		t.Fatalf("expected update operation, got %q", stmt.DML.Operation)
	}
	if !stmt.DML.HasWhere {
		t.Fatalf("expected update to have where")
	}
	if stmt.DML.HasJoin {
		t.Fatalf("expected single-table update to report no join")
	}
}

func TestExtractMapsDelete(t *testing.T) {
	parsed, err := Parse("delete from users where id = 1 limit 1;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if stmt.DML == nil {
		t.Fatalf("expected dml metadata to be populated")
	}
	if stmt.DML.Operation != spec.DMLOperationDelete {
		t.Fatalf("expected delete operation, got %q", stmt.DML.Operation)
	}
	if !stmt.DML.HasWhere {
		t.Fatalf("expected delete to have where")
	}
	if !stmt.DML.HasLimit {
		t.Fatalf("expected delete to have limit")
	}
	if stmt.DML.HasJoin {
		t.Fatalf("expected single-table delete to report no join")
	}
}

func TestExtractDistinguishesJoinWithoutOn(t *testing.T) {
	parsed, err := Parse("update users u join accounts a set u.name = 'c' where u.id = a.user_id;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if stmt.DML == nil {
		t.Fatalf("expected dml metadata to be populated")
	}
	if !stmt.DML.HasJoin {
		t.Fatalf("expected update join to report has_join=true")
	}
	if stmt.DML.HasJoinOn {
		t.Fatalf("expected join without ON to report has_join_on=false")
	}
}

func TestExtractMapsInsertSelect(t *testing.T) {
	parsed, err := Parse("insert into users(id, name) select id, name from staging_users;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	stmt := statements[0]
	if stmt.DML == nil {
		t.Fatalf("expected dml metadata to be populated")
	}
	if !stmt.DML.IsInsertSelect {
		t.Fatalf("expected insert-select metadata to be populated")
	}
}

func TestExtractLeavesUnknownStatementsAvailableForLaterLayers(t *testing.T) {
	parsed, err := Parse("select 1;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("expected unknown-but-parseable statement to survive extraction, got %v", err)
	}

	if len(statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(statements))
	}
	if statements[0].Kind != spec.KindUnknown {
		t.Fatalf("expected unknown kind, got %q", statements[0].Kind)
	}
	if statements[0].DDL != nil || statements[0].DML != nil {
		t.Fatalf("expected unknown statement to keep empty DDL/DML substructures")
	}
}

func TestExtractMapsCreateTableLike(t *testing.T) {
	parsed, err := Parse("create table users_copy like users;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !statements[0].DDL.HasReferTable {
		t.Fatalf("expected create table like to set has_refer_table=true")
	}
}

func TestExtractMapsCreateTableAsSelect(t *testing.T) {
	parsed, err := Parse("create table users_copy as select * from users;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !statements[0].DDL.HasSelect {
		t.Fatalf("expected create table as select to set has_select=true")
	}
}

func TestExtractMapsCreateTablePartition(t *testing.T) {
	parsed, err := Parse("create table users (id bigint primary key) partition by hash(id) partitions 4;", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !statements[0].DDL.HasPartition {
		t.Fatalf("expected partitioned create table to set has_partition=true")
	}
}
