// Package ddl defines Tier-1 DDL rules.
// input: domain policy values and a shared rule registry
// output: deterministic registration of the first DDL rule batch
// pos: DDL rule assembly entrypoint for application wiring
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

// Register installs the first Tier-1 DDL rule batch into the shared registry.
func Register(registry *rule.Registry, cfg policy.Policy) error {
	for _, factory := range []struct {
		ruleID    string
		construct func(policy.RulePolicy) (rule.StatementRule, error)
	}{
		{ruleID: ruleIDTableCommentRequired, construct: newTableCommentRequiredRule},
		{ruleID: ruleIDTableNameMaxLength, construct: newTableNameMaxLengthRule},
		{ruleID: ruleIDPrimaryKeyRequired, construct: newPrimaryKeyRequiredRule},
		{ruleID: ruleIDPrimaryKeyColumnsMaxCount, construct: newPrimaryKeyColumnCountRule},
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
