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
