// Package tidbparser verifies the T23-R8/R9 standalone SPLIT and
// ALTER...COMPACT extraction: object identity, index/member targets,
// clause variants, positional value classes, the typed integer payload
// for foldable literals, and the REGIONS count — raw literal text never
// enters the normalized model (issue #102).
package tidbparser

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func extractT23R8(t *testing.T, dialect spec.Dialect, sql string) spec.Statement {
	t.Helper()

	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	if len(wrapped) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(wrapped))
	}
	statement, err := wrapped[0].Extractor.Extract(dialect, wrapped[0].RawSQL)
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	return statement
}

func TestT23R8SplitExtraction(t *testing.T) {
	lit := spec.SplitBoundValue{Class: spec.SplitValueLiteral}
	cases := []struct {
		name           string
		sql            string
		table          string
		index          string
		hasClause      bool
		partitionNames []string
		arities        []int
		classes        [][]spec.SplitBoundValue
		lower, upper   int
		num            *int64
	}{
		{
			name:  "between_table",
			sql:   "SPLIT TABLE t BETWEEN (0) AND (100) REGIONS 4;",
			table: "t", lower: 1, upper: 1,
			classes: nil,
			num:     int64Ptr(4),
		},
		{
			name:  "between_schema_index",
			sql:   "SPLIT TABLE s.t INDEX ix_a BETWEEN (0) AND (100) REGIONS 8;",
			table: "t", index: "ix_a", lower: 1, upper: 1,
			num: int64Ptr(8),
		},
		{
			name:    "by_tuples",
			sql:     "SPLIT TABLE t BY (1),(2),(3);",
			table:   "t",
			arities: []int{1, 1, 1},
			classes: [][]spec.SplitBoundValue{{lit}, {lit}, {lit}},
		},
		{
			name:  "partition_clause_variant",
			sql:   "SPLIT PARTITION TABLE t BETWEEN (0) AND (100) REGIONS 2;",
			table: "t", hasClause: true, lower: 1, upper: 1,
			num: int64Ptr(2),
		},
		{
			name:  "region_for_variant",
			sql:   "SPLIT REGION FOR TABLE t BETWEEN (0) AND (100) REGIONS 2;",
			table: "t", lower: 1, upper: 1,
			num: int64Ptr(2),
		},
		{
			name:           "named_partitions",
			sql:            "SPLIT TABLE t PARTITION (p0, P1) BETWEEN (0) AND (100) REGIONS 2;",
			table:          "t",
			partitionNames: []string{"p0", "p1"},
			lower:          1, upper: 1,
			num: int64Ptr(2),
		},
		{
			name:  "value_classes",
			sql:   "SPLIT TABLE t BETWEEN (NULL, 'abc', 1+1) AND ('5', -3, (7)) REGIONS 2;",
			table: "t", lower: 3, upper: 3,
			num: int64Ptr(2),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			statement := extractT23R8(t, spec.DialectTiDB, tc.sql)
			if statement.Kind != spec.KindDDL {
				t.Fatalf("kind %v want ddl", statement.Kind)
			}
			if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationSplitRegion {
				t.Fatalf("operation %v want split_region", statement.DDL)
			}
			decl := statement.DDL.SplitRegion
			if decl == nil {
				t.Fatal("missing split_region declaration")
			}
			if statement.DDL.Table == nil || statement.DDL.Table.Name != tc.table {
				t.Fatalf("table %+v want %q", statement.DDL.Table, tc.table)
			}
			if decl.IndexName != tc.index {
				t.Fatalf("index %q want %q", decl.IndexName, tc.index)
			}
			if decl.HasPartitionClause != tc.hasClause {
				t.Fatalf("partition clause %t want %t", decl.HasPartitionClause, tc.hasClause)
			}
			if len(decl.PartitionNames) != len(tc.partitionNames) {
				t.Fatalf("partition names %v want %v", decl.PartitionNames, tc.partitionNames)
			}
			for i, name := range tc.partitionNames {
				if decl.PartitionNames[i] != name {
					t.Fatalf("partition name[%d] %q want %q", i, decl.PartitionNames[i], name)
				}
			}
			if len(decl.Bound.Arities) != len(tc.arities) {
				t.Fatalf("arities %v want %v", decl.Bound.Arities, tc.arities)
			}
			for i, arity := range tc.arities {
				if decl.Bound.Arities[i] != arity {
					t.Fatalf("arity[%d] %d want %d", i, decl.Bound.Arities[i], arity)
				}
			}
			for i, tuple := range tc.classes {
				for j, want := range tuple {
					if decl.Bound.Values[i][j].Class != want.Class {
						t.Fatalf("tuple[%d][%d] class %v want %v", i, j, decl.Bound.Values[i][j].Class, want.Class)
					}
				}
			}
			if decl.Bound.LowerArity != tc.lower || decl.Bound.UpperArity != tc.upper {
				t.Fatalf("bounds (%d,%d) want (%d,%d)", decl.Bound.LowerArity, decl.Bound.UpperArity, tc.lower, tc.upper)
			}
			if (decl.Num == nil) != (tc.num == nil) {
				t.Fatalf("num %v want %v", decl.Num, tc.num)
			}
			if tc.num != nil && *decl.Num != *tc.num {
				t.Fatalf("num %d want %d", *decl.Num, *tc.num)
			}
		})
	}
}

