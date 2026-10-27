// Package ddl verifies the T13-R1 partition applicability rules: proven
// inapplicability blocks one finding per ALTER carrying the native refusal
// identity, while missing consumed facts stay evidence gaps — never
// findings — and proven-applicable forms stay silent.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t13r1Config() policy.RulePolicy {
	return policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker}
}

func t13r1Bool(v bool) *bool { return &v }

// t13r1Snapshot is a partitioned primary table whose member facts are all
// vouched: RANGE type, two named partitions, ordinary member structure, and
// an observed engine label — the MySQL exchange compare needs it.
func t13r1Snapshot(name string) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists:         true,
		Table:          &spec.Table{Name: name},
		Columns:        []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
		PrimaryKey:     &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
		Partitioned:    t13r1Bool(true),
		PartitionType:  t13r1Str("range"),
		PartitionNames: []string{"p0", "p1"},
		Options:        map[string]string{"engine": "innodb"},
	}
}

func t13r1Str(v string) *string { return &v }

func t13r1Statement(action string, alter spec.Alter, primary *spec.TableSnapshot) spec.Statement {
	stmt := t13PartitionStatement(spec.DialectMySQL, alter)
	if primary != nil {
		stmt.Metadata = &spec.Metadata{TargetTable: primary}
	}
	return stmt
}

func t13r1StatementExchange(alter spec.Alter, primary, counterpart *spec.TableSnapshot) spec.Statement {
	stmt := t13r1Statement("exchange_partition", alter, primary)
	if counterpart != nil {
		stmt.Metadata.ExchangeTable = counterpart
	}
	return stmt
}

func t13r1Facts(gap rule.EvidenceGap) []string { return gap.RequiredFacts }

// t13r1Gaps answers the optional evidence-gap channel: rules built
// without the reporter contribute nothing.
func t13r1Gaps(r rule.StatementRule, stmt spec.Statement) []rule.EvidenceGap {
	reporter, ok := r.(rule.EvidenceReporter)
	if !ok {
		return nil
	}
	return reporter.EvidenceGaps(stmt)
}

// DROP PARTITION on a known RANGE member is proven applicable — silent on
// findings and gaps alike.
func TestT13R1DropApplicabilitySilence(t *testing.T) {
	t.Parallel()
	r, err := newDropPartitionApplicabilityRule(t13r1Config())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := t13r1Statement("drop_partition", spec.Alter{
		Action: "drop_partition", Partitions: []string{"p0"},
	}, t13r1Snapshot("t"))
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("known RANGE member drop must not fire, got %#v", findings)
	}
	gaps := t13r1Gaps(r, stmt)
	if len(gaps) != 0 {
		t.Fatalf("fully vouched facts owe no gap, got %+v", gaps)
	}
}

// A proven HASH/KEY partition type rejects DROP PARTITION natively (1512).
func TestT13R1DropHashTypeBlocked(t *testing.T) {
	t.Parallel()
	r, err := newDropPartitionApplicabilityRule(t13r1Config())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	shape := t13r1Snapshot("t")
	shape.PartitionType = t13r1Str("hash")
	stmt := t13r1Statement("drop_partition", spec.Alter{
		Action: "drop_partition", Partitions: []string{"p0"},
	}, shape)
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("hash drop must emit exactly one blocker, got %#v", findings)
	}
	f := findings[0]
	if f.Level != rule.LevelBlocker || f.Metadata["reason_code"] != "partition_type_inapplicable" ||
		f.Metadata["native_error"] != "1512" || f.Metadata["action"] != "drop_partition" ||
		f.Metadata["table"] != "t" || f.Metadata["partition"] != "p0" {
		t.Fatalf("hash drop finding = %#v", f)
	}
	if gaps := t13r1Gaps(r, stmt); len(gaps) != 0 {
		t.Fatalf("proven inapplicability owes no gap, got %+v", gaps)
	}
}

// A proven non-partitioned primary rejects partition management (1505).
func TestT13R1DropNonPartitionedBlocked(t *testing.T) {
	t.Parallel()
	r, err := newDropPartitionApplicabilityRule(t13r1Config())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	shape := t13r1Snapshot("t")
	shape.Partitioned = t13r1Bool(false)
	shape.PartitionType = nil
	shape.PartitionNames = nil
	stmt := t13r1Statement("drop_partition", spec.Alter{
		Action: "drop_partition", Partitions: []string{"p0"},
	}, shape)
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["reason_code"] != "not_partitioned" ||
		findings[0].Metadata["native_error"] != "1505" {
		t.Fatalf("non-partitioned drop must block once with not_partitioned/1505, got %#v", findings)
	}
}

