// Package ddl verifies the T13 destructive/data-moving partition action
// rules: DROP/TRUNCATE PARTITION forbid at blocker level and the EXCHANGE
// PARTITION data-move warning.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t13PartitionStatement(dialect spec.Dialect, alter spec.Alter) spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: dialect,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "t"},
			Alter:     []spec.Alter{alter},
		},
	}
}

func t13ForbidConfig() policy.RulePolicy {
	return policy.RulePolicy{Enabled: true, Params: map[string]any{"forbid": true}}
}

// DROP PARTITION forbid fires one blocker per named partition in clause
// order; the action itself is the proven fact — no metadata is required.
func TestDropPartitionForbidFiresPerPartition(t *testing.T) {
	t.Parallel()
	r, err := newDropPartitionForbidRule(t13ForbidConfig())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := t13PartitionStatement(spec.DialectMySQL, spec.Alter{
		Action: "drop_partition", Name: "p0", Partitions: []string{"p0", "p1"},
	})
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected one finding per named partition, got %d: %#v", len(findings), findings)
	}
	for i, partition := range []string{"p0", "p1"} {
		f := findings[i]
		if f.Level != rule.LevelBlocker {
			t.Fatalf("finding %d level = %s, want default blocker", i, f.Level)
		}
		if f.Metadata["table"] != "t" || f.Metadata["partition"] != partition {
			t.Fatalf("finding %d metadata = %#v, want table=t partition=%s", i, f.Metadata, partition)
		}
		if f.Metadata["action"] != "drop_partition" {
			t.Fatalf("finding %d action metadata = %#v", i, f.Metadata)
		}
	}
}

// TRUNCATE PARTITION forbids each named partition; the ALL form emits one
// finding carrying all_partitions and never fabricates a name.
func TestTruncatePartitionForbidForms(t *testing.T) {
	t.Parallel()
	r, err := newTruncatePartitionForbidRule(t13ForbidConfig())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	named := t13PartitionStatement(spec.DialectTiDB, spec.Alter{
		Action: "truncate_partition", Name: "p0", Partitions: []string{"p0"},
	})
	findings, err := r.Evaluate(context.Background(), named)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["partition"] != "p0" {
		t.Fatalf("named truncate = %#v, want one p0 finding", findings)
	}
	all := t13PartitionStatement(spec.DialectTiDB, spec.Alter{
		Action: "truncate_partition", AllPartitions: true,
	})
	findings, err = r.Evaluate(context.Background(), all)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["all_partitions"] != "true" {
		t.Fatalf("ALL truncate = %#v, want one all_partitions finding", findings)
	}
	if _, ok := findings[0].Metadata["partition"]; ok {
		t.Fatalf("ALL truncate must not fabricate a partition name: %#v", findings[0].Metadata)
	}
}

// EXCHANGE PARTITION warns with both object identities and the effective
// validation clause — the warning states the data-move risk; it never
// claims validated compatibility.
func TestExchangePartitionWarnCarriesBothSides(t *testing.T) {
	t.Parallel()
	r, err := newExchangePartitionWarnRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := t13PartitionStatement(spec.DialectMySQL, spec.Alter{
		Action:             "exchange_partition",
		Name:               "p0",
		Partitions:         []string{"p0"},
		ExchangeTable:      &spec.Table{Schema: "s2", Name: "t2"},
		ExchangeValidation: "without",
	})
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one warning, got %d: %#v", len(findings), findings)
	}
	f := findings[0]
	if f.Level != rule.LevelWarning {
		t.Fatalf("exchange warn default level = %s, want warning", f.Level)
	}
	if f.Metadata["table"] != "t" || f.Metadata["partition"] != "p0" ||
		f.Metadata["exchange_table"] != "t2" || f.Metadata["exchange_schema"] != "s2" ||
		f.Metadata["validation"] != "without" {
		t.Fatalf("exchange metadata = %#v", f.Metadata)
	}
}

// forbid=false disarms the forbid rules; an unrelated action never matches.
func TestT13ForbidDisarmAndNonMatchingAction(t *testing.T) {
	t.Parallel()
	drop, err := newDropPartitionForbidRule(policy.RulePolicy{Enabled: true, Params: map[string]any{"forbid": false}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := t13PartitionStatement(spec.DialectMySQL, spec.Alter{
		Action: "drop_partition", Name: "p0", Partitions: []string{"p0"},
	})
	findings, err := drop.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("forbid=false must disarm, got %#v", findings)
	}
	enabled, err := newDropPartitionForbidRule(t13ForbidConfig())
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	other := t13PartitionStatement(spec.DialectMySQL, spec.Alter{Action: "drop_column", Name: "c"})
	findings, err = enabled.Evaluate(context.Background(), other)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("non-partition action must not fire, got %#v", findings)
	}
}

// Configured levels override the defaults exactly like every other rule.
func TestT13ConfiguredLevelOverride(t *testing.T) {
	t.Parallel()
	r, err := newDropPartitionForbidRule(policy.RulePolicy{Enabled: true, Level: rule.LevelNotice, Params: map[string]any{"forbid": true}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := t13PartitionStatement(spec.DialectMySQL, spec.Alter{
		Action: "drop_partition", Name: "p0", Partitions: []string{"p0"},
	})
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Level != rule.LevelNotice {
		t.Fatalf("configured level must apply, got %#v", findings)
	}
}
