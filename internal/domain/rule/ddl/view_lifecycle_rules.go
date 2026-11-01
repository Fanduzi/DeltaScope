package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type viewLifecycleRule struct {
	ruleID string
	level  rule.Level
}

func newViewLifecycleRule(id string, fallback rule.Level, cfg policy.RulePolicy) rule.StatementRule {
	return viewLifecycleRule{ruleID: id, level: configuredLevel(cfg, fallback)}
}

func (r viewLifecycleRule) ID() string { return r.ruleID }

func (r viewLifecycleRule) AppliesTo(statement spec.Statement) bool {
	switch r.ruleID {
	case "ddl.view.target.valid", "ddl.view.source.valid", "ddl.view.security.definer.warn":
		if !orderedGapDialect(statement.Dialect) {
			return false
		}
	case "ddl.view.algorithm.warn", "ddl.view.check_option.valid":
		if statement.Dialect != spec.DialectMySQL {
			return false
		}
	case "ddl.view.options.ignored.warn":
		if statement.Dialect != spec.DialectTiDB {
			return false
		}
	default:
		return false
	}
	if statement.Kind != spec.KindDDL || statement.DDL == nil || statement.DDL.View == nil || statement.Unsupported != nil {
		return false
	}
	if statement.DDL.Operation == spec.DDLOperationDropView {
		return r.ruleID == "ddl.view.target.valid"
	}
	return statement.DDL.Operation == spec.DDLOperationCreateView || statement.DDL.Operation == spec.DDLOperationAlterView
}

func (r viewLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	assessment := spec.AssessViewLifecycle(statement)
	if statement.DDL.Operation == spec.DDLOperationDropView {
		var findings []rule.Finding
		for i, target := range assessment.DropTargets {
			if !target.Invalid {
				continue
			}
			metadata := map[string]any{
				"role": "target", "view": target.Binding.Table.Name, "view_schema": target.Binding.Table.Schema,
			}
			var location *rule.Location
			if i < len(statement.DDL.View.TargetOffsets) {
				location = viewOffsetLocation(statement, statement.DDL.View.TargetOffsets[i])
			}
			findings = append(findings, rule.Finding{
				RuleID: r.ruleID, Level: r.level, Message: "view target is invalid for this operation",
				Suggestion: "check the target relation and operation", Metadata: metadata, Location: location,
			})
		}
		return findings, nil
	}
	if r.ruleID == "ddl.view.source.valid" && assessment.SourceInvalid && len(assessment.SourceFailures) > 0 {
		findings := make([]rule.Finding, 0, len(assessment.SourceFailures))
		for _, failure := range assessment.SourceFailures {
			item := assessment
			item.SourceRootIndex, item.SourceBindingIndex, item.SourceOffset = failure.Root, failure.BindingIndex, failure.Offset
			findings = append(findings, rule.Finding{
				RuleID: r.ruleID, Level: r.level, Message: "view definition has an invalid source",
				Suggestion: "check the authored source relations and columns",
				Metadata:   viewFindingMetadata(statement, item, "source"), Location: viewSourceLocation(statement, item),
			})
		}
		return findings, nil
	}
	if r.ruleID == "ddl.view.options.ignored.warn" {
		var findings []rule.Finding
		for _, option := range []struct {
			explicit bool
			role     string
			offset   int
		}{
			{statement.DDL.View.AlgorithmExplicit, "algorithm", statement.DDL.View.AlgorithmOffset},
			{statement.DDL.View.CheckExplicit, "check", statement.DDL.View.CheckOffset},
		} {
			if option.explicit {
				findings = append(findings, rule.Finding{
					RuleID: r.ruleID, Level: r.level,
					Message:    "view option does not provide the requested guarantee",
					Suggestion: "review the vendor's supported view behavior",
					Metadata:   viewFindingMetadata(statement, assessment, option.role),
					Location:   viewOffsetLocation(statement, option.offset),
				})
			}
		}
		return findings, nil
	}
	var message, suggestion, role string
	switch r.ruleID {
	case "ddl.view.target.valid":
		if assessment.TargetInvalid {
			message = "view target is invalid for this operation"
			suggestion = "check the target relation and operation"
			role = "target"
		}
	case "ddl.view.source.valid":
		if assessment.SourceInvalid {
			message = "view definition has an invalid source"
			suggestion = "check the authored source relations and columns"
			role = "source"
		}
	case "ddl.view.security.definer.warn":
		if assessment.Security == "definer" || assessment.ReviewPrincipal {
			message = "view security or named definer requires review"
			suggestion = "review the view's privilege context"
			role = "view"
		}
	case "ddl.view.algorithm.warn":
		if assessment.AlgorithmWarn {
			message = "view algorithm requires review"
			suggestion = "review the requested algorithm and view definition"
			role = "algorithm"
		}
	case "ddl.view.check_option.valid":
		if assessment.CheckInvalid {
			message = "view check option requires an updatable view"
			suggestion = "use an updatable view definition"
			role = "check"
		}
	}
	if message == "" {
		return nil, nil
	}
	finding := rule.Finding{
		RuleID: r.ruleID, Level: r.level, Message: message, Suggestion: suggestion,
		Metadata: viewFindingMetadata(statement, assessment, role),
	}
	if role == "source" {
		finding.Location = viewSourceLocation(statement, assessment)
	} else if role == "algorithm" {
		finding.Location = viewOffsetLocation(statement, statement.DDL.View.AlgorithmOffset)
	} else if role == "check" {
		finding.Location = viewOffsetLocation(statement, statement.DDL.View.CheckOffset)
	} else if role == "view" && assessment.ReviewPrincipal {
		finding.Location = viewOffsetLocation(statement, statement.DDL.View.DefinerOffset)
	} else if role == "view" && statement.DDL.View.SecurityExplicit {
		finding.Location = viewOffsetLocation(statement, statement.DDL.View.SecurityOffset)
	}
	return []rule.Finding{finding}, nil
}

