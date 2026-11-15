// Package ddl verifies the T23 TiDB AUTO_RANDOM declaration-validity rule.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func newT23AutoRandomRule(t *testing.T) rule.StatementRule {
	t.Helper()
	statementRule, err := newAutoRandomValidRule(policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	return statementRule
}

// t23AutoRandomStatement builds a TiDB CREATE TABLE statement with one
// AUTO_RANDOM column shape. mode nil/empty means the clustered-index
// session mode is unobserved.
func t23AutoRandomStatement(column spec.Column, pk *spec.Index, mode string, alterPKKnown, alterPK bool) spec.Statement {
	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectTiDB,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateTable,
			Table:      &spec.Table{Name: "t"},
			Columns:    []spec.Column{column},
			PrimaryKey: pk,
		},
	}
	if mode != "" || alterPKKnown {
		statement.Metadata = &spec.Metadata{Instance: &spec.InstanceFacts{
			TiDBEnableClusteredIndex: mode,
			TiDBAlterPrimaryKey:      alterPK,
			TiDBAlterPrimaryKeyKnown: alterPKKnown,
		}}
	}
	return statement
}

func t23ClusteredPK(cols ...string) *spec.Index {
	return &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: cols, PrimaryKeyType: "clustered"}
}

func TestT23AutoRandomValidClusteredPK(t *testing.T) {
	t.Parallel()
	r := newT23AutoRandomRule(t)
	statement := t23AutoRandomStatement(
		spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true},
		t23ClusteredPK("id"), "", false, false)
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
	if gaps := r.(interface {
		EvidenceGaps(spec.Statement) []rule.EvidenceGap
	}).EvidenceGaps(statement); len(gaps) != 0 {
		t.Fatalf("expected no gaps, got %+v", gaps)
	}
}

func TestT23AutoRandomValidMatrix(t *testing.T) {
	t.Parallel()
	r := newT23AutoRandomRule(t)
	shard0 := int64(0)
	shard16 := int64(16)
	shard15 := int64(15)
	range31 := int64(31)
	range41 := int64(41)
	range65 := int64(65)
	cases := []struct {
		name    string
		column  spec.Column
		pk      *spec.Index
		mode    string
		alterPK *bool
	}{
		{"non_bigint", spec.Column{Name: "id", Type: "int", NotNull: true, AutoRandom: true}, t23ClusteredPK("id"), "", nil},
		{"with_auto_increment", spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoIncrement: true, AutoRandom: true}, t23ClusteredPK("id"), "", nil},
		{"with_default", spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true, HasDefault: true, DefaultValue: "5"}, t23ClusteredPK("id"), "", nil},
		{"shard_zero", spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true, AutoRandomShardBits: &shard0}, t23ClusteredPK("id"), "", nil},
		{"shard_over_max", spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true, AutoRandomShardBits: &shard16}, t23ClusteredPK("id"), "", nil},
		{"range_under_min", spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true, AutoRandomRangeBits: &range31}, t23ClusteredPK("id"), "", nil},
		{"range_over_max", spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true, AutoRandomRangeBits: &range65}, t23ClusteredPK("id"), "", nil},
		// Signed BIGINT AUTO_RANDOM(15,41): 41-15-1 = 25 incremental bits
		// < 27 with otherwise legal authored bounds.
		{"incremental_under_min", spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true, AutoRandomShardBits: &shard15, AutoRandomRangeBits: &range41}, t23ClusteredPK("id"), "", nil},
		{"no_primary_key", spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true}, nil, "", nil},
		{"nonclustered_pk", spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true},
			&spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}, PrimaryKeyType: "nonclustered"}, "", nil},
		{"not_first_pk_member", spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true}, t23ClusteredPK("a", "id"), "", nil},
		{"mode_off_unmarked", spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true},
			&spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}, "off", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			statement := t23AutoRandomStatement(tc.column, tc.pk, tc.mode, tc.alterPK != nil, tc.alterPK != nil && *tc.alterPK)
			findings, err := r.Evaluate(context.Background(), statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 || findings[0].Level != rule.LevelBlocker {
				t.Fatalf("expected one auto_random.valid blocker, got %+v", findings)
			}
		})
	}
}

// TestT23AutoRandomUnknownModeGap proves an unobserved clustered-index mode
// withholds the answer as a gap instead of a guessed pass or refusal.
func TestT23AutoRandomUnknownModeGap(t *testing.T) {
	t.Parallel()
	r := newT23AutoRandomRule(t)
	statement := t23AutoRandomStatement(
		spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true},
		&spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
		"", false, false)
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("unknown mode must not fabricate a blocker, got %+v", findings)
	}
	gaps := r.(interface {
		EvidenceGaps(spec.Statement) []rule.EvidenceGap
	}).EvidenceGaps(statement)
	if len(gaps) != 1 || gaps[0].ReasonCode != gapReasonMissingInstanceFact {
		t.Fatalf("expected one missing_instance_fact gap, got %+v", gaps)
	}
	if len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != autoRandomFactClusteredMode {
		t.Fatalf("unobserved mode must owe only the clustered-mode fact, got %+v", gaps[0].RequiredFacts)
	}

	// An observed int_only mode narrows the pending premise to the
	// unobserved alter-primary-key fact alone.
	intOnly := t23AutoRandomStatement(
		spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true},
		&spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
		"int_only", false, false)
	gaps = r.(interface {
		EvidenceGaps(spec.Statement) []rule.EvidenceGap
	}).EvidenceGaps(intOnly)
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != autoRandomFactAlterPrimaryKey {
		t.Fatalf("int_only must owe only the alter-primary-key fact, got %+v", gaps)
	}
}

