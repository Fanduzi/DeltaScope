package ddl

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateForeignKeyRule struct{ level rule.Level }

func newPGCreateForeignKeyRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateForeignKeyRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateForeignKeyRule) ID() string { return ruleIDPGCreateForeignKeyValid }

func (r pgCreateForeignKeyRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil &&
		s.DDL != nil && s.DDL.Operation == spec.DDLOperationCreateTable && s.DDL.PGCreateForeignKeys != nil
}

func assessPGCreateForeignKeys(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	decl := s.DDL.PGCreateForeignKeys
	if decl.Unbound || !decl.Target.Qualified() {
		return "", "create_table.foreign_key_namespace"
	}
	destination := s.Metadata.PGCreateDestination
	if destination == nil || destination.Target != decl.Target {
		return "", "create_table.foreign_key_destination"
	}
	switch {
	case !destination.SchemaExists:
		return "create_foreign_key_schema_absent", ""
	case decl.IfNotExists && destination.RelationExists:
		return "", ""
	case destination.RelationExists:
		return "create_foreign_key_destination_occupied", ""
	}
	if len(s.Metadata.PGCreateForeignKeys) != len(decl.Declarations) {
		return "", "referenced_table.pg_relation"
	}
	findSource := func(name string) (*spec.PGColumnState, bool) {
		if name == "" {
			return nil, false
		}
		if spec.PGSystemColumnName(name) {
			return &spec.PGColumnState{System: true}, true
		}
		if slices.ContainsFunc(s.DDL.Columns, func(c spec.Column) bool { return c.Name == name }) {
			return &spec.PGColumnState{Name: name}, true
		}
		return nil, true
	}
	for i, fk := range decl.Declarations {
		if fk.Reference == decl.Target.Table {
			return "", "create_table.self_reference"
		}
		if !fk.CreateReferenceQualified() || len(fk.Columns) == 0 {
			return "", "foreign_key.target_identity"
		}
		ref := s.Metadata.PGCreateForeignKeys[i]
		if ref.Target != fk.Reference {
			return "", "referenced_table.pg_relation"
		}
		reason, fact, _ = assessPGForeignKeyMembers(&fk, ref.Table, ref.Keys, findSource)
		if reason != "" || fact != "" {
			return reason, fact
		}
	}
	return "", "foreign_key.pg_equality_compatibility"
}

func (r pgCreateForeignKeyRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCreateForeignKeys(s)
	if reason == "" {
		return nil, nil
	}
	message := pgForeignKeyTargetMessage(reason)
	switch reason {
	case "create_foreign_key_schema_absent":
		message = "The PostgreSQL table creation schema does not exist"
	case "create_foreign_key_destination_occupied":
		message = "A PostgreSQL relation already occupies the creation target"
	}
	target := s.DDL.PGCreateForeignKeys.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "table": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCreateForeignKeyRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCreateForeignKeys(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_create_foreign_key", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