func viewFindingMetadata(statement spec.Statement, assessment spec.ViewLifecycleAssessment, role string) map[string]any {
	var target spec.Table
	if statement.DDL.Table != nil {
		target = *statement.DDL.Table
	}
	if statement.Metadata != nil && statement.Metadata.View != nil {
		target = statement.Metadata.View.TargetIdentity
	}
	metadata := map[string]any{"role": role, "view": target.Name, "view_schema": target.Schema}
	if role == "source" && assessment.SourceRootIndex >= 0 &&
		statement.Metadata != nil && statement.Metadata.View != nil &&
		assessment.SourceRootIndex < len(statement.Metadata.View.Sources) {
		source := statement.Metadata.View.Sources[assessment.SourceRootIndex].Table
		metadata["schema"] = source.Schema
		metadata["table"] = source.Name
	}
	return metadata
}

func viewSourceLocation(statement spec.Statement, assessment spec.ViewLifecycleAssessment) *rule.Location {
	if statement.Line <= 0 || statement.Column <= 0 || statement.DDL.View.Definition == nil {
		return nil
	}
	definition := statement.DDL.View.Definition
	offset := assessment.SourceOffset
	if offset >= 0 {
		return viewOffsetLocation(statement, offset)
	}
	if assessment.SourceBindingIndex >= 0 && assessment.SourceBindingIndex < len(definition.Bindings) {
		offset = definition.Bindings[assessment.SourceBindingIndex].Offset
	} else if index := assessment.SourceBindingIndex - len(definition.Bindings); index >= 0 && index < len(definition.References) {
		offset = definition.References[index].Offset
	}
	if offset < 0 && assessment.SourceRootIndex >= 0 && assessment.SourceRootIndex < len(definition.SourceOffsets) {
		offset = definition.SourceOffsets[assessment.SourceRootIndex]
	}
	return viewOffsetLocation(statement, offset)
}

func viewOffsetLocation(statement spec.Statement, offset int) *rule.Location {
	if statement.Line <= 0 || statement.Column <= 0 || offset < 0 || offset > len(statement.RawSQL) {
		return nil
	}
	location := &rule.Location{Line: statement.Line, Column: statement.Column}
	for _, r := range statement.RawSQL[:offset] {
		if r == '\n' {
			location.Line++
			location.Column = 1
		} else {
			location.Column++
		}
	}
	return location
}

func (r viewLifecycleRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	assessment := spec.AssessViewLifecycle(statement)
	var facts []string
	switch r.ruleID {
	case "ddl.view.target.valid":
		facts = assessment.TargetRequired
	case "ddl.view.source.valid":
		facts = assessment.SourceRequired
	case "ddl.view.security.definer.warn":
		if assessment.SecurityUnknown {
			facts = []string{"view.security"}
		}
	case "ddl.view.check_option.valid":
		facts = assessment.CheckRequired
	}
	if len(facts) == 0 {
		return nil
	}
	return []rule.EvidenceGap{{ReasonCode: gapReasonUnknownTableState, RequiredFacts: facts}}
}
