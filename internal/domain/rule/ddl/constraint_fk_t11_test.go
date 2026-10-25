// Package ddl defines Tier-1 DDL rules.
// input: T11 unified PK/UNIQUE/FK CREATE/ALTER contract tests — foreign-key forbid and naming parity on alter.Constraint payloads, ALTER primary-key column-count parity, and the drop_foreign_key existence member rule over the constraint collection
// output: assertions that pin the frozen T11 public contract on the landed implementation
// pos: T11 rule tests (constraint-fk acceptance rows)
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func alterFKStatement(name string, constraint spec.Constraint) spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "child"},
			Alter: []spec.Alter{{
				Action:     "add_constraint",
				Name:       name,
				Constraint: &constraint,
			}},
		},
	}
}

func TestT11ForeignKeyForbidAppliesToAlterConstraint(t *testing.T) {
	t.Parallel()
	r, err := newTableForeignKeyForbidRule(policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new fk forbid rule: %v", err)
	}
	stmt := alterFKStatement("fk_c", spec.Constraint{
		Type:              "foreign_key",
		Name:              "fk_c",
		Columns:           []string{"c"},
		ReferencedTable:   "parent",
		ReferencedColumns: []string{"id"},
	})
	if !r.AppliesTo(stmt) {
		t.Fatalf("expected fk forbid to apply to ALTER add_constraint foreign_key")
	}
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Level != rule.LevelBlocker {
		t.Fatalf("expected blocker, got %s", f.Level)
	}
	md := f.Metadata
	if md["table"] != "child" || md["constraint"] != "fk_c" {
		t.Fatalf("unexpected metadata %#v", md)
	}
	cols, ok := md["columns"].([]string)
	if !ok || len(cols) != 1 || cols[0] != "c" {
		t.Fatalf("expected columns [c], got %#v", md["columns"])
	}
	if md["referenced_table"] != "parent" {
		t.Fatalf("expected referenced_table parent, got %#v", md["referenced_table"])
	}
	ref, ok := md["referenced_columns"].([]string)
	if !ok || len(ref) != 1 || ref[0] != "id" {
		t.Fatalf("expected referenced_columns [id], got %#v", md["referenced_columns"])
	}
}

func TestT11ForeignKeyForbidAlterMultiAndBare(t *testing.T) {
	t.Parallel()
	r, err := newTableForeignKeyForbidRule(policy.RulePolicy{Enabled: true, Params: map[string]any{"forbid": true}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	two := alterFKStatement("f1", spec.Constraint{Type: "foreign_key", Name: "f1", Columns: []string{"a"}, ReferencedTable: "p", ReferencedColumns: []string{"id"}})
	two.DDL.Alter = append(two.DDL.Alter, spec.Alter{
		Action:     "add_constraint",
		Name:       "f2",
		Constraint: &spec.Constraint{Type: "foreign_key", Name: "f2", Columns: []string{"b"}, ReferencedTable: "p", ReferencedColumns: []string{"id"}},
	})
	findings, err := r.Evaluate(context.Background(), two)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected two findings for two FK alters, got %d", len(findings))
	}

	bare := alterFKStatement("", spec.Constraint{Type: "foreign_key", Columns: []string{"c"}, ReferencedTable: "p", ReferencedColumns: []string{"id"}})
	findings, err = r.Evaluate(context.Background(), bare)
	if err != nil {
		t.Fatalf("evaluate bare: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one finding for unnamed FK, got %d", len(findings))
	}
	// A bare add authors no constraint name — the finding must not quote
	// an empty name or publish a `constraint` metadata key as if one were
	// authored.
	if strings.Contains(findings[0].Message, `""`) {
		t.Fatalf("bare-add message must not quote an empty name: %q", findings[0].Message)
	}
	if _, ok := findings[0].Metadata["constraint"]; ok {
		t.Fatalf("bare-add must not fabricate a constraint metadata key: %#v", findings[0].Metadata)
	}
}

func TestT11ForeignKeyForbidAlterActionMetadata(t *testing.T) {
	t.Parallel()
	r, err := newTableForeignKeyForbidRule(policy.RulePolicy{Enabled: true, Params: map[string]any{"forbid": true}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := alterFKStatement("fk_bx", spec.Constraint{
		Type:              "foreign_key",
		Name:              "fk_bx",
		Columns:           []string{"b", "x"},
		ReferencedSchema:  "other",
		ReferencedTable:   "ref",
		ReferencedColumns: []string{"y", "z"},
		OnDelete:          "set_null",
		OnUpdate:          "cascade",
	})
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(findings))
	}
	md := findings[0].Metadata
	if md["referenced_schema"] != "other" || md["on_delete"] != "set_null" || md["on_update"] != "cascade" {
		t.Fatalf("expected action+schema metadata, got %#v", md)
	}
	cols := md["columns"].([]string)
	if len(cols) != 2 || cols[0] != "b" || cols[1] != "x" {
		t.Fatalf("ordered columns lost: %#v", cols)
	}
}

func TestT11ForeignKeyNameSelectorSeesAlterConstraint(t *testing.T) {
	t.Parallel()
	r, err := newNamingPrefixRule(ruleIDConstraintForeignKeyNamePrefixRequire, "foreign key constraint", rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"prefix": "fk_"},
	}, selectForeignKeyConstraintNames, appliesToCreateTableOrAlterFKConstraint)
	if err != nil {
		t.Fatalf("new naming rule: %v", err)
	}
	bad := alterFKStatement("bad", spec.Constraint{Type: "foreign_key", Name: "bad", Columns: []string{"c"}, ReferencedTable: "p", ReferencedColumns: []string{"id"}})
	if !r.AppliesTo(bad) {
		t.Fatalf("expected fk naming rule to apply to ALTER add_constraint foreign_key")
	}
	findings, err := r.Evaluate(context.Background(), bad)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one naming finding for ALTER FK name, got %d", len(findings))
	}
	ok := alterFKStatement("fk_x", spec.Constraint{Type: "foreign_key", Name: "fk_x", Columns: []string{"c"}, ReferencedTable: "p", ReferencedColumns: []string{"id"}})
	findings, err = r.Evaluate(context.Background(), ok)
	if err != nil {
		t.Fatalf("evaluate ok: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no finding for compliant name, got %d", len(findings))
	}
}