// Partitioned truth is the consumed premise: an unobserved snapshot owes
// exactly the partitioned fact — never an absence finding.
func TestT13R1DropUnknownPartitionedReportsGap(t *testing.T) {
	t.Parallel()
	r, err := newDropPartitionApplicabilityRule(t13r1Config())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	for _, tc := range []struct {
		name     string
		shape    *spec.TableSnapshot
		attach   bool
		wantFact string
	}{
		{name: "no snapshot", shape: nil, attach: false, wantFact: "target_table.partitioned"},
		{name: "partitioned unobserved", shape: func() *spec.TableSnapshot {
			s := t13r1Snapshot("t")
			s.Partitioned = nil
			s.PartitionType = nil
			return s
		}(), attach: true, wantFact: "target_table.partitioned"},
		{name: "type unobserved", shape: func() *spec.TableSnapshot {
			s := t13r1Snapshot("t")
			s.PartitionType = nil
			return s
		}(), attach: true, wantFact: "target_table.partition_type"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stmt := t13r1Statement("drop_partition", spec.Alter{
				Action: "drop_partition", Partitions: []string{"p0"},
			}, nil)
			if tc.attach {
				stmt.Metadata = &spec.Metadata{TargetTable: tc.shape}
			}
			findings, err := r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("%s must not fabricate a refusal finding, got %#v", tc.name, findings)
			}
			gaps := t13r1Gaps(r, stmt)
			if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
				t.Fatalf("%s owes exactly one unknown_table_state gap, got %+v", tc.name, gaps)
			}
			found := false
			for _, fact := range t13r1Facts(gaps[0]) {
				found = found || fact == tc.wantFact
			}
			if !found {
				t.Fatalf("%s gap must name %s, got %+v", tc.name, tc.wantFact, gaps[0].RequiredFacts)
			}
		})
	}
}

// DROP/TRUNCATE are the ONLY type-gated actions: TRUNCATE and EXCHANGE
// execute on HASH natively — their applicability rules stay silent.
func TestT13R1TruncateAndExchangeIgnoreHashType(t *testing.T) {
	t.Parallel()
	shape := t13r1Snapshot("t")
	shape.PartitionType = t13r1Str("hash")
	truncateRule, err := newTruncatePartitionApplicabilityRule(t13r1Config())
	if err != nil {
		t.Fatalf("truncate rule: %v", err)
	}
	stmt := t13r1Statement("truncate_partition", spec.Alter{
		Action: "truncate_partition", Partitions: []string{"p0"},
	}, shape)
	findings, err := truncateRule.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("hash TRUNCATE executes natively — no finding, got %#v", findings)
	}
	exchangeRule, err := newExchangePartitionApplicabilityRule(t13r1Config())
	if err != nil {
		t.Fatalf("exchange rule: %v", err)
	}
	counterpart := t13r1Snapshot("t2")
	counterpart.Partitioned = t13r1Bool(false)
	counterpart.PartitionType = nil
	counterpart.PartitionNames = nil
	xstmt := t13r1StatementExchange(spec.Alter{
		Action: "exchange_partition", Partitions: []string{"p0"},
		ExchangeTable: &spec.Table{Name: "t2"},
	}, shape, counterpart)
	findings, err = exchangeRule.Evaluate(context.Background(), xstmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("hash EXCHANGE executes natively — no finding, got %#v", findings)
	}
}

// Named-partition truth: a member proven absent blocks natively —
// DROP→1507, TRUNCATE/EXCHANGE→1735.
func TestT13R1TargetAbsentFindings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		action      string
		nativeError string
	}{
		{"drop", "drop_partition", "1507"},
		{"truncate", "truncate_partition", "1735"},
		{"exchange", "exchange_partition", "1735"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := newPartitionTargetExistsRule(tc.action, t13r1Config())
			if err != nil {
				t.Fatalf("new rule: %v", err)
			}
			stmt := t13r1Statement(tc.action, spec.Alter{
				Action: tc.action, Partitions: []string{"p99"},
			}, t13r1Snapshot("t"))
			findings, err := r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("absent member must block once, got %#v", findings)
			}
			f := findings[0]
			if f.Metadata["reason_code"] != "partition_absent" || f.Metadata["native_error"] != tc.nativeError ||
				f.Metadata["partition"] != "p99" || f.Metadata["action"] != tc.action {
				t.Fatalf("absent-member finding = %#v", f)
			}
			if gaps := t13r1Gaps(r, stmt); len(gaps) != 0 {
				t.Fatalf("proven absence owes no gap, got %+v", gaps)
			}
		})
	}
}

