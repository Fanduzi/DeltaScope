// Package audit orchestrates audit use cases at the application layer.
// input: audit requests carrying SQL text, dialect, optional target_version, optional policy override paths, optional metadata providers, and shared input normalization
// output: end-to-end audit results assembled from policy loading, parsing, extraction, metadata enrichment, rule evaluation, a resolved canonical version identity, and a review floor for partial parser failures and structured unsupported statements
// pos: application service entrypoint for the unified offline/metadata-aware SQL audit use case with preserved statement impact estimates and target/observed version reconciliation
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	application "github.com/Fanduzi/DeltaScope/internal/application"
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
	// ErrInvalidTargetVersion indicates target_version did not match [v]MAJOR.MINOR.PATCH.
	ErrInvalidTargetVersion = spec.ErrInvalidTargetVersion
	// ErrTargetVersionMismatch indicates the caller-supplied target_version
	// contradicts the observed live-server version and cannot be honored.
	ErrTargetVersionMismatch = errors.New("audit target_version does not match the observed server version")
	// ErrDialectProductMismatch indicates the requested audit dialect conflicts
	// with the product independently derived from the observed server banner
	// (for example dialect=tidb against a MySQL banner). It is an input error:
	// the caller-declared product can never override observed identity.
	ErrDialectProductMismatch = errors.New("audit dialect does not match the observed server product")
)

