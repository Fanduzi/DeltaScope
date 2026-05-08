// Package audit orchestrates audit use cases at the application layer.
// input: audit requests carrying SQL text, dialect, optional policy override paths, and optional metadata providers
// output: end-to-end audit results assembled from policy loading, parsing, extraction, metadata enrichment, post-enrichment impact refinement, and rule evaluation
// pos: application service entrypoint for the unified offline/metadata-aware SQL audit use case with preserved statement impact estimates
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	apppolicy "github.com/Fanduzi/DeltaScope/internal/application/policy"
	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	ddlrules "github.com/Fanduzi/DeltaScope/internal/domain/rule/ddl"
	dmlrules "github.com/Fanduzi/DeltaScope/internal/domain/rule/dml"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

var (
	// ErrEmptySQL indicates the request did not include auditable SQL text.
	ErrEmptySQL = errors.New("audit SQL must not be empty")
	// ErrUnknownDialect indicates the request did not specify a supported dialect.
	ErrUnknownDialect = errors.New("audit dialect must be mysql, tidb, or postgresql")
	// ErrUnsupportedStatement indicates at least one parsed statement is recognized but unsupported.
	ErrUnsupportedStatement = errors.New("audit includes unsupported statements")
)

// Request describes one application-level audit invocation.
type Request struct {
	SQL              string
	Dialect          spec.Dialect
	ConfigPath       string
	Schema           string
	MetadataProvider MetadataProvider
	Metadata         *MetadataRequest
}

// Service coordinates the full audit use case.
type Service struct{}

// NewService returns a ready-to-use audit service.
func NewService() Service {
	return Service{}
}

// AuditSQL is the convenience application entrypoint used by outer adapters.
func AuditSQL(ctx context.Context, request Request) (report.Result, error) {
	return NewService().Audit(ctx, request)
}

// Audit executes the full SQL audit flow.
func (s Service) Audit(ctx context.Context, request Request) (report.Result, error) {
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	if strings.TrimSpace(request.SQL) == "" {
		return report.Result{}, ErrEmptySQL
	}
	if request.Dialect != spec.DialectMySQL && request.Dialect != spec.DialectTiDB && request.Dialect != spec.DialectPostgreSQL {
		return report.Result{}, ErrUnknownDialect
	}

	policyCfg, err := apppolicy.Load(request.ConfigPath)
	if err != nil {
		return report.Result{}, fmt.Errorf("load policy: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}

	parsed, err := Parse(ctx, request.SQL, request.Dialect)
	if err != nil {
		if request.Dialect == spec.DialectMySQL || request.Dialect == spec.DialectTiDB {
			if token, ok := possiblePostgreSQLMismatch(request.SQL); ok {
				result := report.Aggregate(nil, []rule.Finding{buildPossiblePostgreSQLMismatchFinding(string(request.Dialect), token)})
				result.Verdict = report.VerdictReview
				return result, err
			}
		}
		return report.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}

	statements, err := Extract(ctx, parsed)
	if err != nil {
		return report.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}

	statements, err = enrichStatementsWithMetadata(ctx, request.Dialect, metadataRequestFor(request), statements)
	if err != nil {
		return report.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	if planner, ok := planEstimatorFor(request).(PlanEstimator); ok {
		statements = attachImpactEstimatesWithPlanner(ctx, planner, statements)
	} else {
		statements = attachImpactEstimates(statements)
	}

	registry, err := buildRegistry(policyCfg)
	if err != nil {
		return report.Result{}, err
	}

	result, err := EvaluateStatements(ctx, registry, statements)
	if err != nil {
		return report.Result{}, err
	}
	if request.Dialect == spec.DialectMySQL || request.Dialect == spec.DialectTiDB {
		if token, ok := possiblePostgreSQLMismatch(request.SQL); ok {
			result = addGlobalFinding(result, buildPossiblePostgreSQLMismatchFinding(string(request.Dialect), token))
		}
	}
	if len(result.Unsupported) > 0 {
		return result, fmt.Errorf("%w: %d item(s)", ErrUnsupportedStatement, len(result.Unsupported))
	}
	return result, nil
}

func metadataRequestFor(request Request) *MetadataRequest {
	legacy := request.Metadata
	schema := strings.TrimSpace(request.Schema)
	provider := request.MetadataProvider

	if legacy != nil {
		if schema == "" {
			schema = strings.TrimSpace(legacy.Schema)
		}
		if provider == nil {
			provider = legacy.Provider
		}
	}
	if schema == "" && provider == nil {
		return nil
	}
	return &MetadataRequest{
		Schema:   schema,
		Provider: provider,
	}
}

func planEstimatorFor(request Request) any {
	if request.MetadataProvider != nil {
		return request.MetadataProvider
	}
	if request.Metadata != nil {
		return request.Metadata.Provider
	}
	return nil
}

func buildRegistry(cfg domainpolicy.Policy) (*rule.Registry, error) {
	registry := rule.NewRegistry()
	if err := ddlrules.Register(registry, cfg); err != nil {
		return nil, fmt.Errorf("register ddl rules: %w", err)
	}
	if err := dmlrules.Register(registry, cfg); err != nil {
		return nil, fmt.Errorf("register dml rules: %w", err)
	}
	return registry, nil
}

func addGlobalFinding(result report.Result, finding rule.Finding) report.Result {
	findings := append(append([]rule.Finding(nil), result.GlobalFindings...), finding)
	reaggregated := report.Aggregate(result.Statements, findings)
	reaggregated.Unsupported = append([]spec.UnsupportedDetail(nil), result.Unsupported...)
	reaggregated.RuleSummary = result.RuleSummary
	return reaggregated
}
