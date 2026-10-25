// Package ddl defines Tier-1 DDL rules.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type singlePrimaryKeyColumnRule struct {
	ruleID    string
	level     rule.Level
	required  bool
	predicate func(spec.Column) bool
	message   string
	suggest   string
}

func newSinglePrimaryKeyColumnRule(ruleID string, fallbackLevel rule.Level, message, suggest string, predicate func(spec.Column) bool, cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleID, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	if ruleID == ruleIDPrimaryKeyBigintRequire {
		basePredicate := predicate
		predicate = func(column spec.Column) bool {
			return basePredicate(column) || isPrimaryKeyBigintLike(column)
		}
	}
	return singlePrimaryKeyColumnRule{
		ruleID:    ruleID,
		level:     configuredLevel(cfg, fallbackLevel),
		required:  required,
		predicate: predicate,
		message:   message,
		suggest:   suggest,
	}, nil
}

func (r singlePrimaryKeyColumnRule) ID() string { return r.ruleID }

func (r singlePrimaryKeyColumnRule) AppliesTo(statement spec.Statement) bool {
	if !r.required || isPostgreSQLDialectGatedPrimaryKeyRule(r.ruleID, statement.Dialect) {
		return false
	}
	return appliesToCreateTable(statement) && statement.DDL.PrimaryKey != nil ||
		appliesToAlterAddConstraintPrimaryKey(statement)
}

func (r singlePrimaryKeyColumnRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	// The ALTER-declared member carries no type facts of its own — resolve
	// its name through the statement's ordered column facts. Only a
	// resolved member predicates; an absent member is the native refusal
	// and an undecidable one is the missing-fact boundary EvidenceGaps
	// owns — a zero-value column never fabricates a violation.
	if appliesToAlterTable(statement) {
		members := alterDeclaredPrimaryKeyMembers(statement)
		if len(members) != 1 {
			return nil, nil
		}
		column, verdict := orderedColumnFact(statement, members[0])
		if verdict != orderedColumnResolved || r.predicate(column) {
			return nil, nil
		}
		return []rule.Finding{r.finding(statement, column)}, nil
	}
	columns := primaryKeyColumnSpecs(statement)
	if len(columns) != 1 || r.predicate(columns[0]) {
		return nil, nil
	}
	return []rule.Finding{r.finding(statement, columns[0])}, nil
}

func (r singlePrimaryKeyColumnRule) finding(statement spec.Statement, column spec.Column) rule.Finding {
	return rule.Finding{
		Level:      r.level,
		Message:    fmt.Sprintf("primary key column %q %s", column.Name, r.message),
		Suggestion: r.suggest,
		Metadata: map[string]any{
			"table":  statement.DDL.Table.Name,
			"column": column.Name,
			"type":   column.Type,
		},
	}
}

// EvidenceGaps reports the ALTER-path premise gaps under the same
// decidability premise Evaluate shares: a target whose state was never
// provided owes the existence-plus-columns facts, a present table with a
// withheld column set owes the columns fact, a member whose outcome the
// statement cannot decide (dropped mid-statement, or a same-statement
// declaration TiDB refuses to apply) owes the same columns fact, and a
// settled-absent table, an out-of-scope member count, or a member
// proven-absent at the key action owes nothing — a proven native refusal
// is not a fact gap.
func (r singlePrimaryKeyColumnRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !orderedGapDialect(statement.Dialect) || !r.AppliesTo(statement) || !appliesToAlterTable(statement) {
		return nil
	}
	members := alterDeclaredPrimaryKeyMembers(statement)
	if len(members) != 1 {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"target_table.columns", "target_table.existence"},
		}}
	}
	if !snapshot.Exists {
		return nil
	}
	if snapshot.Columns == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"target_table.columns"},
		}}
	}
	if _, verdict := orderedColumnFact(statement, members[0]); verdict == orderedColumnUndecidable {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"target_table.columns"},
		}}
	}
	return nil
}

type primaryKeyNotNullRule struct {
	required bool
	level    rule.Level
}

func newPrimaryKeyNotNullRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDPrimaryKeyNotNullRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return primaryKeyNotNullRule{required: required, level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r primaryKeyNotNullRule) ID() string { return ruleIDPrimaryKeyNotNullRequire }

func (r primaryKeyNotNullRule) AppliesTo(statement spec.Statement) bool {
	return r.required && !isPostgreSQLDialectGatedPrimaryKeyRule(ruleIDPrimaryKeyNotNullRequire, statement.Dialect) && appliesToCreateTable(statement) && statement.DDL.PrimaryKey != nil
}

func (r primaryKeyNotNullRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	findings := make([]rule.Finding, 0)
	for _, column := range primaryKeyColumnSpecs(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if column.NotNull {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("primary key column %q must be NOT NULL", column.Name),
			Suggestion: "add an explicit NOT NULL constraint to the primary key column",
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"column": column.Name,
			},
		})
	}
	return findings, nil
}

func isPrimaryKeyBigintLike(column spec.Column) bool {
	switch baseType(column) {
	case "bigint", "int8", "pg_catalog.int8":
		return true
	default:
		return false
	}
}

func isPostgreSQLDialectGatedPrimaryKeyRule(ruleID string, dialect spec.Dialect) bool {
	if dialect != spec.DialectPostgreSQL {
		return false
	}
	switch ruleID {
	case ruleIDPrimaryKeyUnsignedRequire, ruleIDPrimaryKeyAutoIncrementRequire, ruleIDPrimaryKeyNotNullRequire:
		return true
	default:
		return false
	}
}
