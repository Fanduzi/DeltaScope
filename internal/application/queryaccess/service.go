// Package queryaccess provides the application service for query access analysis.
// input: SQL text, dialect, mode, default schema, and optional schema resolver
// output: domain-typed query access results with optional metadata resolution
// pos: application orchestration layer for query access analysis
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"errors"
	"fmt"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// ErrExtractionFailed indicates query access extraction failed without exposing SQL text.
var ErrExtractionFailed = errors.New("query access extraction failed")

// Service orchestrates query access analysis.
type Service struct{}

// Analyze performs query access analysis with optional metadata resolution.
// When SchemaResolver is nil, wildcards and unqualified columns remain unresolved.
func (s *Service) Analyze(ctx context.Context, req QueryAccessRequest) (QueryAccessResult, error) {
	if err := ctx.Err(); err != nil {
		return QueryAccessResult{}, fmt.Errorf("analyze cancelled: %w", err)
	}

	extracted, err := extractByDialect(ctx, req)
	if err != nil {
		return QueryAccessResult{}, ErrExtractionFailed
	}

	if req.SchemaResolver != nil {
		extracted.DomainResult = resolveMetadata(ctx, req.SchemaResolver, req.Dialect, req.DefaultSchema, extracted.DomainResult)
	}

	if hasFunctionCallReasonCode(extracted.DomainResult.ReasonCodes) {
		extracted.DomainResult.ReasonCodes = append(extracted.DomainResult.ReasonCodes, domain.ReasonFunctionEffect)
	}

	// Build requirements based on mode
	reqs, warnings, _, reqErr := buildRequirements(
		extracted.DomainResult.Mode,
		extracted.DomainResult.Relations,
		extracted.DomainResult.ReferencedColumns,
		extracted.DomainResult.Outputs,
		extracted.DomainResult.Unresolved,
	)
	if reqErr != nil {
		return QueryAccessResult{}, fmt.Errorf("build requirements: %w", reqErr)
	}
	extracted.DomainResult.Requirements = reqs
	extracted.DomainResult.Warnings = append(extracted.DomainResult.Warnings, warnings...)

	extracted.DomainResult.Relations = domain.SortRelations(extracted.DomainResult.Relations)
	extracted.DomainResult.ReferencedColumns = domain.SortColumns(extracted.DomainResult.ReferencedColumns)
	extracted.DomainResult.Requirements = domain.SortRequirements(extracted.DomainResult.Requirements)
	extracted.DomainResult.Outputs = domain.SortOutputs(extracted.DomainResult.Outputs)
	extracted.DomainResult.Unresolved = domain.SortUnresolved(extracted.DomainResult.Unresolved)
	extracted.DomainResult.ReasonCodes = domain.SortReasonCodes(extracted.DomainResult.ReasonCodes)
	extracted.DomainResult.Warnings = domain.SortWarningCodes(extracted.DomainResult.Warnings)

	if err := domain.ValidateResult(&extracted.DomainResult); err != nil {
		return QueryAccessResult{}, fmt.Errorf("invalid result: %w", err)
	}

	return extracted, nil
}

func extractByDialect(ctx context.Context, req QueryAccessRequest) (QueryAccessResult, error) {
	switch req.Dialect {
	case "mysql", "tidb":
		return ExtractTiDBQueryAccess(ctx, req)
	case "postgresql":
		return AnalyzePostgreSQL(ctx, req)
	default:
		return QueryAccessResult{}, fmt.Errorf("unsupported dialect: %q", req.Dialect)
	}
}

func hasFunctionCallReasonCode(codes []domain.ReasonCode) bool {
	for _, code := range codes {
		if code == "function_call" {
			return true
		}
	}
	return false
}
