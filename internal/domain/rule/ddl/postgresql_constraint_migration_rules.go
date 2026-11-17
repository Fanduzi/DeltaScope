package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgConstraintMigrationRule struct {
	primaryKey bool
	level      rule.Level
}

func newPGForeignKeyNotValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgConstraintMigrationRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func newPGPrimaryKeyConcurrentIndexRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgConstraintMigrationRule{primaryKey: true, level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r pgConstraintMigrationRule) ID() string {
	if r.primaryKey {
		return ruleIDPGAlterAddPrimaryKeyConcurrentIndexWarn
	}
	return ruleIDPGAlterAddForeignKeyNotValidRequire
}

func (r pgConstraintMigrationRule) declaration() (kind, option, fact string) {
	if r.primaryKey {
		return "primary_key", "using_index", "constraint.using_index"
	}
	return "foreign_key", "not_valid", "constraint.not_valid"
}

func (r pgConstraintMigrationRule) AppliesTo(s spec.Statement) bool {
	if s.Dialect != spec.DialectPostgreSQL || s.Kind != spec.KindDDL || s.DDL == nil || s.DDL.Table == nil || s.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	kind, _, _ := r.declaration()
	for _, a := range s.DDL.Alter {
		if a.Action == "add_constraint" && a.Options["constraint_type"] == kind {
			return true
		}
	}
	return false
}

func (r pgConstraintMigrationRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(s) {
		return nil
	}
	kind, option, fact := r.declaration()
	for _, a := range s.DDL.Alter {
		if a.Action == "add_constraint" && a.Options["constraint_type"] == kind && a.Options[option] != "true" && a.Options[option] != "false" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_constraint_declaration", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}

func (r pgConstraintMigrationRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	kind, option, _ := r.declaration()
	message := "ADD FOREIGN KEY without NOT VALID requests initial validation of existing rows on PostgreSQL"
	suggestion := "review the validation scan and locks on both tables; consider NOT VALID then VALIDATE CONSTRAINT where supported (PostgreSQL 17 does not allow NOT VALID foreign keys on partitioned tables)"
	if r.primaryKey {
		message = "ADD PRIMARY KEY without USING INDEX requests an index build under PostgreSQL's ALTER TABLE lock"
		suggestion = "review the index build and lock impact; consider a separately built eligible unique index via CREATE UNIQUE INDEX CONCURRENTLY then ADD PRIMARY KEY USING INDEX where supported; attachment and NOT NULL checks can still require locks or scans"
	}
	var findings []rule.Finding
	for _, a := range s.DDL.Alter {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if a.Action != "add_constraint" || a.Options["constraint_type"] != kind || a.Options[option] != "false" {
			continue
		}
		metadata := map[string]any{"action": "add_constraint", "table": s.DDL.Table.Name, "constraint_type": kind, option: false}
		if a.Name != "" {
			metadata["constraint"] = a.Name
		}
		if s.DDL.Table.Schema != "" {
			metadata["schema"] = s.DDL.Table.Schema
		}
		findings = append(findings, rule.Finding{Level: r.level, Message: message, Suggestion: suggestion, Metadata: metadata})
	}
	return findings, nil
}
