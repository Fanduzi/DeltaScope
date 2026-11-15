// Package audit orchestrates audit use cases at the application layer.
package audit

import (
	"context"
	"sort"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// EvaluateStatements applies registered rules and aggregates their findings into a report result.
func EvaluateStatements(ctx context.Context, registry *rule.Registry, statements []spec.Statement) (report.Result, error) {
	statementResults := make([]report.StatementResult, 0, len(statements))
	unsupported := make([]spec.UnsupportedDetail, 0, len(statements)/4+1)
	supportedStatements := make([]spec.Statement, 0, len(statements))

	appliedIDs := make(map[string]struct{})
	skippedDedup := make(map[string]rule.SkippedRule)

	for idx, statement := range statements {
		if err := ctx.Err(); err != nil {
			return report.Result{}, err
		}
		coverage, gaps := statementCoverage(statement.Dialect, statement)
		if statement.Unsupported != nil || statement.ResourceLimit != nil {
			if statement.ResourceLimit == nil {
				gaps = []spec.UnsupportedDetail{*statement.Unsupported}
			}
			for _, item := range gaps {
				item.Index = idx
				if statement.SensitiveSQL() {
					// A sensitive family's public SQL is always the
					// fixed-shape rendering — a pre-set item.SQL can
					// never outrank it (T26/#105).
					item.SQL = statement.PublicSQL()
				} else if item.SQL == "" {
					item.SQL = statement.RawSQL
				}
				unsupported = append(unsupported, item)
			}
			statementResults = append(statementResults, report.StatementResult{
				Index:         idx,
				Kind:          statement.Kind.String(),
				RawSQL:        statement.PublicSQL(),
				NormalizedSQL: statement.PublicNormalizedSQL(),
				Coverage:      coverage,
			})
			if err := ctx.Err(); err != nil {
				return report.Result{}, err
			}
			continue
		}

		eval, err := registry.EvaluateStatementDetailed(ctx, statement)
		if err != nil {
			return report.Result{}, err
		}

		// A stored-routine definition audits its statically bounded body
		// through the same registry: member findings bind to the parent
		// statement index and the body's original source positions; member
		// unsupported forms and evidence gaps degrade the parent's coverage
		// exactly like top-level equivalents (T27/#106).
		var bodyFindings []rule.Finding
		var bodyUnsupported []spec.UnsupportedDetail
		if statement.DDL != nil && statement.DDL.Routine != nil && statement.DDL.Routine.Body != nil {
			var bodyEval *rule.StatementEvaluation
			bodyFindings, bodyUnsupported, bodyEval, err = evaluateRoutineBody(ctx, registry, statement, idx)
			if err != nil {
				return report.Result{}, err
			}
			for _, id := range bodyEval.AppliedRuleIDs {
				appliedIDs[id] = struct{}{}
			}
			for _, s := range bodyEval.Skipped {
				if _, seen := skippedDedup[s.RuleID]; !seen {
					skippedDedup[s.RuleID] = s
				}
			}
			eval.EvidenceGaps = append(eval.EvidenceGaps, bodyEval.EvidenceGaps...)
			gaps = append(gaps, bodyUnsupported...)
			if len(bodyUnsupported) > 0 && coverage.Status == report.CoverageComplete {
				coverage.Status = report.CoverageIncomplete
			}
		}

		findings := append(eval.Findings, bodyFindings...)
		for i := range findings {
			findings[i].StatementIndex = idx
			findings[i].StatementKind = statement.Kind.String()
		}
		findings = enrichFindings(findings, &statement)
		for i := range findings {
			if findings[i].Location == nil && (statement.Line > 0 || statement.Column > 0) {
				findings[i].Location = &rule.Location{
					Line:   statement.Line,
					Column: statement.Column,
				}
			}
		}

		for i := range gaps {
			gaps[i].Index = idx
			if statement.SensitiveSQL() {
				gaps[i].SQL = statement.PublicSQL()
			} else if gaps[i].SQL == "" {
				gaps[i].SQL = statement.RawSQL
			}
		}
		unsupported = append(unsupported, gaps...)

		for _, id := range eval.AppliedRuleIDs {
			appliedIDs[id] = struct{}{}
		}

		for _, s := range eval.Skipped {
			if _, seen := skippedDedup[s.RuleID]; !seen {
				skippedDedup[s.RuleID] = s
			}
		}

		// Metadata evidence gaps lower an otherwise-complete statement to
		// unverified; they never override a recognized-but-incomplete status.
		if len(eval.EvidenceGaps) > 0 && coverage.Status == report.CoverageComplete {
			coverage.Status = report.CoverageUnverified
		}

		statementResults = append(statementResults, report.StatementResult{
			Index:         idx,
			Kind:          statement.Kind.String(),
			RawSQL:        statement.PublicSQL(),
			NormalizedSQL: statement.PublicNormalizedSQL(),
			Findings:      findings,
			EvidenceGaps:  eval.EvidenceGaps,
			Impact:        reportImpact(statement),
			Coverage:      coverage,
		})
		supportedStatements = append(supportedStatements, statement)
	}

	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	globalFindings, err := registry.EvaluateGlobal(ctx, supportedStatements)
	if err != nil {
		return report.Result{}, err
	}
	globalFindings = enrichFindings(globalFindings, nil)

	result := report.Aggregate(statementResults, globalFindings)
	result.Unsupported = unsupported

	loaded := registry.LoadedStatementRuleCount()
	if loaded > 0 {
		skipped := make([]rule.SkippedRule, 0, len(skippedDedup))
		for _, s := range skippedDedup {
			skipped = append(skipped, s)
		}
		sort.Slice(skipped, func(i, j int) bool {
			return skipped[i].RuleID < skipped[j].RuleID
		})
		result.RuleSummary = &report.RuleSummary{
			Loaded:     loaded,
			Applicable: len(appliedIDs),
			Skipped:    skipped,
		}
	}

	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	return result, nil
}

func reportImpact(statement spec.Statement) *report.Impact {
	if statement.ResourceLimit != nil || statement.DML == nil || statement.DML.Impact == nil {
		return nil
	}

	impact := statement.DML.Impact
	result := &report.Impact{
		EstimatedRows:  impact.EstimatedRows,
		EstimatedRatio: impact.EstimatedRatio,
		RiskLevel:      impact.RiskLevel,
		Confidence:     impact.Confidence,
		Source:         impact.Source,
	}
	if impact.ReasonCodes != nil {
		result.ReasonCodes = append([]string(nil), impact.ReasonCodes...)
	}
	if impact.Notes != nil {
		result.Notes = append([]string(nil), impact.Notes...)
	}
	return result
}

// evaluateRoutineBody audits each statically bounded body member of a
// stored-routine definition through the shared registry. Member findings
// keep their own rule IDs and levels; their locations are translated
// from parent-raw space into file space using the parent's line/column,
// and every finding is tagged with the owning routine and member index
// (T27/#106). Unparseable members surface as unsupported details instead
// of being silently skipped.
func evaluateRoutineBody(ctx context.Context, registry *rule.Registry, parent spec.Statement, parentIndex int) ([]rule.Finding, []spec.UnsupportedDetail, *rule.StatementEvaluation, error) {
	decl := parent.DDL.Routine
	findings := make([]rule.Finding, 0)
	unsupported := make([]spec.UnsupportedDetail, 0)
	combined := &rule.StatementEvaluation{}
	for memberIndex, bs := range decl.Body.Statements {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		inner := bs.Statement
		if inner.Unsupported != nil {
			item := *inner.Unsupported
			item.Index = parentIndex
			item.SQL = inner.RawSQL
			if item.Metadata == nil {
				item.Metadata = map[string]any{}
			}
			item.Metadata["routine"] = decl.Name.Name
			item.Metadata["routine_member"] = memberIndex
			unsupported = append(unsupported, item)
			continue
		}
		eval, err := registry.EvaluateStatementDetailed(ctx, inner)
		if err != nil {
			return nil, nil, nil, err
		}
		combined.AppliedRuleIDs = append(combined.AppliedRuleIDs, eval.AppliedRuleIDs...)
		combined.Skipped = append(combined.Skipped, eval.Skipped...)
		combined.EvidenceGaps = append(combined.EvidenceGaps, eval.EvidenceGaps...)
		for i := range eval.Findings {
			f := eval.Findings[i]
			if f.Location == nil && (inner.Line > 0 || inner.Column > 0) {
				f.Location = &rule.Location{Line: inner.Line, Column: inner.Column}
			}
			if f.Location != nil {
				// Inner positions live in parent-raw space; translate to
				// file space using the parent statement's own origin.
				if f.Location.Line == 1 {
					f.Location.Column += parent.Column - 1
				}
				f.Location.Line += parent.Line - 1
			}
			if f.Metadata == nil {
				f.Metadata = map[string]any{}
			}
			f.Metadata["routine"] = decl.Name.Name
			f.Metadata["routine_member"] = memberIndex
			findings = append(findings, f)
		}
	}
	return findings, unsupported, combined, nil
}
