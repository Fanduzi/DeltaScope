// Package spec defines normalized statement specifications for rule evaluation.
// input: statement kind and dialect domain scenarios
// output: coverage for typed statement metadata
// pos: domain specification test coverage
// note: if this file changes, update this header and module README.md.
package spec

import "testing"

func TestStatementKindAndDialectTypes(t *testing.T) {
	stmt := Statement{
		Kind:    KindDDL,
		Dialect: DialectMySQL,
	}

	if stmt.Kind != KindDDL {
		t.Fatalf("expected kind %q, got %q", KindDDL, stmt.Kind)
	}
	if stmt.Dialect != DialectMySQL {
		t.Fatalf("expected dialect %q, got %q", DialectMySQL, stmt.Dialect)
	}
}

func TestStatementMetadataSupportsInstanceAndTargetTableFacts(t *testing.T) {
	stmt := Statement{
		Metadata: &Metadata{
			Instance: &InstanceFacts{
				Version:                   "8.0.36",
				DefaultCharset:            "utf8mb4",
				InnoDBLargePrefixEnabled:  true,
				InnoDBDefaultRowFormat:    "dynamic",
				InnoDBAdaptiveHashEnabled: true,
			},
			TargetTable: &TableSnapshot{
				Schema: "app",
				Exists: true,
				Table:  &Table{Name: "users"},
				Columns: []Column{
					{Name: "id", Type: "bigint"},
					{Name: "email", Type: "varchar"},
				},
				PrimaryKey: &Index{Name: "PRIMARY", Kind: IndexKindPrimary, Columns: []string{"id"}},
				Indexes: []Index{
					{Name: "idx_email", Kind: IndexKindSecondary, Columns: []string{"email"}},
				},
			},
		},
	}

	if stmt.Metadata == nil || stmt.Metadata.Instance == nil {
		t.Fatalf("expected statement metadata instance facts")
	}
	if stmt.Metadata.Instance.DefaultCharset != "utf8mb4" {
		t.Fatalf("expected instance charset utf8mb4, got %q", stmt.Metadata.Instance.DefaultCharset)
	}
	if stmt.Metadata.TargetTable == nil || !stmt.Metadata.TargetTable.Exists {
		t.Fatalf("expected target table snapshot to exist")
	}
	if !stmt.Metadata.TargetTable.HasColumn("EMAIL") {
		t.Fatalf("expected case-insensitive column lookup to succeed")
	}
	if !stmt.Metadata.TargetTable.HasIndex("IDX_EMAIL") {
		t.Fatalf("expected case-insensitive index lookup to succeed")
	}
	if !stmt.Metadata.TargetTable.HasPrimaryKey() {
		t.Fatalf("expected primary key presence to be reported")
	}
}

func TestDDLTracksExplicitNamesForNamingGovernanceTargets(t *testing.T) {
	stmt := Statement{
		Kind:    KindDDL,
		Dialect: DialectMySQL,
		DDL: &DDL{
			Operation: DDLOperationCreateTable,
			Table:     &Table{Schema: "app", Name: "tbl_orders"},
			Columns: []Column{
				{Name: "order_id", Type: "bigint"},
				{Name: "user_id", Type: "bigint"},
			},
			PrimaryKey: &Index{
				Name:    "primary",
				Kind:    IndexKindPrimary,
				Columns: []string{"order_id"},
			},
			Indexes: []Index{
				{Name: "uniq_orders_user", Kind: IndexKindUnique, Columns: []string{"user_id"}},
				{Name: "idx_orders_user", Kind: IndexKindSecondary, Columns: []string{"user_id"}},
				{Name: "full_orders_note", Kind: IndexKindFulltext, Columns: []string{"note_body"}},
			},
			Constraints: []Constraint{
				{Type: "foreign_key", Name: "fk_orders_user", Columns: []string{"user_id"}},
				{Type: "check", Name: "chk_orders_amount", Columns: []string{"amount"}},
			},
		},
	}

	if stmt.DDL == nil || stmt.DDL.Table == nil {
		t.Fatalf("expected ddl table metadata")
	}
	if stmt.DDL.Table.Name != "tbl_orders" {
		t.Fatalf("expected table name tbl_orders, got %q", stmt.DDL.Table.Name)
	}
	if len(stmt.DDL.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(stmt.DDL.Columns))
	}
	if stmt.DDL.Columns[0].Name != "order_id" || stmt.DDL.Columns[1].Name != "user_id" {
		t.Fatalf("expected explicit column names, got %+v", stmt.DDL.Columns)
	}
	if stmt.DDL.PrimaryKey == nil || stmt.DDL.PrimaryKey.Name != "primary" || stmt.DDL.PrimaryKey.Kind != IndexKindPrimary {
		t.Fatalf("expected named primary key metadata, got %+v", stmt.DDL.PrimaryKey)
	}
	if len(stmt.DDL.Indexes) != 3 {
		t.Fatalf("expected 3 named indexes, got %d", len(stmt.DDL.Indexes))
	}
	if stmt.DDL.Indexes[0].Name != "uniq_orders_user" || stmt.DDL.Indexes[0].Kind != IndexKindUnique {
		t.Fatalf("expected named unique index, got %+v", stmt.DDL.Indexes[0])
	}
	if stmt.DDL.Indexes[1].Name != "idx_orders_user" || stmt.DDL.Indexes[1].Kind != IndexKindSecondary {
		t.Fatalf("expected named secondary index, got %+v", stmt.DDL.Indexes[1])
	}
	if stmt.DDL.Indexes[2].Name != "full_orders_note" || stmt.DDL.Indexes[2].Kind != IndexKindFulltext {
		t.Fatalf("expected named fulltext index, got %+v", stmt.DDL.Indexes[2])
	}
	if len(stmt.DDL.Constraints) != 2 {
		t.Fatalf("expected 2 named constraints, got %d", len(stmt.DDL.Constraints))
	}
	if stmt.DDL.Constraints[0].Type != "foreign_key" || stmt.DDL.Constraints[0].Name != "fk_orders_user" {
		t.Fatalf("expected named foreign key metadata, got %+v", stmt.DDL.Constraints[0])
	}
	if stmt.DDL.Constraints[1].Type != "check" || stmt.DDL.Constraints[1].Name != "chk_orders_amount" {
		t.Fatalf("expected named check constraint metadata, got %+v", stmt.DDL.Constraints[1])
	}
}
