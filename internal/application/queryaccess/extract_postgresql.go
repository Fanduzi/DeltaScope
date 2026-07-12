//go:build postgresql

// Package queryaccess provides the PostgreSQL query access application adapter.
// input: QueryAccessRequest with SQL, dialect, mode, and default schema
// output: QueryAccessResult wrapping the domain Result
// pos: application adapter for PostgreSQL query access analysis
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"fmt"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
	pgparser "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/postgresql"
)

// AnalyzePostgreSQL extracts query access facts from PostgreSQL SQL and converts to domain Result.
func AnalyzePostgreSQL(ctx context.Context, req QueryAccessRequest) (QueryAccessResult, error) {
	mode := domain.NormalizeMode(domain.Mode(req.Mode))
	if err := domain.ValidateMode(mode); err != nil {
		return QueryAccessResult{}, fmt.Errorf("invalid mode: %w", err)
	}

	extractor := &pgparser.QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(ctx, req.SQL, req.Dialect, req.DefaultSchema)
	if err != nil {
		return QueryAccessResult{}, fmt.Errorf("extract query access: %w", err)
	}

	result := domain.Result{
		Dialect:            req.Dialect,
		Mode:               mode,
		ReadClassification: domain.ReadClassification(facts.ReadClassification),
		Admission:          domain.IndeterminateAdmission,
	}

	result.Relations = make([]domain.RelationReference, 0, len(facts.Relations))
	for _, r := range facts.Relations {
		result.Relations = append(result.Relations, domain.RelationReference{
			Schema:             r.Schema,
			Name:               r.Name,
			Alias:              r.Alias,
			Kind:               domain.RelationKind(r.Kind),
			PermissionRequired: r.Kind == "table" || r.Kind == "view",
		})
	}

	result.ReferencedColumns = make([]domain.ColumnReference, 0, len(facts.ColumnReferences))
	for _, c := range facts.ColumnReferences {
		usages := make([]domain.UsageContext, 0, len(c.Usages))
		for _, u := range c.Usages {
			usages = append(usages, domain.UsageContext(u))
		}
		result.ReferencedColumns = append(result.ReferencedColumns, domain.ColumnReference{
			Table:  c.Table,
			Column: c.Column,
			Usages: usages,
		})
	}

	result.Outputs = make([]domain.OutputColumn, 0, len(facts.Outputs))
	for _, o := range facts.Outputs {
		result.Outputs = append(result.Outputs, domain.OutputColumn{
			Name:    o.Name,
			Sources: o.Sources,
		})
	}

	result.Unresolved = make([]domain.Unresolved, 0, len(facts.Unresolved))
	for _, u := range facts.Unresolved {
		result.Unresolved = append(result.Unresolved, domain.Unresolved{
			Reference: u.Reference,
			Reason:    domain.ReasonCode(u.Reason),
		})
	}

	// Map parser presence-only unproven effect codes. Do not accept caller-supplied
	// reason codes (request has no such field) and never embed SQL/effect text.
	result.ReasonCodes = convertReasonCodes(facts.ReasonCodes)

	// Internal candidates only: not copied onto domain.Result (public contract).
	// Names/paths stay application-internal for future resolver input.
	return QueryAccessResult{
		DomainResult:     result,
		EffectCandidates: mapEffectCandidates(facts.EffectCandidates),
	}, nil
}

func mapEffectCandidates(in []pgparser.EffectCandidate) []EffectCandidate {
	if len(in) == 0 {
		return nil
	}
	out := make([]EffectCandidate, 0, len(in))
	for _, c := range in {
		kinds := make([]string, 0, len(c.OperandKinds))
		for _, k := range c.OperandKinds {
			kinds = append(kinds, string(k))
		}
		out = append(out, EffectCandidate{
			Kind:           EffectCandidateKind(c.Kind),
			Ordinal:        c.Ordinal,
			NamePath:       append([]string(nil), c.NamePath...),
			ExplicitSchema: c.ExplicitSchema,
			Arity:          c.Arity,
			OperandKinds:   kinds,
			IsAggregate:    c.IsAggregate,
			HasWindow:      c.HasWindow,
			HasFilter:      c.HasFilter,
			TargetTypePath: append([]string(nil), c.TargetTypePath...),
		})
	}
	return out
}
