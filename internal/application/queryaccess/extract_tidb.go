// Package queryaccess provides TiDB query access extraction bridging infrastructure facts to domain types.
// input: SQL text, dialect, mode, default schema, and optional schema resolver
// output: domain-typed query access results for transport adapters
// pos: application adapter bridging TiDB infrastructure query access facts to domain query access types
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"fmt"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
	tidbparser "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/tidb"
)

// ExtractTiDBQueryAccess extracts query access facts from TiDB SQL and converts to domain types.
func ExtractTiDBQueryAccess(ctx context.Context, req QueryAccessRequest) (QueryAccessResult, error) {
	if err := ctx.Err(); err != nil {
		return QueryAccessResult{}, fmt.Errorf("extract cancelled: %w", err)
	}

	extractor := &tidbparser.QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(ctx, req.SQL, req.Dialect, req.DefaultSchema)
	if err != nil {
		return QueryAccessResult{}, fmt.Errorf("extract query access: %w", err)
	}

	mode := domain.NormalizeMode(domain.Mode(req.Mode))

	result := domain.Result{
		Dialect:            req.Dialect,
		Mode:               mode,
		ReadClassification: domain.ReadClassification(facts.ReadClassification),
		Relations:          convertRelations(facts.Relations),
		ReferencedColumns:  convertColumns(facts.ColumnReferences),
		Outputs:            convertOutputs(facts.Outputs),
		Unresolved:         convertUnresolved(facts.Unresolved),
	}

	result.ReasonCodes = convertReasonCodes(facts.ReasonCodes)

	result.Admission = computeAdmission(result.ReadClassification)

	return QueryAccessResult{
		DomainResult:     result,
		EffectCandidates: mapTiDBEffectCandidates(facts.EffectCandidates),
	}, nil
}

func mapTiDBEffectCandidates(in []tidbparser.EffectCandidate) []EffectCandidate {
	if len(in) == 0 {
		return nil
	}
	out := make([]EffectCandidate, 0, len(in))
	for _, candidate := range in {
		mapped := EffectCandidate{
			Kind:                 EffectCandidateKind(candidate.Kind),
			Ordinal:              candidate.Ordinal,
			NamePath:             append([]string(nil), candidate.NamePath...),
			OriginalNamePath:     append([]string(nil), candidate.OriginalNamePath...),
			ExplicitSchema:       candidate.ExplicitSchema,
			IsQuoted:             candidate.IsQuoted,
			Canonical:            candidate.Canonical,
			Ambiguous:            candidate.Ambiguous,
			ParserClassification: candidate.ParserClassification,
			UnqualifiedRelation:  candidate.UnqualifiedRelation,
			Arity:                candidate.Arity,
			IsAggregate:          candidate.IsAggregate,
			HasWindow:            candidate.HasWindow,
			HasFilter:            candidate.HasFilter,
			HasDistinct:          candidate.HasDistinct,
			HasAggOrder:          candidate.HasAggOrder,
			HasWithinGroup:       candidate.HasWithinGroup,
			HasFrame:             candidate.HasFrame,
			HasNamedWindow:       candidate.HasNamedWindow,
			HasWindowPartition:   candidate.HasWindowPartition,
			HasWindowOrder:       candidate.HasWindowOrder,
			TargetTypePath:       append([]string(nil), candidate.TargetTypePath...),
		}
		for _, kind := range candidate.OperandKinds {
			mapped.OperandKinds = append(mapped.OperandKinds, string(kind))
		}
		for _, kind := range candidate.WindowPartitionKinds {
			mapped.WindowPartitionKinds = append(mapped.WindowPartitionKinds, string(kind))
		}
		for _, kind := range candidate.WindowOrderKinds {
			mapped.WindowOrderKinds = append(mapped.WindowOrderKinds, string(kind))
		}
		for _, kind := range candidate.WindowFrameKinds {
			mapped.WindowFrameKinds = append(mapped.WindowFrameKinds, string(kind))
		}
		for _, ref := range candidate.OperandColumnRefs {
			mapped.OperandColumnRefs = append(mapped.OperandColumnRefs, OperandColumnRef{Schema: ref.Schema, Table: ref.Table, Column: ref.Column})
		}
		for _, ref := range candidate.WindowPartitionColumnRefs {
			mapped.WindowPartitionColumnRefs = append(mapped.WindowPartitionColumnRefs, OperandColumnRef{Schema: ref.Schema, Table: ref.Table, Column: ref.Column})
		}
		for _, ref := range candidate.WindowOrderColumnRefs {
			mapped.WindowOrderColumnRefs = append(mapped.WindowOrderColumnRefs, OperandColumnRef{Schema: ref.Schema, Table: ref.Table, Column: ref.Column})
		}
		out = append(out, mapped)
	}
	return out
}

func convertRelations(facts []tidbparser.RelationFact) []domain.RelationReference {
	if len(facts) == 0 {
		return nil
	}
	refs := make([]domain.RelationReference, 0, len(facts))
	for _, f := range facts {
		refs = append(refs, domain.RelationReference{
			Schema:             f.Schema,
			Name:               f.Name,
			Alias:              f.Alias,
			Kind:               domain.RelationKind(f.Kind),
			PermissionRequired: f.Kind == "table" || f.Kind == "view",
		})
	}
	return refs
}

func convertColumns(facts []tidbparser.ColumnFact) []domain.ColumnReference {
	if len(facts) == 0 {
		return nil
	}
	refs := make([]domain.ColumnReference, 0, len(facts))
	for _, f := range facts {
		usages := make([]domain.UsageContext, 0, len(f.Usages))
		for _, u := range f.Usages {
			usages = append(usages, domain.UsageContext(u))
		}
		refs = append(refs, domain.ColumnReference{
			Schema: f.Schema,
			Table:  f.Table,
			Column: f.Column,
			Usages: domain.DeduplicateUsages(usages),
		})
	}
	return refs
}

func convertOutputs(facts []tidbparser.OutputFact) []domain.OutputColumn {
	if len(facts) == 0 {
		return nil
	}
	outputs := make([]domain.OutputColumn, 0, len(facts))
	for _, f := range facts {
		outputs = append(outputs, domain.OutputColumn{
			Name:    f.Name,
			Sources: f.Sources,
		})
	}
	return outputs
}

func convertUnresolved(facts []tidbparser.UnresolvedFact) []domain.Unresolved {
	if len(facts) == 0 {
		return nil
	}
	unresolved := make([]domain.Unresolved, 0, len(facts))
	for _, f := range facts {
		unresolved = append(unresolved, domain.Unresolved{
			Reference: f.Reference,
			Reason:    domain.ReasonCode(f.Reason),
		})
	}
	return unresolved
}

func convertReasonCodes(codes []string) []domain.ReasonCode {
	if len(codes) == 0 {
		return nil
	}
	result := make([]domain.ReasonCode, 0, len(codes))
	for _, c := range codes {
		result = append(result, domain.ReasonCode(c))
	}
	return result
}

func computeAdmission(classification domain.ReadClassification) domain.Admission {
	switch classification {
	case domain.ReadOnly:
		return domain.Admissible
	case domain.NotReadOnly:
		return domain.Rejected
	default:
		return domain.IndeterminateAdmission
	}
}
