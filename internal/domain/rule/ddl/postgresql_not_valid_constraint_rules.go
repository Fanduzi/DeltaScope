// Package ddl defines Tier-1 DDL rules.
// input: PostgreSQL DDL statement batches with NOT VALID and VALIDATE constraint alter actions
// output: global findings for NOT VALID constraints without later validation in the same audit batch
// pos: PostgreSQL global migration-safety rule for deferred constraint validation pairing
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type notValidConstraintValidateRequiredRule struct {
	level rule.Level
}

type notValidConstraintKey struct {
	schema     string
	table      string
	constraint string
}

type pendingNotValidConstraint struct {
	key            notValidConstraintKey
	constraintType string
	statementIndex int
}

func newNotValidConstraintValidateRequiredRule(cfg policy.RulePolicy) (rule.GlobalRule, error) {
	required, err := boolParam(ruleIDPGAlterNotValidConstraintValidateRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	if !required {
		return notValidConstraintValidateRequiredRule{}, nil
	}
	return notValidConstraintValidateRequiredRule{
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r notValidConstraintValidateRequiredRule) ID() string {
	return ruleIDPGAlterNotValidConstraintValidateRequire
}

func (r notValidConstraintValidateRequiredRule) EvaluateAll(ctx context.Context, statements []spec.Statement) ([]rule.Finding, error) {
	if r.level == "" {
		return nil, nil
	}

	pending := make([]pendingNotValidConstraint, 0)
	for idx, statement := range statements {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if statement.Dialect != spec.DialectPostgreSQL || statement.Kind != spec.KindDDL || statement.DDL == nil {
			continue
		}
		if statement.DDL.Operation != spec.DDLOperationAlterTable || statement.DDL.Table == nil {
			continue
		}
		for _, alter := range statement.DDL.Alter {
			switch alter.Action {
			case "add_constraint":
				item, ok := notValidConstraintAddition(statement, alter, idx)
				if ok {
					pending = append(pending, item)
				}
			case "validate_constraint":
				key, ok := validateConstraintKey(statement, alter)
				if !ok {
					continue
				}
				pending = removeValidatedNotValidConstraints(pending, key)
			}
		}
	}

	findings := make([]rule.Finding, 0, len(pending))
	for _, item := range pending {
		findings = append(findings, r.finding(item))
	}
	return findings, nil
}

func notValidConstraintAddition(statement spec.Statement, alter spec.Alter, statementIndex int) (pendingNotValidConstraint, bool) {
	if alter.Name == "" || alter.Options["not_valid"] != "true" {
		return pendingNotValidConstraint{}, false
	}
	constraintType := alter.Options["constraint_type"]
	if constraintType != "check" && constraintType != "foreign_key" {
		return pendingNotValidConstraint{}, false
	}
	key, ok := alterConstraintKey(statement, alter.Name)
	if !ok {
		return pendingNotValidConstraint{}, false
	}
	return pendingNotValidConstraint{
		key:            key,
		constraintType: constraintType,
		statementIndex: statementIndex,
	}, true
}

func validateConstraintKey(statement spec.Statement, alter spec.Alter) (notValidConstraintKey, bool) {
	if alter.Name == "" {
		return notValidConstraintKey{}, false
	}
	return alterConstraintKey(statement, alter.Name)
}

func alterConstraintKey(statement spec.Statement, constraint string) (notValidConstraintKey, bool) {
	if statement.DDL == nil || statement.DDL.Table == nil || statement.DDL.Table.Name == "" || constraint == "" {
		return notValidConstraintKey{}, false
	}
	return notValidConstraintKey{
		schema:     statement.DDL.Table.Schema,
		table:      statement.DDL.Table.Name,
		constraint: constraint,
	}, true
}

func removeValidatedNotValidConstraints(items []pendingNotValidConstraint, key notValidConstraintKey) []pendingNotValidConstraint {
	if len(items) == 0 {
		return items
	}
	out := items[:0]
	for _, item := range items {
		if item.key == key {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (r notValidConstraintValidateRequiredRule) finding(item pendingNotValidConstraint) rule.Finding {
	table := item.key.table
	if item.key.schema != "" {
		table = item.key.schema + "." + item.key.table
	}
	return rule.Finding{
		Level:   r.level,
		Message: fmt.Sprintf("NOT VALID constraint %q on table %q should be followed by VALIDATE CONSTRAINT in the audited migration batch", item.key.constraint, table),
		Explanation: &rule.FindingExplanation{
			Why:        "NOT VALID registers the constraint without validating existing rows; the constraint remains unvalidated until PostgreSQL runs VALIDATE CONSTRAINT.",
			Risk:       "Existing rows may continue violating the constraint, and reviewers can miss the deferred validation step if it is not present in the reviewed migration batch.",
			Suggestion: "Add a later ALTER TABLE ... VALIDATE CONSTRAINT ... statement in the same migration batch, or disable this rule when validation is intentionally handled by a separate deployment.",
		},
		Metadata: map[string]any{
			"action":            "add_constraint",
			"required_followup": "validate_constraint",
			"dialect":           spec.DialectPostgreSQL.String(),
			"schema":            item.key.schema,
			"table":             item.key.table,
			"constraint":        item.key.constraint,
			"constraint_type":   item.constraintType,
			"not_valid":         true,
			"statement_index":   item.statementIndex,
		},
	}
}