// An unobserved member list owes the partitions fact — never an absence
// finding; TRUNCATE PARTITION ALL consumes no member name.
func TestT13R1TargetMemberUnobservedGapAndAll(t *testing.T) {
	t.Parallel()
	r, err := newPartitionTargetExistsRule("drop_partition", t13r1Config())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	shape := t13r1Snapshot("t")
	shape.PartitionNames = nil
	stmt := t13r1Statement("drop_partition", spec.Alter{
		Action: "drop_partition", Partitions: []string{"p0"},
	}, shape)
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("unobserved members must not fabricate absence, got %#v", findings)
	}
	gaps := t13r1Gaps(r, stmt)
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.partitions" {
		t.Fatalf("unobserved member list owes target_table.partitions, got %+v", gaps)
	}
	truncR, err := newPartitionTargetExistsRule("truncate_partition", t13r1Config())
	if err != nil {
		t.Fatalf("truncate rule: %v", err)
	}
	allStmt := t13r1Statement("truncate_partition", spec.Alter{
		Action: "truncate_partition", AllPartitions: true,
	}, shape)
	allFindings, err := truncR.Evaluate(context.Background(), allStmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(allFindings) != 0 || len(t13r1Gaps(truncR, allStmt)) != 0 {
		t.Fatalf("TRUNCATE ALL consumes no member name — no finding, no gap; got findings=%#v gaps=%+v",
			allFindings, t13r1Gaps(truncR, allStmt))
	}
}

// A missing counterpart table rejects the EXCHANGE natively (1146).
func TestT13R1CounterpartAbsentBlocked(t *testing.T) {
	t.Parallel()
	r, err := newExchangeCounterpartCompatibilityRule(t13r1Config())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := t13r1StatementExchange(spec.Alter{
		Action: "exchange_partition", Partitions: []string{"p0"},
		ExchangeTable: &spec.Table{Name: "t2"},
	}, t13r1Snapshot("t"), &spec.TableSnapshot{Exists: false, Table: &spec.Table{Name: "t2"}})
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["reason_code"] != "counterpart_absent" ||
		findings[0].Metadata["native_error"] != "1146" || findings[0].Metadata["exchange_table"] != "t2" {
		t.Fatalf("absent counterpart must block once, got %#v", findings)
	}
}

// A partitioned counterpart is a proven refusal (1732); a column-sequence
// or primary-key mismatch is a proven refusal (1736); compatible structure
// stays silent without claiming validation passed.
func TestT13R1CounterpartShapeChecks(t *testing.T) {
	t.Parallel()
	r, err := newExchangeCounterpartCompatibilityRule(t13r1Config())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	t.Run("partitioned counterpart", func(t *testing.T) {
		t.Parallel()
		stmt := t13r1StatementExchange(spec.Alter{
			Action: "exchange_partition", Partitions: []string{"p0"},
			ExchangeTable: &spec.Table{Name: "t2"},
		}, t13r1Snapshot("t"), t13r1Snapshot("t2"))
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 1 || findings[0].Metadata["reason_code"] != "counterpart_partitioned" ||
			findings[0].Metadata["native_error"] != "1732" {
			t.Fatalf("partitioned counterpart must block once, got %#v", findings)
		}
	})
	t.Run("column mismatch", func(t *testing.T) {
		t.Parallel()
		counterpart := &spec.TableSnapshot{
			Exists:      true,
			Table:       &spec.Table{Name: "t2"},
			Columns:     []spec.Column{{Name: "id", Type: "int"}, {Name: "extra", Type: "int"}},
			PrimaryKey:  &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
			Partitioned: t13r1Bool(false),
		}
		stmt := t13r1StatementExchange(spec.Alter{
			Action: "exchange_partition", Partitions: []string{"p0"},
			ExchangeTable: &spec.Table{Name: "t2"},
		}, t13r1Snapshot("t"), counterpart)
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 1 || findings[0].Metadata["reason_code"] != "counterpart_definition_mismatch" ||
			findings[0].Metadata["native_error"] != "1736" {
			t.Fatalf("column mismatch must block once, got %#v", findings)
		}
	})
	t.Run("primary key mismatch", func(t *testing.T) {
		t.Parallel()
		counterpart := &spec.TableSnapshot{
			Exists:      true,
			Table:       &spec.Table{Name: "t2"},
			Columns:     []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			PrimaryKey:  nil,
			Partitioned: t13r1Bool(false),
		}
		stmt := t13r1StatementExchange(spec.Alter{
			Action: "exchange_partition", Partitions: []string{"p0"},
			ExchangeTable: &spec.Table{Name: "t2"},
		}, t13r1Snapshot("t"), counterpart)
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 1 || findings[0].Metadata["reason_code"] != "counterpart_definition_mismatch" ||
			findings[0].Metadata["native_error"] != "1736" {
			t.Fatalf("PK mismatch must block once, got %#v", findings)
		}
	})
	t.Run("compatible stays silent", func(t *testing.T) {
		t.Parallel()
		counterpart := &spec.TableSnapshot{
			Exists:      true,
			Table:       &spec.Table{Name: "t2"},
			Columns:     []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			PrimaryKey:  &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
			Partitioned: t13r1Bool(false),
			Options:     map[string]string{"engine": "innodb"},
		}
		stmt := t13r1StatementExchange(spec.Alter{
			Action: "exchange_partition", Partitions: []string{"p0"},
			ExchangeTable: &spec.Table{Name: "t2"},
		}, t13r1Snapshot("t"), counterpart)
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("vouched-compatible counterpart stays silent, got %#v", findings)
		}
		if gaps := t13r1Gaps(r, stmt); len(gaps) != 0 {
			t.Fatalf("vouched-compatible counterpart owes no gap, got %+v", gaps)
		}
	})
}

