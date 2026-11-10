// Package ddl verifies the T25 whole-table InnoDB tablespace rules:
// DISCARD TABLESPACE forbids at blocker level on the authored action,
// IMPORT TABLESPACE warns on the unattested external file, and the shared
// applicability rule emits proven native refusals on a non-InnoDB or
// partitioned target while owing unobserved engine/partitioned facts
// through evidence gaps. TiDB suppresses every rule at the vendor boundary.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t25Statement(dialect spec.Dialect, alter spec.Alter, snapshot *spec.TableSnapshot) spec.Statement {
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: dialect,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "t"},
			Alter:     []spec.Alter{alter},
		},
	}
	if snapshot != nil {
		stmt.Metadata = &spec.Metadata{TargetTable: snapshot}
	}
	return stmt
}

func t25Shape(engine string, partitioned *bool) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists:      true,
		Table:       &spec.Table{Name: "t"},
		Partitioned: partitioned,
		Options:     map[string]string{"engine": engine},
	}
}

func t25Bool(value bool) *bool { return &value }

// DISCARD TABLESPACE forbids on the authored action alone — a complete
// premise (proven InnoDB, unpartitioned) keeps the single policy blocker.
func TestDiscardTablespaceForbid(t *testing.T) {
	t.Parallel()
	r, err := newDiscardTablespaceForbidRule(policy.RulePolicy{Enabled: true, Params: map[string]any{"forbid": true}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := t25Statement(spec.DialectMySQL,
		spec.Alter{Action: "discard_tablespace"},
		t25Shape("innodb", t25Bool(false)))
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly one blocker, got %d: %#v", len(findings), findings)
	}
	if findings[0].Level != rule.LevelBlocker {
		t.Fatalf("level = %s, want blocker", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "discard_tablespace" || findings[0].Metadata["table"] != "t" {
		t.Fatalf("metadata = %#v, want action/table", findings[0].Metadata)
	}
}

// IMPORT TABLESPACE never fires the discard forbid, and its own warning is
// the only finding the action emits.
func TestImportTablespaceWarnOnly(t *testing.T) {
	t.Parallel()
	discard, err := newDiscardTablespaceForbidRule(policy.RulePolicy{Enabled: true, Params: map[string]any{"forbid": true}})
	if err != nil {
		t.Fatalf("new discard rule: %v", err)
	}
	warn, err := newImportTablespaceWarnRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new warn rule: %v", err)
	}
	stmt := t25Statement(spec.DialectMySQL,
		spec.Alter{Action: "import_tablespace"},
		t25Shape("innodb", t25Bool(false)))
	if findings, _ := discard.Evaluate(context.Background(), stmt); len(findings) != 0 {
		t.Fatalf("import must not fire the discard forbid, got %#v", findings)
	}
	findings, err := warn.Evaluate(context.Background(), stmt)
	if err != nil || len(findings) != 1 || findings[0].Level != rule.LevelWarning {
		t.Fatalf("import warn = %#v, err %v, want one warning", findings, err)
	}
}

// An ordinary ALTER action never trips the tablespace rules.
func TestTablespaceRulesIgnoreOrdinaryAlter(t *testing.T) {
	t.Parallel()
	for _, ctor := range []func(policy.RulePolicy) (rule.StatementRule, error){
		newDiscardTablespaceForbidRule, newImportTablespaceWarnRule,
		newDiscardTablespaceApplicabilityRule, newImportTablespaceApplicabilityRule,
	} {
		r, err := ctor(policy.RulePolicy{Enabled: true, Params: map[string]any{"forbid": true}})
		if err != nil {
			t.Fatalf("new rule: %v", err)
		}
		stmt := t25Statement(spec.DialectMySQL,
			spec.Alter{Action: "add_columns"},
			t25Shape("innodb", t25Bool(false)))
		if findings, _ := r.Evaluate(context.Background(), stmt); len(findings) != 0 {
			t.Fatalf("rule %s fired on ordinary alter: %#v", r.ID(), findings)
		}
	}
}

// The InnoDB-only contract: a proven MyISAM target refuses natively —
// distinct from the policy forbid, which fires on the authored form alone.
func TestTablespaceApplicabilityEngineRefusal(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"discard_tablespace", "import_tablespace"} {
		var ctor func(policy.RulePolicy) (rule.StatementRule, error)
		if action == "discard_tablespace" {
			ctor = newDiscardTablespaceApplicabilityRule
		} else {
			ctor = newImportTablespaceApplicabilityRule
		}
		r, err := ctor(policy.RulePolicy{Enabled: true})
		if err != nil {
			t.Fatalf("new rule: %v", err)
		}
		stmt := t25Statement(spec.DialectMySQL,
			spec.Alter{Action: action},
			t25Shape("myisam", t25Bool(false)))
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil || len(findings) != 1 {
			t.Fatalf("%s on MyISAM = %#v, err %v, want one refusal", action, findings, err)
		}
		if findings[0].Level != rule.LevelBlocker ||
			findings[0].Metadata["reason_code"] != "tablespace_engine_inapplicable" ||
			findings[0].Metadata["engine"] != "myisam" {
			t.Fatalf("%s refusal metadata = %#v, want engine_inapplicable/myisam", action, findings[0].Metadata)
		}
	}
}

// A proven InnoDB unpartitioned target produces no refusal finding — the
// applicability gate passes.
func TestTablespaceApplicabilityInnoDBPasses(t *testing.T) {
	t.Parallel()
	r, err := newDiscardTablespaceApplicabilityRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := t25Statement(spec.DialectMySQL,
		spec.Alter{Action: "discard_tablespace"},
		t25Shape("innodb", t25Bool(false)))
	if findings, _ := r.Evaluate(context.Background(), stmt); len(findings) != 0 {
		t.Fatalf("InnoDB target must pass applicability, got %#v", findings)
	}
	if reporter, ok := r.(rule.EvidenceReporter); ok {
		if gaps := reporter.EvidenceGaps(stmt); len(gaps) != 0 {
			t.Fatalf("complete premise must owe no gaps, got %#v", gaps)
		}
	}
}

// An unproven engine owes the fact through EvidenceGaps — never a
// fabricated refusal and never a fabricated pass.
func TestTablespaceApplicabilityEngineGap(t *testing.T) {
	t.Parallel()
	r, err := newDiscardTablespaceApplicabilityRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	snapshot := t25Shape("", t25Bool(false))
	delete(snapshot.Options, "engine")
	stmt := t25Statement(spec.DialectMySQL,
		spec.Alter{Action: "discard_tablespace"}, snapshot)
	if findings, _ := r.Evaluate(context.Background(), stmt); len(findings) != 0 {
		t.Fatalf("unproven engine must not refuse, got %#v", findings)
	}
	reporter, ok := r.(rule.EvidenceReporter)
	if !ok {
		t.Fatalf("applicability rule must implement EvidenceReporter")
	}
	gaps := reporter.EvidenceGaps(stmt)
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.engine" {
		t.Fatalf("gaps = %#v, want target_table.engine", gaps)
	}
}

