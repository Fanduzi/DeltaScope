// Package ddl verifies the T06-A15 field-level fact resolution for the
// row-size rule.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t06a15RowSizeStatement builds one CREATE TABLE statement carrying only the
// declared inputs the role names: an optional engine, charset, and row_format
// table option plus the column list. instance may be nil (offline shape) or a
// non-nil object with only the fields the role provides.
func t06a15RowSizeStatement(dialect spec.Dialect, options map[string]string, columns []spec.Column, instance *spec.InstanceFacts) spec.Statement {
	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: dialect,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "t"},
			Options:   options,
			Columns:   columns,
		},
	}
	if instance != nil {
		statement.Metadata = &spec.Metadata{Instance: instance}
	}
	return statement
}

const (
	// Stable fact identifiers pinned by the T06-A15 contract — declared
	// locally so the assertions compile against the pre-fix baseline too.
	t06a15FactDefaultCharset = "instance.default_charset"
	t06a15FactRowFormat      = "instance.innodb_default_row_format"
)

func t06a15RowSizeRule(t *testing.T, required bool) rule.StatementRule {
	t.Helper()
	ruleUnderTest, err := newTableRowSizeRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": required},
	})
	if err != nil {
		t.Fatalf("new row-size rule: %v", err)
	}
	return ruleUnderTest
}

// t06a15Gaps returns the rule's reported evidence gaps through the
// EvidenceReporter seam — the same surface the registry consumes.
func t06a15Gaps(t *testing.T, ruleUnderTest rule.StatementRule, statement spec.Statement) []rule.EvidenceGap {
	t.Helper()
	reporter, ok := ruleUnderTest.(rule.EvidenceReporter)
	if !ok {
		t.Fatal("row-size rule must implement EvidenceReporter")
	}
	return reporter.EvidenceGaps(statement)
}

func t06a15AssertGaps(t *testing.T, gaps []rule.EvidenceGap, facts ...string) {
	t.Helper()
	if len(facts) == 0 {
		if len(gaps) != 0 {
			t.Fatalf("expected no gaps, got %+v", gaps)
		}
		return
	}
	if len(gaps) != 1 {
		t.Fatalf("expected exactly one gap, got %+v", gaps)
	}
	if gaps[0].ReasonCode != "missing_instance_fact" {
		t.Fatalf("expected missing_instance_fact gap, got %+v", gaps[0])
	}
	if len(gaps[0].RequiredFacts) != len(facts) {
		t.Fatalf("expected facts %v, got %v", facts, gaps[0].RequiredFacts)
	}
	for i, fact := range facts {
		if gaps[0].RequiredFacts[i] != fact {
			t.Fatalf("expected facts %v, got %v", facts, gaps[0].RequiredFacts)
		}
	}
}

// Declared-complete statements evaluate identically with a nil instance: the
// 65535 boundary stays strict-greater under MySQL and TiDB.
func TestT06A15DeclaredInputsComputeWithoutInstance(t *testing.T) {
	t.Parallel()
	options := map[string]string{"engine": "InnoDB", "charset": "latin1", "row_format": "DYNAMIC"}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			ruleUnderTest := t06a15RowSizeRule(t, true)
			for _, tc := range []struct {
				name      string
				length    int
				findings  int
				estimated int
			}{
				{"below", 65532, 0, 0},
				{"equal", 65533, 0, 0},
				{"above", 65534, 1, 65536},
			} {
				t.Run(tc.name, func(t *testing.T) {
					statement := t06a15RowSizeStatement(dialect, options,
						[]spec.Column{{Name: "c", Type: "varchar", Length: tc.length}}, nil)
					findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
					if err != nil {
						t.Fatalf("evaluate: %v", err)
					}
					if len(findings) != tc.findings {
						t.Fatalf("expected %d findings, got %+v", tc.findings, findings)
					}
					if tc.findings == 1 {
						if got := findings[0].Metadata["estimated"]; got != tc.estimated {
							t.Fatalf("expected estimated=%d, got %v", tc.estimated, got)
						}
						if got := findings[0].Metadata["charset"]; got != "latin1" {
							t.Fatalf("expected table charset latin1, got %v", got)
						}
						if got := findings[0].Metadata["row_format"]; got != "DYNAMIC" {
							t.Fatalf("expected row_format DYNAMIC, got %v", got)
						}
					}
					t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, statement))
				})
			}
		})
	}
}

