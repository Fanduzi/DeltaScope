// Package ddl verifies the T05-A5 destination-conflict exception on mixed-case names.
// input: one CHANGE or RENAME statement whose old and new names differ only by case
// output: zero findings and zero gaps from the destination rule, while a different name still gaps
// pos: rule-level same-identity premise shared by Evaluate and EvidenceGaps; the parser folds case before AuditSQL
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestColumnTargetExistsSameIdentityHasNoGap(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ruleID  string
		action  string
		newName string
		wantGap bool
	}{
		{ruleID: ruleIDAlterChangeColumnTargetExistsForbid, action: "change_column", newName: "C"},
		{ruleID: ruleIDAlterRenameColumnTargetExistsForbid, action: "rename_column", newName: "C"},
		{ruleID: ruleIDAlterChangeColumnTargetExistsForbid, action: "change_column", newName: "c2", wantGap: true},
	}
	for _, tc := range cases {
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			tc, dialect := tc, dialect
			t.Run(tc.action+"/"+tc.newName+"/"+string(dialect), func(t *testing.T) {
				t.Parallel()
				checked, err := newColumnTargetExistsRule(tc.ruleID, tc.action, rule.LevelBlocker, policy.RulePolicy{})
				if err != nil {
					t.Fatalf("rule: %v", err)
				}
				statement := spec.Statement{
					Kind:    spec.KindDDL,
					Dialect: dialect,
					DDL: &spec.DDL{
						Operation: spec.DDLOperationAlterTable,
						Table:     &spec.Table{Name: "t"},
						Alter: []spec.Alter{{
							Action: tc.action,
							Name:   "c",
							Column: &spec.AlterColumn{OldName: "c", Definition: &spec.Column{Name: tc.newName, Type: "int"}},
						}},
					},
				}
				findings, err := checked.Evaluate(context.Background(), statement)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				gaps := checked.(rule.EvidenceReporter).EvidenceGaps(statement)
				if len(findings) != 0 {
					t.Fatalf("findings = %#v", findings)
				}
				if tc.wantGap {
					if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
						t.Fatalf("gaps = %#v", gaps)
					}
					return
				}
				if len(gaps) != 0 {
					t.Fatalf("same identity gaps = %#v", gaps)
				}
			})
		}
	}
}
