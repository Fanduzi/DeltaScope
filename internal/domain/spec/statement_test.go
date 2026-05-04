// Package spec defines normalized statement specifications for rule evaluation.
// input: statement kind and dialect domain scenarios
// output: coverage for typed statement metadata
// pos: domain specification test coverage
// note: if this file changes, update this header and module README.md.
package spec

import (
	"encoding/json"
	"testing"
)

func TestKindAndDialectStringReturnUnderlyingValue(t *testing.T) {
	if got := KindDDL.String(); got != "ddl" {
		t.Fatalf("expected ddl string, got %q", got)
	}
	if got := KindDML.String(); got != "dml" {
		t.Fatalf("expected dml string, got %q", got)
	}
	if got := DialectMySQL.String(); got != "mysql" {
		t.Fatalf("expected mysql string, got %q", got)
	}
	if got := DialectTiDB.String(); got != "tidb" {
		t.Fatalf("expected tidb string, got %q", got)
	}
	if got := DialectPostgreSQL.String(); got != "postgresql" {
		t.Fatalf("expected postgresql string, got %q", got)
	}
}

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
	if DialectPostgreSQL != Dialect("postgresql") {
		t.Fatalf("expected postgresql dialect constant, got %q", DialectPostgreSQL)
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
	if stmt.Metadata.TargetTable.FindColumn("   ") != nil {
		t.Fatal("expected blank column lookup to return nil")
	}
	if stmt.Metadata.TargetTable.FindIndex("") != nil {
		t.Fatal("expected blank index lookup to return nil")
	}
}

func TestTableSnapshotHasPrimaryKeyRecognizesConstraintOnlySnapshot(t *testing.T) {
	snapshot := TableSnapshot{
		Constraints: []Constraint{{Type: "primary_key", Name: "users_pkey", Columns: []string{"id"}}},
	}

	if !snapshot.HasPrimaryKey() {
		t.Fatal("expected constraint-backed primary key to count as existing")
	}
}

func TestTableSnapshotHasPrimaryKeyRecognizesIndexOnlySnapshot(t *testing.T) {
	snapshot := TableSnapshot{
		PrimaryKey: &Index{Name: "PRIMARY", Kind: IndexKindPrimary, Columns: []string{"id"}},
	}

	if !snapshot.HasPrimaryKey() {
		t.Fatal("expected primary key index to count as existing")
	}
}