func TestT23R8SplitValueClasses(t *testing.T) {
	statement := extractT23R8(t, spec.DialectTiDB,
		"SPLIT TABLE t BETWEEN (NULL, 'abc', 1+1, '5') AND (-3, (7), ABS(1), 4.5) REGIONS 2;")
	decl := statement.DDL.SplitRegion
	wantLower := []spec.SplitValueClass{
		spec.SplitValueNull,
		spec.SplitValueNonNumeric, // 'abc' cannot convert to an integer member
		spec.SplitValueExpr,       // 1+1 is a non-literal expression
		spec.SplitValueLiteral,    // '5' converts to a number
	}
	wantUpper := []spec.SplitValueClass{
		spec.SplitValueLiteral, // unary - integer keeps literal class
		spec.SplitValueLiteral, // parenthesized literal unwraps
		spec.SplitValueExpr,    // ABS(1) is a non-literal expression
		spec.SplitValueLiteral, // float literal
	}
	if len(decl.Bound.Lower) != len(wantLower) || len(decl.Bound.Upper) != len(wantUpper) {
		t.Fatalf("classes (%v,%v)", decl.Bound.Lower, decl.Bound.Upper)
	}
	for i, want := range wantLower {
		if decl.Bound.Lower[i].Class != want {
			t.Fatalf("lower[%d] %v want %v", i, decl.Bound.Lower[i].Class, want)
		}
	}
	for i, want := range wantUpper {
		if decl.Bound.Upper[i].Class != want {
			t.Fatalf("upper[%d] %v want %v", i, decl.Bound.Upper[i].Class, want)
		}
	}
}

// TestT23R9SplitBoundPayloads pins the typed integer payload the
// extractor keeps for the int-handle arm: signed and unsigned literals,
// unary sign folding, integer strings, and the payload-free arms
// (floats, non-integer numerics, expressions, NULL) — raw text never
// enters the model.
func TestT23R9SplitBoundPayloads(t *testing.T) {
	cases := []struct {
		name     string
		sql      string
		position string
		index    int
		want     spec.SplitBoundValue
	}{
		{"signed_literal", "SPLIT TABLE t BETWEEN (-9223372036854775807) AND (100) REGIONS 4;", "lower", 0,
			spec.SplitBoundValue{Class: spec.SplitValueLiteral, Int: spec.SplitIntSigned, Int64: -9223372036854775807}},
		{"unsigned_literal", "SPLIT TABLE t BETWEEN (0) AND (18446744073709551615) REGIONS 4;", "upper", 0,
			spec.SplitBoundValue{Class: spec.SplitValueLiteral, Int: spec.SplitIntUnsigned, Uint64: 18446744073709551615}},
		{"unary_minus", "SPLIT TABLE t BETWEEN (-100) AND (0) REGIONS 4;", "lower", 0,
			spec.SplitBoundValue{Class: spec.SplitValueLiteral, Int: spec.SplitIntSigned, Int64: -100}},
		{"unary_minus_on_max", "SPLIT TABLE t BETWEEN (-18446744073709551615) AND (0) REGIONS 4;", "lower", 0,
			spec.SplitBoundValue{Class: spec.SplitValueLiteral, Int: spec.SplitIntNone}},
		{"integer_string", "SPLIT TABLE t BETWEEN ('42') AND (10000) REGIONS 4;", "lower", 0,
			spec.SplitBoundValue{Class: spec.SplitValueLiteral, Int: spec.SplitIntSigned, Int64: 42}},
		{"numeric_noninteger_string", "SPLIT TABLE t BETWEEN ('1.5') AND (10000) REGIONS 4;", "lower", 0,
			spec.SplitBoundValue{Class: spec.SplitValueLiteral, Int: spec.SplitIntNone}},
		{"float_literal", "SPLIT TABLE t BETWEEN (1.5) AND (10000) REGIONS 4;", "lower", 0,
			spec.SplitBoundValue{Class: spec.SplitValueLiteral, Int: spec.SplitIntNone}},
		{"expr_no_payload", "SPLIT TABLE t BETWEEN (abs(0)) AND (10000) REGIONS 4;", "lower", 0,
			spec.SplitBoundValue{Class: spec.SplitValueExpr, Int: spec.SplitIntNone}},
		{"null_no_payload", "SPLIT TABLE t BETWEEN (NULL) AND (10000) REGIONS 4;", "lower", 0,
			spec.SplitBoundValue{Class: spec.SplitValueNull, Int: spec.SplitIntNone}},
		{"bitwise_unary_expr", "SPLIT TABLE t BETWEEN (~1) AND (10000) REGIONS 4;", "lower", 0,
			spec.SplitBoundValue{Class: spec.SplitValueExpr, Int: spec.SplitIntNone}},
		{"unary_on_nonnumeric_string", "SPLIT TABLE t BETWEEN (-'abc') AND (10000) REGIONS 4;", "lower", 0,
			spec.SplitBoundValue{Class: spec.SplitValueExpr, Int: spec.SplitIntNone}},
		{"unary_on_numeric_string", "SPLIT TABLE t BETWEEN (-'42') AND (10000) REGIONS 4;", "lower", 0,
			spec.SplitBoundValue{Class: spec.SplitValueLiteral, Int: spec.SplitIntSigned, Int64: -42}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			statement := extractT23R8(t, spec.DialectTiDB, tc.sql)
			decl := statement.DDL.SplitRegion
			var got spec.SplitBoundValue
			switch tc.position {
			case "lower":
				got = decl.Bound.Lower[tc.index]
			case "upper":
				got = decl.Bound.Upper[tc.index]
			}
			if got.Class != tc.want.Class || got.Int != tc.want.Int ||
				got.Int64 != tc.want.Int64 || got.Uint64 != tc.want.Uint64 {
				t.Fatalf("bound %+v want %+v", got, tc.want)
			}
		})
	}
}