// A column-level charset wins over the table declaration for the estimate
// while finding metadata keeps the table-level value.
func TestT06A15ColumnCharsetOverridesTableCharset(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t06a15RowSizeRule(t, true)
	statement := t06a15RowSizeStatement(spec.DialectMySQL,
		map[string]string{"engine": "InnoDB", "charset": "latin1", "row_format": "DYNAMIC"},
		[]spec.Column{{Name: "c", Type: "varchar", Length: 16384, Charset: "utf8mb4"}}, nil)

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %+v", findings)
	}
	if got := findings[0].Metadata["estimated"]; got != 65538 {
		t.Fatalf("expected estimated=65538, got %v", got)
	}
	if got := findings[0].Metadata["charset"]; got != "latin1" {
		t.Fatalf("metadata.charset keeps the table-level value, got %v", got)
	}
	t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, statement))
}

// When every charset-sensitive column declares its own charset, an omitted
// table charset is not a required fact: no gap fires and the unresolved
// table-level value stays "" in finding metadata.
func TestT06A15ColumnOnlyCharsetNeedsNoTableCharset(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t06a15RowSizeRule(t, true)
	statement := t06a15RowSizeStatement(spec.DialectMySQL,
		map[string]string{"engine": "InnoDB", "row_format": "DYNAMIC"},
		[]spec.Column{{Name: "c", Type: "varchar", Length: 16384, Charset: "utf8mb4"}}, nil)

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["estimated"] != 65538 {
		t.Fatalf("expected one finding estimated=65538, got %+v", findings)
	}
	if got := findings[0].Metadata["charset"]; got != "" {
		t.Fatalf("expected empty table-level charset metadata, got %v", got)
	}
	t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, statement))
}

// A charset-sensitive column without any declaration source reports the
// default_charset fact and emits no fabricated estimate.
func TestT06A15MissingCharsetReportsGapNotEstimate(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			ruleUnderTest := t06a15RowSizeRule(t, true)
			statement := t06a15RowSizeStatement(dialect,
				map[string]string{"engine": "InnoDB", "row_format": "DYNAMIC"},
				[]spec.Column{{Name: "c", Type: "varchar", Length: 65534}}, nil)

			findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("expected no fabricated estimate, got %+v", findings)
			}
			t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, statement), t06a15FactDefaultCharset)
		})
	}
}

// An unresolved row format never falls back to DYNAMIC: the COMPACT/REDUNDANT
// branch stays unverified and reports the fact, while a proven total-size
// finding is preserved.
func TestT06A15MissingRowFormatReportsGap(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t06a15RowSizeRule(t, true)
	for _, tc := range []struct {
		name     string
		length   int
		findings int
		rowMeta  string
	}{
		{"safe-total", 65532, 0, ""},
		{"over-total", 65534, 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statement := t06a15RowSizeStatement(spec.DialectMySQL,
				map[string]string{"engine": "InnoDB", "charset": "latin1"},
				[]spec.Column{{Name: "c", Type: "varchar", Length: tc.length}}, nil)
			findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != tc.findings {
				t.Fatalf("expected %d findings, got %+v", tc.findings, findings)
			}
			if tc.findings == 1 {
				if got := findings[0].Metadata["row_format"]; got != tc.rowMeta {
					t.Fatalf("expected row_format %q in metadata, got %v", tc.rowMeta, got)
				}
			}
			t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, statement), t06a15FactRowFormat)
		})
	}
}

