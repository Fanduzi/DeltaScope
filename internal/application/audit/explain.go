// Package audit enriches evaluated findings with shared explanation metadata.
// input: rule findings, shipped catalog entries, and optional statement metadata context
// output: additive per-finding explanation data without changing verdict semantics
// pos: application explanation enrichment between evaluation and report aggregation
// note: if this file changes, update this header and module README.md.
package audit

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichFindings(findings []rule.Finding, statement *spec.Statement) []rule.Finding {
	if len(findings) == 0 {
		return findings
	}

	enriched := make([]rule.Finding, len(findings))
	copy(enriched, findings)
	for i := range enriched {
		enriched[i].Explanation = mergeFindingExplanation(enriched[i], explanationForFinding(enriched[i], statement))
	}
	return enriched
}

func explanationForFinding(finding rule.Finding, statement *spec.Statement) *rule.FindingExplanation {
	entry, ok := catalog.Lookup(finding.RuleID)
	if !ok {
		return &rule.FindingExplanation{
			Summary:    finding.Message,
			Suggestion: finding.Suggestion,
		}
	}

	explanation := &rule.FindingExplanation{
		Summary:    entry.Summary,
		Why:        entry.Why,
		Risk:       entry.Risk,
		Suggestion: entry.Suggestion,
	}
	if entry.MetadataNotes == nil {
		return explanation
	}

	explanation.Metadata = &rule.ExplanationMetadata{}
	if metadataAvailable(statement, entry.MetadataNotes.Kinds) {
		explanation.Metadata.Status = "available"
		explanation.Metadata.Note = entry.MetadataNotes.Required
		return explanation
	}
	explanation.Metadata.Status = "limited"
	explanation.Metadata.Note = entry.MetadataNotes.Missing
	return explanation
}

func mergeFindingExplanation(finding rule.Finding, generated *rule.FindingExplanation) *rule.FindingExplanation {
	if generated == nil {
		return finding.Explanation
	}

	merged := *generated
	if finding.Explanation != nil {
		if finding.Explanation.Summary != "" {
			merged.Summary = finding.Explanation.Summary
		}
		if finding.Explanation.Why != "" {
			merged.Why = finding.Explanation.Why
		}
		if finding.Explanation.Risk != "" {
			merged.Risk = finding.Explanation.Risk
		}
		if finding.Explanation.Suggestion != "" {
			merged.Suggestion = finding.Explanation.Suggestion
		}
		if finding.Explanation.Metadata != nil {
			merged.Metadata = finding.Explanation.Metadata
		}
	}
	if finding.Suggestion != "" {
		merged.Suggestion = finding.Suggestion
	}
	return &merged
}

func metadataAvailable(statement *spec.Statement, kinds []catalog.MetadataKind) bool {
	if statement == nil || statement.Metadata == nil {
		return false
	}
	for _, kind := range kinds {
		switch kind {
		case catalog.MetadataKindSchema:
			if statement.Metadata.Schema == "" {
				return false
			}
		case catalog.MetadataKindTargetTable:
			if statement.Metadata.TargetTable == nil {
				return false
			}
		case catalog.MetadataKindInstance:
			if statement.Metadata.Instance == nil {
				return false
			}
		default:
			return false
		}
	}
	return true
}