func TestT11PrimaryKeyColumnCountAlterConstraint(t *testing.T) {
	t.Parallel()
	r, err := newPrimaryKeyColumnCountRule(policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"limit": 1},
	})
	if err != nil {
		t.Fatalf("new pk count rule: %v", err)
	}
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "t"},
			Alter: []spec.Alter{{
				Action: "add_constraint",
				Name:   "primary",
				Index:  &spec.AlterIndex{Definition: &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"a", "b"}}},
			}},
		},
	}
	if !r.AppliesTo(stmt) {
		t.Fatalf("expected pk columns rule to apply to ALTER ADD PRIMARY KEY")
	}
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one finding for 2-column PK over limit 1, got %d", len(findings))
	}
	if findings[0].Metadata["actual"] != 2 {
		t.Fatalf("expected actual=2, got %#v", findings[0].Metadata)
	}
}

func TestT11DropForeignKeyExistsRule(t *testing.T) {
	t.Parallel()
	r, err := newAlterObjectExistenceRule(ruleIDAlterDropForeignKeyExistsRequire,
		[]string{"drop_foreign_key"}, "foreign_key", false, rule.LevelBlocker,
		policy.RulePolicy{Enabled: true}, alterObjectName, snapshotHasForeignKeyConstraint)
	if err != nil {
		t.Fatalf("new drop-fk exists rule: %v", err)
	}
	reporter, ok := r.(rule.EvidenceReporter)
	if !ok {
		t.Fatalf("drop-fk exists rule must report evidence gaps")
	}
	mk := func(constraints []spec.Constraint, unknown bool, snapshotNil bool) spec.Statement {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "child"},
				Alter:     []spec.Alter{{Action: "drop_foreign_key", Name: "fk_c"}},
			},
		}
		if !snapshotNil {
			stmt.Metadata = &spec.Metadata{TargetTable: &spec.TableSnapshot{
				Exists:             true,
				Table:              &spec.Table{Name: "child"},
				Constraints:        constraints,
				ConstraintsUnknown: unknown,
			}}
		}
		return stmt
	}

	// Proven present: no finding, no gap.
	present := mk([]spec.Constraint{{Type: "foreign_key", Name: "fk_c", Columns: []string{"c"}}}, false, false)
	findings, err := r.Evaluate(context.Background(), present)
	if err != nil {
		t.Fatalf("evaluate present: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no finding for proven-present fk, got %d", len(findings))
	}
	if gaps := reporter.EvidenceGaps(present); len(gaps) != 0 {
		t.Fatalf("expected no gap for proven-present fk, got %v", gaps)
	}

	// Proven absent (complete empty set): one missing finding.
	absent := mk([]spec.Constraint{}, false, false)
	findings, err = r.Evaluate(context.Background(), absent)
	if err != nil {
		t.Fatalf("evaluate absent: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one missing finding, got %d", len(findings))
	}
	md := findings[0].Metadata
	if md["name"] != "fk_c" || md["exists"] != false || md["table"] != "child" {
		t.Fatalf("unexpected missing-finding metadata %#v", md)
	}
	if gaps := reporter.EvidenceGaps(absent); len(gaps) != 0 {
		t.Fatalf("proven set owes no gap, got %v", gaps)
	}

	// ConstraintsUnknown: no finding, constraint-collection gap.
	unk := mk(nil, true, false)
	findings, err = r.Evaluate(context.Background(), unk)
	if err != nil {
		t.Fatalf("evaluate unknown: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no finding under unproven set, got %d", len(findings))
	}
	gaps := reporter.EvidenceGaps(unk)
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.constraints" {
		t.Fatalf("expected target_table.constraints gap, got %v", gaps)
	}

	// Nil collection is the loaded empty set under the unified collection
	// contract — identical to the proven-empty arm: one missing finding,
	// no gap. The old oracle read nil as "provider did not vouch"; the
	// contract now reserves unknown for ConstraintsUnknown, so the arm
	// below pins the flagged-unknown shape instead.
	nilSet := mk(nil, false, false)
	findings, err = r.Evaluate(context.Background(), nilSet)
	if err != nil {
		t.Fatalf("evaluate nil: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("nil = proven-empty set must answer the missing finding, got %d", len(findings))
	}
	if gaps = reporter.EvidenceGaps(nilSet); len(gaps) != 0 {
		t.Fatalf("proven-empty nil set owes no gap, got %v", gaps)
	}

	// Nil collection plus the explicit unknown flag: the flagged shape
	// stays the withholding arm — no finding, collection gap.
	nilUnknown := mk(nil, true, false)
	findings, err = r.Evaluate(context.Background(), nilUnknown)
	if err != nil {
		t.Fatalf("evaluate nil+unknown: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no finding under flagged-unknown set, got %d", len(findings))
	}
	gaps = reporter.EvidenceGaps(nilUnknown)
	if len(gaps) != 1 || gaps[0].RequiredFacts[0] != "target_table.constraints" {
		t.Fatalf("expected constraints gap under flagged-unknown set, got %v", gaps)
	}

	// No snapshot: existence+constraints gap ordered like member rules.
	noSnap := mk(nil, false, true)
	if findings, err = r.Evaluate(context.Background(), noSnap); err != nil || len(findings) != 0 {
		t.Fatalf("expected no finding without snapshot, got findings=%d err=%v", len(findings), err)
	}
	gaps = reporter.EvidenceGaps(noSnap)
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 2 ||
		gaps[0].RequiredFacts[0] != "target_table.constraints" || gaps[0].RequiredFacts[1] != "target_table.existence" {
		t.Fatalf("expected [constraints existence] gap, got %v", gaps)
	}
}