func TestT23R8CompactExtraction(t *testing.T) {
	cases := []struct {
		name           string
		sql            string
		table          string
		kind           string
		partitionNames []string
	}{
		{"tiflash", "ALTER TABLE t COMPACT TIFLASH REPLICA;", "t", "tiflash", nil},
		{"bare_normalized_all", "ALTER TABLE t COMPACT;", "t", "all", nil},
		{"schema", "ALTER TABLE s.t COMPACT TIFLASH REPLICA;", "t", "tiflash", nil},
		{"partition", "ALTER TABLE t COMPACT PARTITION p0, P1 TIFLASH REPLICA;", "t", "tiflash", []string{"p0", "p1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			statement := extractT23R8(t, spec.DialectTiDB, tc.sql)
			if statement.Kind != spec.KindDDL {
				t.Fatalf("kind %v want ddl", statement.Kind)
			}
			if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCompactTable {
				t.Fatalf("operation %v want compact_table", statement.DDL)
			}
			decl := statement.DDL.CompactTable
			if decl == nil {
				t.Fatal("missing compact declaration")
			}
			if statement.DDL.Table == nil || statement.DDL.Table.Name != tc.table {
				t.Fatalf("table %+v want %q", statement.DDL.Table, tc.table)
			}
			if decl.ReplicaKind != tc.kind {
				t.Fatalf("kind %q want %q", decl.ReplicaKind, tc.kind)
			}
			if len(decl.PartitionNames) != len(tc.partitionNames) {
				t.Fatalf("partition names %v want %v", decl.PartitionNames, tc.partitionNames)
			}
			for i, name := range tc.partitionNames {
				if decl.PartitionNames[i] != name {
					t.Fatalf("partition name[%d] %q want %q", i, decl.PartitionNames[i], name)
				}
			}
		})
	}
}

func TestT23R8AlterSplitIndexStaysBounded(t *testing.T) {
	// ALTER TABLE ... SPLIT INDEX is an embedded AlterTableSpec, not the
	// standalone statement — it stays bounded-unmodeled through the
	// existing unextracted-option channel.
	statement := extractT23R8(t, spec.DialectTiDB,
		"ALTER TABLE t SPLIT INDEX ix BETWEEN (0) AND (100) REGIONS 4;")
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("embedded split index must stay an alter, got %+v", statement.DDL)
	}
	found := false
	for _, alter := range statement.DDL.Alter {
		if alter.Action == "split_index" {
			found = true
		}
	}
	if !found {
		t.Fatalf("split_index must stay a bounded alter action, got %+v", statement.DDL.Alter)
	}
}

func int64Ptr(v int64) *int64 { return &v }