// TestT23AutoRandomModeOnSingleIntPK proves an unmarked single-int PK under
// observed mode "on" is clustered and passes.
func TestT23AutoRandomModeOnSingleIntPK(t *testing.T) {
	t.Parallel()
	r := newT23AutoRandomRule(t)
	statement := t23AutoRandomStatement(
		spec.Column{Name: "id", Type: "bigint", NotNull: true, AutoRandom: true},
		&spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
		"on", true, true)
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings under mode on, got %+v", findings)
	}
}

// TestT23AutoRandomUnsignedEdge proves the unsigned sign-bit arithmetic:
// UNSIGNED AUTO_RANDOM(15,42) leaves 27 incremental bits (allowed), while
// (15,41) leaves 26 (refused).
func TestT23AutoRandomUnsignedEdge(t *testing.T) {
	t.Parallel()
	r := newT23AutoRandomRule(t)
	shard15 := int64(15)
	range41, range42 := int64(41), int64(42)
	ok := t23AutoRandomStatement(
		spec.Column{Name: "id", Type: "bigint", NotNull: true, Unsigned: true, AutoRandom: true, AutoRandomShardBits: &shard15, AutoRandomRangeBits: &range42},
		t23ClusteredPK("id"), "", false, false)
	if findings, err := r.Evaluate(context.Background(), ok); err != nil || len(findings) != 0 {
		t.Fatalf("unsigned (15,42) must pass, findings=%+v err=%v", findings, err)
	}
	bad := t23AutoRandomStatement(
		spec.Column{Name: "id", Type: "bigint", NotNull: true, Unsigned: true, AutoRandom: true, AutoRandomShardBits: &shard15, AutoRandomRangeBits: &range41},
		t23ClusteredPK("id"), "", false, false)
	if findings, err := r.Evaluate(context.Background(), bad); err != nil || len(findings) != 1 {
		t.Fatalf("unsigned (15,41) must refuse, findings=%+v err=%v", findings, err)
	}
}

// TestT23AutoRandomMySQLSilent proves the rule never fires under MySQL —
// there the attribute is a vendor boundary, not a validity question.
func TestT23AutoRandomMySQLSilent(t *testing.T) {
	t.Parallel()
	r := newT23AutoRandomRule(t)
	statement := t23AutoRandomStatement(
		spec.Column{Name: "id", Type: "int", NotNull: true, AutoRandom: true},
		nil, "", false, false)
	statement.Dialect = spec.DialectMySQL
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("mysql dialect must stay silent, got %+v", findings)
	}
}

// TestT23AutoIncrementRequireAcceptsAutoRandom proves the auto-allocation
// policy intent accepts TiDB's native AUTO_RANDOM allocator only when the
// policy explicitly allows it — unset params keep the strict literal
// AUTO_INCREMENT requirement.
func TestT23AutoIncrementRequireAcceptsAutoRandom(t *testing.T) {
	t.Parallel()
	statement := primaryKeyStatement(spec.Column{Name: "id", Type: "bigint", NotNull: true, Unsigned: true, AutoRandom: true})
	statement.Dialect = spec.DialectTiDB
	allow, err := newSinglePrimaryKeyColumnRule(ruleIDPrimaryKeyAutoIncrementRequire, rule.LevelBlocker, "must use auto_increment", "add AUTO_INCREMENT to the primary key column", func(column spec.Column) bool {
		return column.AutoIncrement
	}, policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": true, "accept_tidb_auto_random": true}})
	if err != nil {
		t.Fatalf("new allow rule: %v", err)
	}
	findings, err := allow.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate allow: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("allowed AUTO_RANDOM must satisfy auto_increment.require, got %+v", findings)
	}
	strict, err := newSinglePrimaryKeyColumnRule(ruleIDPrimaryKeyAutoIncrementRequire, rule.LevelBlocker, "must use auto_increment", "add AUTO_INCREMENT to the primary key column", func(column spec.Column) bool {
		return column.AutoIncrement
	}, policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": true}})
	if err != nil {
		t.Fatalf("new strict rule: %v", err)
	}
	findings, err = strict.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate strict: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("strict policy must flag the missing literal AUTO_INCREMENT, got %+v", findings)
	}
	if _, err = allow.Evaluate(context.Background(), statement); err != nil {
		t.Fatalf("evaluate allow mysql: %v", err)
	}
	statement.Dialect = spec.DialectMySQL
	findings, err = allow.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate mysql: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("MySQL AUTO_RANDOM declaration must not satisfy the literal policy, got %+v", findings)
	}
}