func TestTableSnapshotHasPrimaryKeyReturnsFalseWithoutPrimaryKeyFacts(t *testing.T) {
	snapshot := TableSnapshot{
		Constraints: []Constraint{{Type: "check", Name: "users_id_positive", Columns: []string{"id"}}},
		Indexes:     []Index{{Name: "idx_email", Kind: IndexKindSecondary, Columns: []string{"email"}}},
	}

	if snapshot.HasPrimaryKey() {
		t.Fatal("expected snapshot without primary key facts to report false")
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

func TestConstraintPreservesForeignKeyReferencedObjectFacts(t *testing.T) {
	constraint := Constraint{
		Type:              "foreign_key",
		Name:              "fk_orders_user",
		Columns:           []string{"user_id"},
		ReferencedTable:   "users",
		ReferencedColumns: []string{"id"},
	}

	if constraint.ReferencedTable != "users" {
		t.Fatalf("expected referenced table users, got %q", constraint.ReferencedTable)
	}
	if len(constraint.ReferencedColumns) != 1 || constraint.ReferencedColumns[0] != "id" {
		t.Fatalf("expected referenced columns [id], got %#v", constraint.ReferencedColumns)
	}

	data, err := json.Marshal(constraint)
	if err != nil {
		t.Fatalf("marshal constraint: %v", err)
	}
	var roundTrip Constraint
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal constraint: %v", err)
	}
	if roundTrip.ReferencedTable != "users" {
		t.Fatalf("expected referenced table to round-trip, got %q", roundTrip.ReferencedTable)
	}
	if len(roundTrip.ReferencedColumns) != 1 || roundTrip.ReferencedColumns[0] != "id" {
		t.Fatalf("expected referenced columns to round-trip, got %#v", roundTrip.ReferencedColumns)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got := payload["referenced_table"]; got != "users" {
		t.Fatalf("expected referenced_table json value users, got %#v", got)
	}
	if got := payload["referenced_columns"]; got == nil {
		t.Fatalf("expected referenced_columns json key to be present")
	}
}

func TestConstraintOmitsForeignKeyReferencedFieldsWhenAbsent(t *testing.T) {
	constraint := Constraint{
		Type:    "check",
		Name:    "chk_amount",
		Columns: []string{"amount"},
	}

	data, err := json.Marshal(constraint)
	if err != nil {
		t.Fatalf("marshal constraint: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload["referenced_table"]; ok {
		t.Fatalf("expected referenced_table to be omitted from json for non-FK constraint, got %#v", payload)
	}
	if _, ok := payload["referenced_columns"]; ok {
		t.Fatalf("expected referenced_columns to be omitted from json for non-FK constraint, got %#v", payload)
	}
}

func TestConstraintPreservesForeignKeyReferencedSchemaFacts(t *testing.T) {
	constraint := Constraint{
		Type:              "foreign_key",
		Name:              "fk_orders_user",
		Columns:           []string{"user_id"},
		ReferencedSchema:  "public",
		ReferencedTable:   "users",
		ReferencedColumns: []string{"id"},
	}

	if constraint.ReferencedSchema != "public" {
		t.Fatalf("expected referenced schema public, got %q", constraint.ReferencedSchema)
	}

	data, err := json.Marshal(constraint)
	if err != nil {
		t.Fatalf("marshal constraint: %v", err)
	}
	var roundTrip Constraint
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal constraint: %v", err)
	}
	if roundTrip.ReferencedSchema != "public" {
		t.Fatalf("expected referenced schema to round-trip, got %q", roundTrip.ReferencedSchema)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got := payload["referenced_schema"]; got != "public" {
		t.Fatalf("expected referenced_schema json value public, got %#v", got)
	}
}

func TestConstraintOmitsReferencedSchemaWhenAbsent(t *testing.T) {
	constraint := Constraint{
		Type:    "check",
		Name:    "chk_amount",
		Columns: []string{"amount"},
	}

	data, err := json.Marshal(constraint)
	if err != nil {
		t.Fatalf("marshal constraint: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload["referenced_schema"]; ok {
		t.Fatalf("expected referenced_schema to be omitted from json for non-FK constraint, got %#v", payload)
	}
}

func TestStatementDMLImpactFieldsPreserveZeroValueAndJSONBehavior(t *testing.T) {
	stmt := Statement{
		Kind:    KindDML,
		Dialect: DialectMySQL,
		DML: &DML{
			Operation:      DMLOperationUpdate,
			HasWhere:       true,
			PredicateShape: PredicateShapeUniqueEquality,
			LookupColumns:  []string{"id"},
			MatchedKeyName: "PRIMARY",
			MatchedKeyKind: IndexKindPrimary,
			IsSingleTable:  true,
			Impact: &ImpactEstimate{
				EstimatedRows:  ptrInt64(1),
				EstimatedRatio: ptrFloat64(0.01),
				RiskLevel:      ImpactRiskLow,
				Confidence:     ImpactConfidenceHigh,
				Source:         ImpactSourceShape,
				ReasonCodes:    []string{"pk_equality"},
				Notes:          []string{"shape only"},
			},
		},
		Metadata: &Metadata{
			TargetTable: &TableSnapshot{
				Exists: true,
				Table:  &Table{Name: "users"},
				PrimaryKey: &Index{
					Name:        "PRIMARY",
					Kind:        IndexKindPrimary,
					Columns:     []string{"id"},
					Cardinality: ptrInt64(1000),
				},
				Indexes: []Index{{
					Name:        "idx_users_email",
					Kind:        IndexKindSecondary,
					Columns:     []string{"email"},
					Cardinality: ptrInt64(250),
				}, {
					Name:        "idx_users_zero",
					Kind:        IndexKindSecondary,
					Columns:     []string{"tenant_id"},
					Cardinality: ptrInt64(0),
				}, {
					Name:    "idx_users_unknown",
					Kind:    IndexKindSecondary,
					Columns: []string{"created_at"},
				}},
			},
		},
	}

	data, err := json.Marshal(stmt)
	if err != nil {
		t.Fatalf("marshal statement: %v", err)
	}

	var roundTrip Statement
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal statement: %v", err)
	}

	if roundTrip.DML == nil || roundTrip.DML.Impact == nil {
		t.Fatalf("expected dml impact to round-trip, got %#v", roundTrip.DML)
	}
	if roundTrip.DML.PredicateShape != PredicateShapeUniqueEquality {
		t.Fatalf("expected predicate shape to round-trip, got %q", roundTrip.DML.PredicateShape)
	}
	if roundTrip.DML.Impact.Source != ImpactSourceShape {
		t.Fatalf("expected impact source to round-trip, got %#v", roundTrip.DML.Impact)
	}
	if roundTrip.Metadata == nil || roundTrip.Metadata.TargetTable == nil || roundTrip.Metadata.TargetTable.PrimaryKey == nil {
		t.Fatalf("expected metadata target table primary key, got %#v", roundTrip.Metadata)
	}
	if roundTrip.Metadata.TargetTable.PrimaryKey.Cardinality == nil || *roundTrip.Metadata.TargetTable.PrimaryKey.Cardinality != 1000 {
		t.Fatalf("expected primary key cardinality to round-trip, got %#v", roundTrip.Metadata.TargetTable.PrimaryKey)
	}
	if len(roundTrip.Metadata.TargetTable.Indexes) != 3 {
		t.Fatalf("expected index cardinality to round-trip, got %#v", roundTrip.Metadata.TargetTable.Indexes)
	}
	if roundTrip.Metadata.TargetTable.Indexes[0].Cardinality == nil || *roundTrip.Metadata.TargetTable.Indexes[0].Cardinality != 250 {
		t.Fatalf("expected present cardinality to round-trip, got %#v", roundTrip.Metadata.TargetTable.Indexes[0])
	}
	if roundTrip.Metadata.TargetTable.Indexes[1].Cardinality == nil || *roundTrip.Metadata.TargetTable.Indexes[1].Cardinality != 0 {
		t.Fatalf("expected explicit zero cardinality to round-trip, got %#v", roundTrip.Metadata.TargetTable.Indexes[1])
	}
	if roundTrip.Metadata.TargetTable.Indexes[2].Cardinality != nil {
		t.Fatalf("expected unknown cardinality to remain absent, got %#v", roundTrip.Metadata.TargetTable.Indexes[2])
	}

	var richPayload map[string]any
	if err := json.Unmarshal(data, &richPayload); err != nil {
		t.Fatalf("unmarshal rich payload: %v", err)
	}
	metadataPayload, ok := richPayload["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata payload, got %#v", richPayload)
	}
	targetTablePayload, ok := metadataPayload["target_table"].(map[string]any)
	if !ok {
		t.Fatalf("expected target_table payload, got %#v", metadataPayload)
	}
	primaryKeyPayload, ok := targetTablePayload["primary_key"].(map[string]any)
	if !ok {
		t.Fatalf("expected primary_key payload, got %#v", targetTablePayload)
	}
	if got := primaryKeyPayload["cardinality"]; got != float64(1000) {
		t.Fatalf("expected primary key cardinality json value 1000, got %#v", got)
	}
	indexesPayload, ok := targetTablePayload["indexes"].([]any)
	if !ok || len(indexesPayload) != 3 {
		t.Fatalf("expected indexes payload, got %#v", targetTablePayload["indexes"])
	}
	firstIndex, _ := indexesPayload[0].(map[string]any)
	secondIndex, _ := indexesPayload[1].(map[string]any)
	thirdIndex, _ := indexesPayload[2].(map[string]any)
	if got := firstIndex["cardinality"]; got != float64(250) {
		t.Fatalf("expected first index cardinality json value 250, got %#v", got)
	}
	if got := secondIndex["cardinality"]; got != float64(0) {
		t.Fatalf("expected explicit zero cardinality json value 0, got %#v", got)
	}
	if _, ok := thirdIndex["cardinality"]; ok {
		t.Fatalf("expected unknown cardinality to be omitted from json, got %#v", thirdIndex)
	}

	zero := Statement{
		Kind:    KindDML,
		Dialect: DialectMySQL,
		DML: &DML{
			Operation: DMLOperationDelete,
		},
	}

	data, err = json.Marshal(zero)
	if err != nil {
		t.Fatalf("marshal zero statement: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal zero payload: %v", err)
	}
	dmlPayload, ok := payload["dml"].(map[string]any)
	if !ok {
		t.Fatalf("expected dml payload, got %#v", payload)
	}
	if _, ok := dmlPayload["predicate_shape"]; ok {
		t.Fatalf("expected zero predicate_shape to omit from json, got %#v", dmlPayload)
	}
	if _, ok := dmlPayload["impact"]; ok {
		t.Fatalf("expected zero impact to omit from json, got %#v", dmlPayload)
	}
}

// ---------------------------------------------------------------------------
// v0.33.0 Task 2: Contract tests — generated/identity fact preservation
// ---------------------------------------------------------------------------
// These tests assert that Column and UnsupportedDetail carry generated/identity
// facts and that they round-trip through JSON with omitempty behavior.
// ---------------------------------------------------------------------------

func TestColumnPreservesGeneratedWhenFact(t *testing.T) {
	col := Column{
		Name:          "full_name",
		Type:          "text",
		GeneratedWhen: "a",
	}

	if col.GeneratedWhen != "a" {
		t.Fatalf("expected GeneratedWhen 'a', got %q", col.GeneratedWhen)
	}

	data, err := json.Marshal(col)
	if err != nil {
		t.Fatalf("marshal column: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got := payload["generated_when"]; got != "a" {
		t.Fatalf("expected generated_when json value 'a', got %#v", got)
	}
}

func TestColumnOmitsGeneratedWhenWhenEmpty(t *testing.T) {
	col := Column{Name: "email", Type: "text"}

	data, err := json.Marshal(col)
	if err != nil {
		t.Fatalf("marshal column: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload["generated_when"]; ok {
		t.Fatalf("expected generated_when to be omitted when empty, got %#v", payload)
	}
}

func TestColumnPreservesIsIdentityFact(t *testing.T) {
	col := Column{
		Name:        "id",
		Type:        "bigint",
		IsIdentity:  true,
	}

	if !col.IsIdentity {
		t.Fatal("expected IsIdentity true")
	}

	data, err := json.Marshal(col)
	if err != nil {
		t.Fatalf("marshal column: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got := payload["is_identity"]; got != true {
		t.Fatalf("expected is_identity json value true, got %#v", got)
	}
}

func TestColumnOmitsIsIdentityWhenFalse(t *testing.T) {
	col := Column{Name: "email", Type: "text"}

	data, err := json.Marshal(col)
	if err != nil {
		t.Fatalf("marshal column: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload["is_identity"]; ok {
		t.Fatalf("expected is_identity to be omitted when false, got %#v", payload)
	}
}

func TestColumnPreservesIdentityOptionsFact(t *testing.T) {
	col := Column{
		Name:    "id",
		Type:    "bigint",
		IdentityOptions: map[string]any{
			"start":    int32(10),
			"increment": int32(5),
			"cycle":    true,
		},
	}

	if col.IdentityOptions == nil {
		t.Fatal("expected IdentityOptions to be populated")
	}
	if col.IdentityOptions["start"] != int32(10) {
		t.Fatalf("expected start=10, got %v", col.IdentityOptions["start"])
	}
	if col.IdentityOptions["cycle"] != true {
		t.Fatalf("expected cycle=true, got %v", col.IdentityOptions["cycle"])
	}

	data, err := json.Marshal(col)
	if err != nil {
		t.Fatalf("marshal column: %v", err)
	}
	var roundTrip Column
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	if roundTrip.IdentityOptions == nil {
		t.Fatal("expected IdentityOptions to survive round-trip")
	}
}

func TestColumnOmitsIdentityOptionsWhenNil(t *testing.T) {
	col := Column{Name: "email", Type: "text"}

	data, err := json.Marshal(col)
	if err != nil {
		t.Fatalf("marshal column: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload["identity_options"]; ok {
		t.Fatalf("expected identity_options to be omitted when nil, got %#v", payload)
	}
}

func TestUnsupportedDetailPreservesMetadata(t *testing.T) {
	detail := UnsupportedDetail{
		Feature: "generated_column",
		Reason:  "unsupported in v1",
		Metadata: map[string]any{
			"column":         "full_name",
			"generated_when": "a",
		},
	}

	if detail.Metadata == nil {
		t.Fatal("expected Metadata to be populated")
	}
	if detail.Metadata["column"] != "full_name" {
		t.Fatalf("expected metadata column=full_name, got %v", detail.Metadata["column"])
	}

	data, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}
	var roundTrip UnsupportedDetail
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	if roundTrip.Metadata == nil {
		t.Fatal("expected Metadata to survive round-trip")
	}
	if roundTrip.Metadata["column"] != "full_name" {
		t.Fatalf("expected metadata column to round-trip, got %v", roundTrip.Metadata["column"])
	}
}

func TestUnsupportedDetailOmitsMetadataWhenNil(t *testing.T) {
	detail := UnsupportedDetail{
		Feature: "select",
		Reason:  "unsupported",
	}

	data, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload["metadata"]; ok {
		t.Fatalf("expected metadata to be omitted when nil, got %#v", payload)
	}
}

func TestIndexPreservesAdvancedFactsInJSON(t *testing.T) {
	idx := Index{
		Name:              "idx_active",
		Kind:              IndexKindSecondary,
		Columns:           []string{"email"},
		AccessMethod:      "btree",
		IncludedColumns:   []string{"name"},
		HasPredicate:      true,
		HasExpressionKeys: true,
		ExpressionCount:   1,
	}

	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if got := payload["access_method"]; got != "btree" {
		t.Fatalf("expected access_method 'btree', got %#v", got)
	}
	if got := payload["has_predicate"]; got != true {
		t.Fatalf("expected has_predicate true, got %#v", got)
	}
	if got := payload["has_expression_keys"]; got != true {
		t.Fatalf("expected has_expression_keys true, got %#v", got)
	}
	if got := payload["expression_count"]; got != float64(1) {
		t.Fatalf("expected expression_count 1, got %#v", got)
	}

	incRaw, ok := payload["included_columns"].([]any)
	if !ok || len(incRaw) != 1 || incRaw[0] != "name" {
		t.Fatalf("expected included_columns [name], got %#v", payload["included_columns"])
	}

	var roundTrip Index
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	if roundTrip.AccessMethod != "btree" {
		t.Fatalf("expected access_method to round-trip, got %q", roundTrip.AccessMethod)
	}
	if !roundTrip.HasPredicate {
		t.Fatal("expected HasPredicate to round-trip")
	}
}

func TestIndexOmitsAdvancedFactsWhenZero(t *testing.T) {
	idx := Index{
		Name:  "idx_simple",
		Kind:  IndexKindSecondary,
		Columns: []string{"email"},
	}

	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if _, ok := payload["access_method"]; ok {
		t.Fatalf("expected access_method omitted when empty, got %#v", payload)
	}
	if _, ok := payload["included_columns"]; ok {
		t.Fatalf("expected included_columns omitted when nil, got %#v", payload)
	}
	if _, ok := payload["has_predicate"]; ok {
		t.Fatalf("expected has_predicate omitted when false, got %#v", payload)
	}
	if _, ok := payload["has_expression_keys"]; ok {
		t.Fatalf("expected has_expression_keys omitted when false, got %#v", payload)
	}
	if _, ok := payload["expression_count"]; ok {
		t.Fatalf("expected expression_count omitted when zero, got %#v", payload)
	}
}

func ptrInt64(value int64) *int64 {
	return &value
}

func ptrFloat64(value float64) *float64 {
	return &value
}

func TestDDLObjectLifecycleFieldsSerializeCorrectly(t *testing.T) {
	ddl := &DDL{
		Operation:  DDLOperationDropSchema,
		ObjectName: "staging",
		ObjectType: "schema",
		Options:    map[string]string{"if_exists": "true", "cascade": "true"},
	}

	data, err := json.Marshal(ddl)
	if err != nil {
		t.Fatalf("marshal ddl: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if got := payload["object_name"]; got != "staging" {
		t.Fatalf("expected object_name 'staging', got %#v", got)
	}
	if got := payload["object_type"]; got != "schema" {
		t.Fatalf("expected object_type 'schema', got %#v", got)
	}
	if got := payload["operation"]; got != "drop_schema" {
		t.Fatalf("expected operation 'drop_schema', got %#v", got)
	}

	opts, ok := payload["options"].(map[string]any)
	if !ok {
		t.Fatalf("expected options map, got %#v", payload["options"])
	}
	if got := opts["if_exists"]; got != "true" {
		t.Fatalf("expected if_exists 'true', got %#v", got)
	}
	if got := opts["cascade"]; got != "true" {
		t.Fatalf("expected cascade 'true', got %#v", got)
	}
}

func TestDDLTypeLifecycleOperationsSerializeCorrectly(t *testing.T) {
	createType := &DDL{
		Operation:  DDLOperationCreateType,
		ObjectName: "color",
		ObjectType: "type",
		Options:    map[string]string{"type_kind": "enum", "labels": "red,green,blue"},
	}
	data, err := json.Marshal(createType)
	if err != nil {
		t.Fatalf("marshal create_type ddl: %v", err)
	}
	var rt DDL
	if err := json.Unmarshal(data, &rt); err != nil {
		t.Fatalf("unmarshal create_type ddl: %v", err)
	}
	if rt.Operation != "create_type" {
		t.Fatalf("expected operation create_type, got %q", rt.Operation)
	}
	if rt.ObjectName != "color" {
		t.Fatalf("expected object_name color, got %q", rt.ObjectName)
	}
	if rt.Options["labels"] != "red,green,blue" {
		t.Fatalf("expected labels red,green,blue, got %q", rt.Options["labels"])
	}

	alterType := &DDL{
		Operation:  DDLOperationAlterType,
		ObjectName: "color",
		ObjectType: "type",
		Options:    map[string]string{"type_kind": "enum", "action": "add_value", "value": "yellow", "if_not_exists": "true", "placement": "after", "neighbor": "green"},
	}
	data, err = json.Marshal(alterType)
	if err != nil {
		t.Fatalf("marshal alter_type ddl: %v", err)
	}
	if err := json.Unmarshal(data, &rt); err != nil {
		t.Fatalf("unmarshal alter_type ddl: %v", err)
	}
	if rt.Operation != "alter_type" {
		t.Fatalf("expected operation alter_type, got %q", rt.Operation)
	}
	if rt.Options["placement"] != "after" {
		t.Fatalf("expected placement after, got %q", rt.Options["placement"])
	}

	dropType := &DDL{
		Operation:  DDLOperationDropType,
		ObjectName: "color",
		ObjectType: "type",
		Options:    map[string]string{"if_exists": "true", "cascade": "true"},
	}
	data, err = json.Marshal(dropType)
	if err != nil {
		t.Fatalf("marshal drop_type ddl: %v", err)
	}
	if err := json.Unmarshal(data, &rt); err != nil {
		t.Fatalf("unmarshal drop_type ddl: %v", err)
	}
	if rt.Operation != "drop_type" {
		t.Fatalf("expected operation drop_type, got %q", rt.Operation)
	}
	if rt.Options["cascade"] != "true" {
		t.Fatalf("expected cascade true, got %q", rt.Options["cascade"])
	}
}

func TestDDLDomainOperationsRoundTrip(t *testing.T) {
	var rt DDL

	createDomain := &DDL{
		Operation:  DDLOperationCreateDomain,
		ObjectName: "email",
		ObjectType: "domain",
		Options:    map[string]string{"type_kind": "domain", "base_type": "text", "not_null": "true"},
	}
	data, err := json.Marshal(createDomain)
	if err != nil {
		t.Fatalf("marshal create_domain ddl: %v", err)
	}
	if err := json.Unmarshal(data, &rt); err != nil {
		t.Fatalf("unmarshal create_domain ddl: %v", err)
	}
	if rt.Operation != "create_domain" {
		t.Fatalf("expected operation create_domain, got %q", rt.Operation)
	}
	if rt.Options["base_type"] != "text" {
		t.Fatalf("expected base_type text, got %q", rt.Options["base_type"])
	}

	alterDomain := &DDL{
		Operation:  DDLOperationAlterDomain,
		ObjectName: "email",
		ObjectType: "domain",
		Options:    map[string]string{"action": "add_constraint", "constraint": "chk_email", "has_check": "true"},
	}
	data, err = json.Marshal(alterDomain)
	if err != nil {
		t.Fatalf("marshal alter_domain ddl: %v", err)
	}
	if err := json.Unmarshal(data, &rt); err != nil {
		t.Fatalf("unmarshal alter_domain ddl: %v", err)
	}
	if rt.Operation != "alter_domain" {
		t.Fatalf("expected operation alter_domain, got %q", rt.Operation)
	}
	if rt.Options["action"] != "add_constraint" {
		t.Fatalf("expected action add_constraint, got %q", rt.Options["action"])
	}

	dropDomain := &DDL{
		Operation:  DDLOperationDropDomain,
		ObjectName: "email",
		ObjectType: "domain",
		Options:    map[string]string{"if_exists": "true", "cascade": "true"},
	}
	data, err = json.Marshal(dropDomain)
	if err != nil {
		t.Fatalf("marshal drop_domain ddl: %v", err)
	}
	if err := json.Unmarshal(data, &rt); err != nil {
		t.Fatalf("unmarshal drop_domain ddl: %v", err)
	}
	if rt.Operation != "drop_domain" {
		t.Fatalf("expected operation drop_domain, got %q", rt.Operation)
	}
	if rt.Options["cascade"] != "true" {
		t.Fatalf("expected cascade true, got %q", rt.Options["cascade"])
	}
}

func TestDDLObjectFieldsOmitWhenEmpty(t *testing.T) {
	ddl := &DDL{
		Operation: DDLOperationCreateIndex,
		Table:     &Table{Name: "users"},
		Indexes: []Index{
			{Name: "idx_email", Kind: IndexKindSecondary, Columns: []string{"email"}},
		},
	}

	data, err := json.Marshal(ddl)
	if err != nil {
		t.Fatalf("marshal ddl: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if _, ok := payload["object_name"]; ok {
		t.Fatalf("expected object_name to be omitted when empty, got %#v", payload)
	}
	if _, ok := payload["object_type"]; ok {
		t.Fatalf("expected object_type to be omitted when empty, got %#v", payload)
	}
}