// The PostgreSQL cross-schema advisory judges an ALTER-declared foreign
// key exactly like a CREATE-declared one — the referenced_schema option
// is a judged fact, never a silently-applicable pass.
func TestT11CrossSchemaAdvisoryJudgesAlterAdd(t *testing.T) {
	t.Parallel()
	r, err := newTableForeignKeyCrossSchemaAdvisoryRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "child", Schema: "app"},
			Alter: []spec.Alter{{
				Action: "add_constraint",
				Name:   "fk_c",
				Options: map[string]string{
					"constraint_type":    "foreign_key",
					"columns":            "c",
					"referenced_schema":  "other",
					"referenced_table":   "parent",
					"referenced_columns": "id",
				},
			}},
		},
	}
	if !r.AppliesTo(stmt) {
		t.Fatal("cross-schema advisory must apply to ALTER add_constraint foreign_key")
	}
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one cross-schema finding, got %d", len(findings))
	}
	md := findings[0].Metadata
	if md["constraint"] != "fk_c" || md["referenced_schema"] != "other" || md["table_schema"] != "app" {
		t.Fatalf("cross-schema metadata wrong: %#v", md)
	}

	// Same-schema reference stays silent on the ALTER path too.
	stmt.DDL.Alter[0].Options["referenced_schema"] = "app"
	findings, err = r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate same-schema: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("same-schema reference must stay silent, got %d", len(findings))
	}
}
