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

func ptrInt64(value int64) *int64 {
	return &value
}

func ptrFloat64(value float64) *float64 {
	return &value
}
