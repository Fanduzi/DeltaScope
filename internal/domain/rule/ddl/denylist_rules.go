// Package ddl defines Tier-1 DDL rules.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type tableDenylistRule struct {
	ruleID          string
	level           rule.Level
	schemas         map[string]struct{}
	tables          map[string]struct{}
	qualifiedTables map[string]struct{}
}

func newTableDenylistRule(ruleID string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	schemas, err := normalizedStringSetParam(ruleID, cfg, "schemas", nil)
	if err != nil {
		return nil, err
	}
	tables, err := normalizedStringSetParam(ruleID, cfg, "tables", nil)
	if err != nil {
		return nil, err
	}
	qualifiedTables, err := normalizedStringSetParam(ruleID, cfg, "qualified_tables", nil)
	if err != nil {
		return nil, err
	}
	return tableDenylistRule{
		ruleID:          ruleID,
		level:           configuredLevel(cfg, fallbackLevel),
		schemas:         schemas,
		tables:          tables,
		qualifiedTables: qualifiedTables,
	}, nil
}

func (r tableDenylistRule) ID() string { return r.ruleID }

func (r tableDenylistRule) AppliesTo(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL && statement.DDL != nil && len(statement.DDL.TableTargets()) > 0
}

func (r tableDenylistRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	metadataSchema := normalizedMetadataSchema(statement)
	seen := make(map[spec.Table]struct{})
	findings := make([]rule.Finding, 0)
	targets := append([]spec.Table(nil), statement.DDL.TableTargets()...)
	targetCount := len(targets)
	if statement.DDL.View != nil && statement.DDL.View.Definition != nil {
		for i, source := range statement.DDL.View.Definition.Sources {
			if statement.Metadata != nil && statement.Metadata.View != nil && i < len(statement.Metadata.View.Sources) {
				source = statement.Metadata.View.Sources[i].Table
			}
			targets = append(targets, source)
		}
	}
	for i, table := range targets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tableName := strings.ToLower(strings.TrimSpace(table.Name))
		schemaName := strings.ToLower(strings.TrimSpace(table.Schema))
		if schemaName == "" && i < targetCount {
			schemaName = metadataSchema
		}
		if !r.matches(schemaName, tableName) {
			continue
		}
		identity := spec.Table{Schema: schemaName, Name: tableName}
		if _, duplicate := seen[identity]; duplicate {
			continue
		}
		seen[identity] = struct{}{}
		target := qualifiedTableName(schemaName, tableName)
		var location *rule.Location
		if statement.DDL.View != nil {
			if statement.DDL.Operation == spec.DDLOperationDropView && i < len(statement.DDL.View.TargetOffsets) {
				location = viewOffsetLocation(statement, statement.DDL.View.TargetOffsets[i])
			} else if i >= targetCount && statement.DDL.View.Definition != nil &&
				i-targetCount < len(statement.DDL.View.Definition.SourceOffsets) {
				location = viewOffsetLocation(statement, statement.DDL.View.Definition.SourceOffsets[i-targetCount])
			}
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("DDL target %q is blocked by the table denylist policy", target),
			Suggestion: "run the change against an allowed table or relax the denylist policy intentionally",
			Location:   location,
			Metadata: map[string]any{
				"schema": schemaName,
				"table":  tableName,
			},
		})
	}
	if statement.Metadata != nil && statement.Metadata.View != nil {
		for i, dependency := range statement.Metadata.View.Dependencies {
			if r.matches(dependency.Table.Schema, dependency.Table.Name) {
				root := -1
				if i < len(statement.Metadata.View.DependencyRoots) {
					root = statement.Metadata.View.DependencyRoots[i]
				}
				assessment := spec.ViewLifecycleAssessment{SourceRootIndex: root, SourceBindingIndex: -1, SourceOffset: -1}
				findings = append(findings, rule.Finding{
					Level: r.level, Message: "view depends on a relation blocked by the table denylist policy",
					Suggestion: "review the view's dependencies or relax the denylist policy intentionally",
					Metadata:   viewFindingMetadata(statement, assessment, "source"),
					Location:   viewSourceLocation(statement, assessment),
				})
				break
			}
		}
	}
	return findings, nil
}

func (r tableDenylistRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) || statement.DDL.View == nil || statement.DDL.View.Definition == nil ||
		len(r.schemas)+len(r.tables)+len(r.qualifiedTables) == 0 {
		return nil
	}
	definition := statement.DDL.View.Definition
	var facts []string
	add := func(fact string) {
		for _, existing := range facts {
			if existing == fact {
				return
			}
		}
		facts = append(facts, fact)
	}
	if !definition.SourcesKnown {
		shapeUnsupported := false
		for _, option := range statement.DDL.UnextractedOptions {
			if option == "view_query_shape" {
				shapeUnsupported = true
				break
			}
		}
		if !shapeUnsupported {
			add("view.dependencies")
		}
	}
	if len(definition.Sources) > 0 && (statement.Metadata == nil || statement.Metadata.View == nil) {
		add("view.dependencies")
	} else if statement.Metadata != nil && statement.Metadata.View != nil {
		view := statement.Metadata.View
		if view.DependencyUnproven {
			add("view.dependencies")
		}
		bindings := append(append([]spec.TargetBinding(nil), view.Sources...), view.Dependencies...)
		for _, binding := range bindings {
			snapshot := binding.Snapshot
			switch {
			case snapshot == nil:
				add("relation.kind")
			case !snapshot.Exists || snapshot.Temporary:
				continue
			case snapshot.RelationKind == "":
				add("relation.kind")
			case snapshot.RelationKind == "view" && (snapshot.View == nil || snapshot.View.Definition == nil):
				add("view.definition")
			case snapshot.RelationKind == "view" && !snapshot.View.Definition.SourcesKnown:
				add("view.dependencies")
			}
		}
	}
	if len(facts) == 0 {
		return nil
	}
	return []rule.EvidenceGap{{ReasonCode: gapReasonUnknownTableState, RequiredFacts: facts}}
}

func (r tableDenylistRule) matches(schemaName, tableName string) bool {
	if tableName == "" {
		return false
	}
	if _, ok := r.tables[tableName]; ok {
		return true
	}
	if schemaName != "" {
		if _, ok := r.schemas[schemaName]; ok {
			return true
		}
		if _, ok := r.qualifiedTables[qualifiedTableName(schemaName, tableName)]; ok {
			return true
		}
	}
	return false
}

func normalizedMetadataSchema(statement spec.Statement) string {
	if statement.Metadata == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(statement.Metadata.Schema))
}

func qualifiedTableName(schemaName, tableName string) string {
	if schemaName == "" {
		return tableName
	}
	return schemaName + "." + tableName
}