// Both missing fields report in one gap, in the fixed
// default-charset-then-row-format order, without duplicates.
func TestT06A15MissingBothFactsOrderedOnce(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t06a15RowSizeRule(t, true)
	statement := t06a15RowSizeStatement(spec.DialectMySQL,
		map[string]string{"engine": "InnoDB"},
		[]spec.Column{{Name: "c", Type: "varchar", Length: 65534}}, nil)

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
	t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, statement),
		t06a15FactDefaultCharset, t06a15FactRowFormat)
}

// ROW_FORMAT=DEFAULT is not a known format: it resolves through the real
// instance default when present, and reports the missing fact offline.
func TestT06A15DefaultRowFormatNeedsInstanceFact(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t06a15RowSizeRule(t, true)
	columns := []spec.Column{{Name: "c", Type: "varchar", Length: 65534}}

	offline := t06a15RowSizeStatement(spec.DialectMySQL,
		map[string]string{"engine": "InnoDB", "charset": "latin1", "row_format": "DEFAULT"}, columns, nil)
	findings, err := ruleUnderTest.Evaluate(context.Background(), offline)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["row_format"] != "" {
		t.Fatalf("expected total finding with empty row_format, got %+v", findings)
	}
	t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, offline), t06a15FactRowFormat)

	withInstance := t06a15RowSizeStatement(spec.DialectMySQL,
		map[string]string{"engine": "InnoDB", "charset": "latin1", "row_format": "DEFAULT"}, columns,
		&spec.InstanceFacts{InnoDBDefaultRowFormat: "COMPACT"})
	findings, err = ruleUnderTest.Evaluate(context.Background(), withInstance)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["row_format"] != "COMPACT" {
		t.Fatalf("expected instance default COMPACT to resolve DEFAULT, got %+v", findings)
	}
	t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, withInstance))
}

// Explicit COMPACT and REDUNDANT declarations evaluate their existing bounds
// without any instance field.
func TestT06A15DeclaredCompactRedundantNeedNoInstance(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t06a15RowSizeRule(t, true)
	// VARCHAR(1000) utf8mb4 → 4002 bytes per column; COMPACT caps each at 768.
	columns := []spec.Column{{Name: "c", Type: "varchar", Length: 1000, Charset: "utf8mb4"}}
	for _, tc := range []struct {
		format  string
		length  int
		wantHit bool
	}{
		// eleven columns overflow the 8126 COMPACT bound (11×768=8448).
		{"COMPACT", 1000, true},
		{"REDUNDANT", 1000, true},
	} {
		many := make([]spec.Column, 0, 11)
		for i := 0; i < 11; i++ {
			many = append(many, columns[0])
		}
		statement := t06a15RowSizeStatement(spec.DialectMySQL,
			map[string]string{"engine": "InnoDB", "charset": "utf8mb4", "row_format": tc.format}, many, nil)
		findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
		if err != nil {
			t.Fatalf("evaluate %s: %v", tc.format, err)
		}
		// The total finding also fires (11×4002=44022 stays under 65535 — only
		// the format-specific bound fires).
		if len(findings) != 1 {
			t.Fatalf("%s: expected exactly the format finding, got %+v", tc.format, findings)
		}
		if got := findings[0].Metadata["row_format"]; got != tc.format {
			t.Fatalf("%s: expected row_format %s, got %v", tc.format, tc.format, got)
		}
		t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, statement))
	}
}

// Fixed-width columns never require a charset fact: an INT-only table without
// any charset declaration computes its total and only reports the unresolved
// row format.
func TestT06A15FixedWidthColumnsNeedNoCharset(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t06a15RowSizeRule(t, true)
	statement := t06a15RowSizeStatement(spec.DialectMySQL,
		map[string]string{"engine": "InnoDB"},
		[]spec.Column{{Name: "c", Type: "int"}, {Name: "d", Type: "bigint"}}, nil)

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
	t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, statement), t06a15FactRowFormat)
}

