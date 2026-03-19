// Package dml defines Tier-1 DML rules.
// input: domain policy values and a shared rule registry
// output: deterministic registration of the first DML rule batch
// pos: DML rule assembly entrypoint for application wiring
// note: if this file changes, update this header and module README.md.
package dml

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

// Register installs the first Tier-1 DML rule batch into the shared registry.
func Register(registry *rule.Registry, cfg policy.Policy) error {
	for _, factory := range []struct {
		ruleID    string
		construct func(policy.RulePolicy) (rule.StatementRule, error)
	}{
		{ruleID: ruleIDWhereRequire, construct: newWhereRequiredRule},
		{ruleID: ruleIDLimitForbid, construct: newLimitForbiddenRule},
		{ruleID: ruleIDOrderByForbid, construct: newOrderByForbiddenRule},
		{ruleID: ruleIDSubqueryForbid, construct: newSubqueryForbiddenRule},
		{ruleID: ruleIDJoinOnRequire, construct: newJoinOnRequiredRule},
		{ruleID: ruleIDInsertRowsMaxCount, construct: newInsertRowsMaxCountRule},
		{ruleID: ruleIDReplaceForbid, construct: newReplaceForbiddenRule},
		{ruleID: ruleIDInsertSelectForbid, construct: newInsertSelectForbiddenRule},
		{ruleID: ruleIDOnDuplicateForbid, construct: newOnDuplicateForbiddenRule},
	} {
		ruleCfg, ok := cfg.Rules[factory.ruleID]
		if !ok || !ruleCfg.Enabled {
			continue
		}

		statementRule, err := factory.construct(ruleCfg)
		if err != nil {
			return err
		}
		if err := registry.RegisterStatement(statementRule); err != nil {
			return err
		}
	}

	return nil
}