// Counterpart facts the rule actually consumes report bounded gaps keyed to
// the exchange identity — never findings.
func TestT13R1CounterpartFactGaps(t *testing.T) {
	t.Parallel()
	r, err := newExchangeCounterpartCompatibilityRule(t13r1Config())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	for _, tc := range []struct {
		name        string
		counterpart *spec.TableSnapshot
		wantFact    string
	}{
		{name: "no counterpart projection", counterpart: nil, wantFact: "exchange_table.existence"},
		{name: "partitioned unobserved", counterpart: func() *spec.TableSnapshot {
			return &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: "t2"},
				Columns:    []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
				PrimaryKey: &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}}
		}(), wantFact: "exchange_table.partitioned"},
		{name: "columns unobserved", counterpart: func() *spec.TableSnapshot {
			return &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: "t2"},
				Partitioned: t13r1Bool(false),
				PrimaryKey:  &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}}}
		}(), wantFact: "exchange_table.columns"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stmt := t13r1StatementExchange(spec.Alter{
				Action: "exchange_partition", Partitions: []string{"p0"},
				ExchangeTable: &spec.Table{Name: "t2"},
			}, t13r1Snapshot("t"), tc.counterpart)
			findings, err := r.Evaluate(context.Background(), stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("%s must not fabricate a refusal, got %#v", tc.name, findings)
			}
			gaps := t13r1Gaps(r, stmt)
			if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
				t.Fatalf("%s owes one unknown_table_state gap, got %+v", tc.name, gaps)
			}
			found := false
			for _, fact := range gaps[0].RequiredFacts {
				found = found || fact == tc.wantFact
			}
			if !found {
				t.Fatalf("%s must name %s, got %+v", tc.name, tc.wantFact, gaps[0].RequiredFacts)
			}
		})
	}
}

// Disabled applicability rules emit neither findings nor gaps.
func TestT13R1DisabledRulesStaySilent(t *testing.T) {
	t.Parallel()
	cfg := policy.RulePolicy{Enabled: false}
	rules := []rule.StatementRule{}
	for _, make := range []func() (rule.StatementRule, error){
		func() (rule.StatementRule, error) { return newDropPartitionApplicabilityRule(cfg) },
		func() (rule.StatementRule, error) { return newTruncatePartitionApplicabilityRule(cfg) },
		func() (rule.StatementRule, error) { return newExchangePartitionApplicabilityRule(cfg) },
		func() (rule.StatementRule, error) { return newPartitionTargetExistsRule("drop_partition", cfg) },
		func() (rule.StatementRule, error) { return newPartitionTargetExistsRule("truncate_partition", cfg) },
		func() (rule.StatementRule, error) { return newPartitionTargetExistsRule("exchange_partition", cfg) },
		func() (rule.StatementRule, error) { return newExchangeCounterpartCompatibilityRule(cfg) },
	} {
		built, err := make()
		if err != nil {
			t.Fatalf("disabled rule construction must succeed, got %v", err)
		}
		rules = append(rules, built)
	}
	shape := t13r1Snapshot("t")
	shape.PartitionType = t13r1Str("hash")
	shape.PartitionNames = nil
	stmt := t13r1Statement("drop_partition", spec.Alter{
		Action: "drop_partition", Partitions: []string{"p0"},
	}, shape)
	for _, r := range rules {
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("disabled evaluate: %v", err)
		}
		if len(findings) != 0 || len(t13r1Gaps(r, stmt)) != 0 {
			t.Fatalf("disabled %s must emit nothing, got findings=%#v gaps=%+v",
				r.ID(), findings, t13r1Gaps(r, stmt))
		}
	}
}