// A proven partitioned target refuses the whole-table form; an unproven
// partitioned state owes the fact instead of guessing.
func TestTablespaceApplicabilityPartitioned(t *testing.T) {
	t.Parallel()
	r, err := newDiscardTablespaceApplicabilityRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := t25Statement(spec.DialectMySQL,
		spec.Alter{Action: "discard_tablespace"},
		t25Shape("innodb", t25Bool(true)))
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil || len(findings) != 1 ||
		findings[0].Metadata["reason_code"] != "tablespace_partitioned_target" {
		t.Fatalf("partitioned target = %#v, err %v, want one partitioned refusal", findings, err)
	}
	unknown := t25Statement(spec.DialectMySQL,
		spec.Alter{Action: "discard_tablespace"},
		t25Shape("innodb", nil))
	reporter, ok := r.(rule.EvidenceReporter)
	if !ok {
		t.Fatalf("wrapped applicability rule must forward EvidenceReporter")
	}
	gaps := reporter.EvidenceGaps(unknown)
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.partitioned" {
		t.Fatalf("unproven partitioned gaps = %#v, want target_table.partitioned", gaps)
	}
}

// TiDB refuses the whole-table forms at the vendor boundary: every rule
// suppresses under the TiDB dialect — the vendor aspect answers instead.
func TestTablespaceRulesTiDBVendorGate(t *testing.T) {
	t.Parallel()
	for _, ctor := range []func(policy.RulePolicy) (rule.StatementRule, error){
		newDiscardTablespaceForbidRule, newImportTablespaceWarnRule,
		newDiscardTablespaceApplicabilityRule, newImportTablespaceApplicabilityRule,
	} {
		r, err := ctor(policy.RulePolicy{Enabled: true, Params: map[string]any{"forbid": true}})
		if err != nil {
			t.Fatalf("new rule: %v", err)
		}
		stmt := t25Statement(spec.DialectTiDB,
			spec.Alter{Action: "discard_tablespace"},
			t25Shape("innodb", t25Bool(false)))
		if r.AppliesTo(stmt) {
			t.Fatalf("rule %s must not apply on TiDB", r.ID())
		}
	}
}