// A declared non-InnoDB engine stays out of scope before any fact lookup: no
// finding, no gap — the rule simply does not check the statement.
func TestT06A15DeclaredNonInnoDBStaysSilent(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t06a15RowSizeRule(t, true)
	statement := t06a15RowSizeStatement(spec.DialectMySQL,
		map[string]string{"engine": "MyISAM", "charset": "latin1", "row_format": "DYNAMIC"},
		[]spec.Column{{Name: "c", Type: "varchar", Length: 65534}}, nil)

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
	t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, statement))
}

// required=false keeps the rule loaded but not applicable: no findings and no
// gaps under either the direct seam or the registry aggregation.
func TestT06A15RequiredFalseProducesNothing(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t06a15RowSizeRule(t, false)
	statement := t06a15RowSizeStatement(spec.DialectMySQL,
		map[string]string{"engine": "InnoDB"},
		[]spec.Column{{Name: "c", Type: "varchar", Length: 65534}}, nil)

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
	if got := t06a15Gaps(t, ruleUnderTest, statement); len(got) != 0 {
		t.Fatalf("expected no gaps, got %+v", got)
	}
}

// The empty-instance object is not a fact source: it resolves identically to
// a nil instance for the same statement.
func TestT06A15EmptyInstanceEqualsNilInstance(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t06a15RowSizeRule(t, true)
	options := map[string]string{"engine": "InnoDB", "charset": "latin1", "row_format": "DYNAMIC"}
	columns := []spec.Column{{Name: "c", Type: "varchar", Length: 65534}}

	nilStatement := t06a15RowSizeStatement(spec.DialectMySQL, options, columns, nil)
	emptyStatement := t06a15RowSizeStatement(spec.DialectMySQL, options, columns, &spec.InstanceFacts{})

	nilFindings, err := ruleUnderTest.Evaluate(context.Background(), nilStatement)
	if err != nil {
		t.Fatalf("evaluate nil: %v", err)
	}
	emptyFindings, err := ruleUnderTest.Evaluate(context.Background(), emptyStatement)
	if err != nil {
		t.Fatalf("evaluate empty: %v", err)
	}
	if len(nilFindings) != 1 || len(emptyFindings) != 1 {
		t.Fatalf("expected both shapes to emit one finding, got nil=%+v empty=%+v", nilFindings, emptyFindings)
	}
	if nilFindings[0].Metadata["estimated"] != emptyFindings[0].Metadata["estimated"] {
		t.Fatalf("nil/empty divergence: %v vs %v", nilFindings[0].Metadata, emptyFindings[0].Metadata)
	}
	t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, nilStatement))
	t06a15AssertGaps(t, t06a15Gaps(t, ruleUnderTest, emptyStatement))

	// And the missing-fact shape agrees too: neither object fabricates fields.
	undeclared := map[string]string{"engine": "InnoDB"}
	nilGaps := t06a15Gaps(t, ruleUnderTest, t06a15RowSizeStatement(spec.DialectMySQL, undeclared, columns, nil))
	emptyGaps := t06a15Gaps(t, ruleUnderTest, t06a15RowSizeStatement(spec.DialectMySQL, undeclared, columns, &spec.InstanceFacts{}))
	t06a15AssertGaps(t, nilGaps, t06a15FactDefaultCharset, t06a15FactRowFormat)
	t06a15AssertGaps(t, emptyGaps, t06a15FactDefaultCharset, t06a15FactRowFormat)
}

// Non-MySQL-family dialects keep the historical gate: no instance object
// means silence, and the new gap vocabulary never leaks to PostgreSQL.
func TestT06A15PostgreSQLKeepsLegacyGate(t *testing.T) {
	t.Parallel()
	ruleUnderTest := t06a15RowSizeRule(t, true)
	statement := t06a15RowSizeStatement(spec.DialectPostgreSQL,
		map[string]string{"engine": "InnoDB"},
		[]spec.Column{{Name: "c", Type: "varchar", Length: 65534}}, nil)

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings without instance, got %+v", findings)
	}
	if got := t06a15Gaps(t, ruleUnderTest, statement); len(got) != 0 {
		t.Fatalf("expected no gap leakage to PostgreSQL, got %+v", got)
	}
}