// Request describes one application-level audit invocation.
type Request struct {
	SQL              string
	Dialect          spec.Dialect
	ConfigPath       string
	Schema           string
	MetadataProvider MetadataProvider
	Metadata         *MetadataRequest
	// TargetVersion optionally pins the audited server version as strict
	// [v]MAJOR.MINOR.PATCH. Empty means no version was supplied — never
	// defaulted. Online it acts only as a constraint checked against the
	// observed identity; offline it is the version fact itself.
	TargetVersion string
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
	sql := application.NormalizeSQLInput(request.SQL)
	if strings.TrimSpace(sql) == "" {
		return report.Result{}, ErrEmptySQL
	}
	if request.Dialect != spec.DialectMySQL && request.Dialect != spec.DialectTiDB && request.Dialect != spec.DialectPostgreSQL {
		return report.Result{}, ErrUnknownDialect
	}
	targetVersion, err := parseAuditTargetVersion(request.Dialect, request.TargetVersion)
	if err != nil {
		return report.Result{}, err
	}

	policyCfg, err := apppolicy.Load(request.ConfigPath)
	if err != nil {
		return report.Result{}, fmt.Errorf("load policy: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}

	parsed, parseErr := parseSQL(ctx, sql, request.Dialect)
	if parseErr != nil && len(parsed.failures) == 0 {
		if request.Dialect == spec.DialectMySQL || request.Dialect == spec.DialectTiDB {
			if token, ok := possiblePostgreSQLMismatch(sql); ok {
				result := report.Aggregate(nil, []rule.Finding{buildPossiblePostgreSQLMismatchFinding(string(request.Dialect), token)})
				result.Verdict = report.VerdictReview
				result.Coverage = report.Coverage{Status: report.CoverageIncomplete}
				return result, parseErr
			}
		}
		var pgCap *PostgreSQLCapabilityBoundaryError
		if errors.As(parseErr, &pgCap) {
			return report.Result{}, parseErr
		}
		return report.Result{
			Coverage:    report.Coverage{Status: report.CoverageIncomplete},
			Diagnostics: []spec.Diagnostic{newParserErrorDiagnosticWithGuidance(request.Dialect, sql)},
		}, errParserUnsupported
	}
	if len(parsed.Statements) == 0 && len(parsed.failures) > 0 {
		result := report.Result{Coverage: report.Coverage{Status: report.CoverageIncomplete}}
		if request.Dialect == spec.DialectMySQL || request.Dialect == spec.DialectTiDB {
			for _, failure := range parsed.failures {
				if token, ok := possiblePostgreSQLMismatch(failure.RawSQL); ok {
					result = addGlobalFinding(result, buildPossiblePostgreSQLMismatchFinding(string(request.Dialect), token))
					result.Verdict = report.VerdictReview
				}
			}
		}
		result.Diagnostics = parserFailureDiagnostics(parsed.failures, request.Dialect)
		return result, errParserUnsupported
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

	statements, err = enrichStatementsWithMetadata(ctx, request.Dialect, metadataRequestFor(request, targetVersion), statements, parsed.failures)
	if err != nil {
		return report.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	if planner, ok := planEstimatorFor(request).(PlanEstimator); ok {
		statements = attachImpactEstimatesWithPlanner(ctx, planner, statements)
	} else {
		statements = attachImpactEstimates(ctx, statements)
	}

	registry, err := buildRegistry(policyCfg)
	if err != nil {
		return report.Result{}, err
	}

	result, err := EvaluateStatements(ctx, registry, statements)
	if err != nil {
		return report.Result{}, err
	}
	result.Version = resolvedVersionIdentity(statements)
	if request.Dialect == spec.DialectMySQL || request.Dialect == spec.DialectTiDB {
		for _, failure := range parsed.failures {
			if token, ok := possiblePostgreSQLMismatch(failure.RawSQL); ok {
				result = addGlobalFinding(result, buildPossiblePostgreSQLMismatchFinding(string(request.Dialect), token))
			}
		}
	}
	if request.Dialect == spec.DialectMySQL {
		for _, statement := range statements {
			if statementHasMySQLUnsupportedReturning(statement) {
				result = addGlobalFinding(result, buildMySQLReturningUnsupportedFinding(string(request.Dialect)))
				break
			}
		}
	}
	if len(parsed.failures) > 0 {
		result.Diagnostics = append(result.Diagnostics, parserFailureDiagnostics(parsed.failures, request.Dialect)...)
		result.Coverage = report.Coverage{Status: report.CoverageIncomplete}
		if result.Verdict == report.VerdictPass {
			result.Verdict = report.VerdictReview
		}
		if len(result.Unsupported) > 0 {
			result.Diagnostics = append(result.Diagnostics, newUnsupportedStatementDiagnostic(request.Dialect))
		}
		return result, errParserUnsupported
	}
	if len(result.Unsupported) > 0 {
		result.Diagnostics = append(result.Diagnostics, newUnsupportedStatementDiagnostic(request.Dialect))
		if result.Verdict == report.VerdictPass {
			result.Verdict = report.VerdictReview
		}
		return result, fmt.Errorf("%w: %d item(s)", ErrUnsupportedStatement, len(result.Unsupported))
	}
	return result, nil
}

func parserFailureDiagnostics(failures []parseFailure, dialect spec.Dialect) []spec.Diagnostic {
	diagnostics := make([]spec.Diagnostic, 0, len(failures))
	for _, failure := range failures {
		diagnostic := newParserErrorDiagnosticWithGuidance(dialect, failure.RawSQL)
		diagnostic.Line = failure.Line
		diagnostic.Column = failure.Column
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics
}

// parseAuditTargetVersion validates the optional target_version and stamps it
// with the dialect-implied product. Syntactically valid versions outside the
// validated series are returned with ValidatedRange=false rather than an
// error — malformed strings are the only input error here.
func parseAuditTargetVersion(dialect spec.Dialect, raw string) (*spec.VersionIdentity, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parsed, err := spec.ParseTargetVersion(raw)
	if err != nil {
		return nil, err
	}
	identity := parsed.WithTargetSource(spec.VersionProductForDialect(dialect))
	return &identity, nil
}

// resolvedVersionIdentity returns the shared version fact attached during
// enrichment — identical across statements, so the first populated one wins.
func resolvedVersionIdentity(statements []spec.Statement) *spec.VersionIdentity {
	for i := range statements {
		if statements[i].Metadata != nil && statements[i].Metadata.Version != nil {
			return statements[i].Metadata.Version
		}
	}
	return nil
}

func metadataRequestFor(request Request, targetVersion *spec.VersionIdentity) *MetadataRequest {
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
	if schema == "" && provider == nil && targetVersion == nil {
		return nil
	}
	return &MetadataRequest{
		Schema:        schema,
		Provider:      provider,
		TargetVersion: targetVersion,
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
	findings := make([]rule.Finding, 0, len(result.GlobalFindings)+1)
	findings = append(findings, result.GlobalFindings...)
	findings = append(findings, finding)
	reaggregated := report.Aggregate(result.Statements, findings)
	reaggregated.Unsupported = append([]spec.UnsupportedDetail(nil), result.Unsupported...)
	reaggregated.RuleSummary = result.RuleSummary
	reaggregated.Version = result.Version
	return reaggregated
}
