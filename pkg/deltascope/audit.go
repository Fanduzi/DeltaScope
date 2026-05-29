// Package deltascope exposes the public library surface for consumers.
// input: public audit requests carrying SQL text, dialect, optional config path, and optional metadata providers
// output: stable audit results for embedding DeltaScope in tools and agents
// pos: public audit API above the internal application service
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"errors"
	"fmt"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// Dialect identifies the SQL dialect for public callers.
type Dialect string

const (
	DialectMySQL      Dialect = "mysql"
	DialectTiDB       Dialect = "tidb"
	DialectPostgreSQL Dialect = "postgresql"
)

var ErrUnsupportedStatement = errors.New("deltascope audit includes unsupported statements")

// Verdict identifies the final public audit outcome.
type Verdict string

const (
	VerdictPass   Verdict = "pass"
	VerdictReview Verdict = "review"
	VerdictReject Verdict = "reject"
)

// Level identifies the public finding severity.
type Level string

const (
	LevelBlocker Level = "blocker"
	LevelWarning Level = "warning"
	LevelNotice  Level = "notice"
)

// Metadata mirrors the optional domain metadata facts exposed on statements.
type Metadata = spec.Metadata

// InstanceFacts mirror metadata-aware instance facts for public providers.
type InstanceFacts = spec.InstanceFacts

// TableSnapshot mirrors metadata-aware target table snapshots for public providers.
type TableSnapshot = spec.TableSnapshot

// Table mirrors the domain table shape used inside metadata snapshots.
type Table = spec.Table

// Column mirrors the domain column shape used inside metadata snapshots.
type Column = spec.Column

// Index mirrors the domain index shape used inside metadata snapshots.
type Index = spec.Index

// Constraint mirrors the domain constraint shape used inside metadata snapshots.
type Constraint = spec.Constraint

// MetadataProvider supplies optional metadata-aware facts for one public audit request.
type MetadataProvider interface {
	LoadInstanceFacts(ctx context.Context, dialect Dialect, schema string) (*InstanceFacts, error)
	LoadTableSnapshot(ctx context.Context, dialect Dialect, schema string, table string) (*TableSnapshot, error)
}

// PlanEstimateProvider optionally supplies planner-backed DML impact estimates.
type PlanEstimateProvider interface {
	LoadPlanEstimate(ctx context.Context, statement spec.Statement) (*spec.ImpactEstimate, error)
}

// Request describes one public audit invocation.
type Request struct {
	SQL              string
	Dialect          Dialect
	ConfigPath       string
	Schema           string
	MetadataProvider MetadataProvider
}

// Summary captures high-level public audit counts.
type Summary struct {
	Statements int `json:"statements"`
	Blockers   int `json:"blockers"`
	Warnings   int `json:"warnings"`
	Notices    int `json:"notices"`
}

// Location identifies a public source span when available.
type Location struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}

// Explanation is the stable public result-level explanation shape.
type Explanation struct {
	Summary string   `json:"summary,omitempty"`
	Reasons []string `json:"reasons,omitempty"`
}

// ImpactSource identifies the public origin of a statement-level DML impact estimate.
type ImpactSource string

// ImpactRisk identifies the public conservative risk bucket for a DML statement.
type ImpactRisk string

// ImpactConfidence identifies the public estimate-confidence bucket.
type ImpactConfidence string

const (
	ImpactSourceShape    ImpactSource = "shape"
	ImpactSourceMetadata ImpactSource = "metadata"
	ImpactSourcePlan     ImpactSource = "plan"

	ImpactRiskLow     ImpactRisk = "low"
	ImpactRiskMedium  ImpactRisk = "medium"
	ImpactRiskHigh    ImpactRisk = "high"
	ImpactRiskUnknown ImpactRisk = "unknown"

	ImpactConfidenceLow    ImpactConfidence = "low"
	ImpactConfidenceMedium ImpactConfidence = "medium"
	ImpactConfidenceHigh   ImpactConfidence = "high"
)

// Impact is the stable public statement-level DML impact estimate shape.
type Impact struct {
	EstimatedRows  *int64           `json:"estimated_rows,omitempty"`
	EstimatedRatio *float64         `json:"estimated_ratio,omitempty"`
	RiskLevel      ImpactRisk       `json:"risk_level,omitempty"`
	Confidence     ImpactConfidence `json:"confidence,omitempty"`
	Source         ImpactSource     `json:"source,omitempty"`
	ReasonCodes    []string         `json:"reason_codes,omitempty"`
	Notes          []string         `json:"notes,omitempty"`
}

// ExplanationMetadata describes how metadata availability affected a public finding explanation.
type ExplanationMetadata struct {
	Status string `json:"status,omitempty"`
	Note   string `json:"note,omitempty"`
}

// FindingExplanation is the stable public per-finding explanation shape.
type FindingExplanation struct {
	Summary    string               `json:"summary,omitempty"`
	Why        string               `json:"why,omitempty"`
	Risk       string               `json:"risk,omitempty"`
	Suggestion string               `json:"suggestion,omitempty"`
	Metadata   *ExplanationMetadata `json:"metadata,omitempty"`
}

// Finding is the stable public finding shape.
type Finding struct {
	RuleID         string              `json:"rule_id"`
	Level          Level               `json:"level"`
	Message        string              `json:"message"`
	StatementIndex int                 `json:"statement_index,omitempty"`
	StatementKind  string              `json:"statement_kind,omitempty"`
	Location       *Location           `json:"location,omitempty"`
	Suggestion     string              `json:"suggestion,omitempty"`
	Metadata       map[string]any      `json:"metadata,omitempty"`
	Explanation    *FindingExplanation `json:"explanation,omitempty"`
}

// StatementResult stores public findings for a single SQL statement.
type StatementResult struct {
	Index         int          `json:"index"`
	Kind          string       `json:"kind"`
	RawSQL        string       `json:"raw_sql,omitempty"`
	NormalizedSQL string       `json:"normalized_sql,omitempty"`
	Findings      []Finding    `json:"findings,omitempty"`
	Impact        *Impact      `json:"impact,omitempty"`
	Explanation   *Explanation `json:"explanation,omitempty"`
}

// Result is the stable public audit output.
type Result struct {
	Verdict        Verdict                  `json:"verdict"`
	Summary        Summary                  `json:"summary"`
	Statements     []StatementResult        `json:"statements,omitempty"`
	GlobalFindings []Finding                `json:"global_findings,omitempty"`
	Unsupported    []spec.UnsupportedDetail `json:"unsupported,omitempty"`
	Explanation    *Explanation             `json:"explanation,omitempty"`
	Diagnostics    []spec.Diagnostic        `json:"diagnostics,omitempty"`
}

// Audit executes the stable public audit flow.
func Audit(ctx context.Context, request Request) (Result, error) {
	appRequest := appaudit.Request{
		SQL:        request.SQL,
		Dialect:    toDomainDialect(request.Dialect),
		ConfigPath: request.ConfigPath,
		Schema:     request.Schema,
	}
	if request.MetadataProvider != nil {
		appRequest.MetadataProvider = publicMetadataProvider{provider: request.MetadataProvider}
	}

	result, err := appaudit.AuditSQL(ctx, appRequest)
	publicResult := fromDomainResult(result)
	if err != nil {
		if errors.Is(err, appaudit.ErrUnsupportedStatement) {
			return publicResult, fmt.Errorf("%w: %v", ErrUnsupportedStatement, err)
		}
		if len(publicResult.Diagnostics) > 0 {
			return publicResult, err
		}
		return Result{}, err
	}

	return publicResult, nil
}

func toDomainDialect(dialect Dialect) spec.Dialect {
	switch dialect {
	case DialectTiDB:
		return spec.DialectTiDB
	case DialectPostgreSQL:
		return spec.DialectPostgreSQL
	case DialectMySQL, "":
		return spec.DialectMySQL
	default:
		return spec.Dialect(dialect)
	}
}

func fromDomainResult(result report.Result) Result {
	public := Result{
		Verdict:        Verdict(result.Verdict),
		Summary:        Summary(result.Summary),
		Statements:     make([]StatementResult, 0, len(result.Statements)),
		GlobalFindings: make([]Finding, 0, len(result.GlobalFindings)),
		Unsupported:    append([]spec.UnsupportedDetail(nil), result.Unsupported...),
		Explanation:    fromDomainExplanation(result.Explanation),
		Diagnostics:    append([]spec.Diagnostic(nil), result.Diagnostics...),
	}

	for _, stmt := range result.Statements {
		public.Statements = append(public.Statements, StatementResult{
			Index:         stmt.Index,
			Kind:          stmt.Kind,
			RawSQL:        stmt.RawSQL,
			NormalizedSQL: stmt.NormalizedSQL,
			Findings:      fromDomainFindings(stmt.Findings),
			Impact:        fromDomainImpact(stmt.Impact),
			Explanation:   fromDomainExplanation(stmt.Explanation),
		})
	}

	public.GlobalFindings = fromDomainFindings(result.GlobalFindings)
	return public
}

func fromDomainFindings(findings []rule.Finding) []Finding {
	public := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		item := Finding{
			RuleID:         finding.RuleID,
			Level:          Level(finding.Level),
			Message:        finding.Message,
			StatementIndex: finding.StatementIndex,
			StatementKind:  finding.StatementKind,
			Suggestion:     finding.Suggestion,
			Metadata:       cloneMetadataMap(finding.Metadata),
			Explanation:    fromDomainFindingExplanation(finding.Explanation),
		}
		if finding.Location != nil {
			item.Location = &Location{
				Line:   finding.Location.Line,
				Column: finding.Location.Column,
			}
		}
		public = append(public, item)
	}
	return public
}

func fromDomainExplanation(explanation *report.Explanation) *Explanation {
	if explanation == nil {
		return nil
	}
	return &Explanation{
		Summary: explanation.Summary,
		Reasons: append([]string(nil), explanation.Reasons...),
	}
}

func fromDomainImpact(impact *report.Impact) *Impact {
	if impact == nil {
		return nil
	}
	return &Impact{
		EstimatedRows:  cloneInt64Ptr(impact.EstimatedRows),
		EstimatedRatio: cloneFloat64Ptr(impact.EstimatedRatio),
		RiskLevel:      ImpactRisk(impact.RiskLevel),
		Confidence:     ImpactConfidence(impact.Confidence),
		Source:         ImpactSource(impact.Source),
		ReasonCodes:    append([]string(nil), impact.ReasonCodes...),
		Notes:          append([]string(nil), impact.Notes...),
	}
}

func cloneMetadataMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneMetadataValue(value)
	}
	return out
}

func cloneMetadataValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMetadataMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = cloneMetadataValue(typed[i])
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(typed))
		for i := range typed {
			out[i] = cloneMetadataMap(typed[i])
		}
		return out
	case [][]string:
		out := make([][]string, len(typed))
		for i := range typed {
			out[i] = append([]string(nil), typed[i]...)
		}
		return out
	case []string:
		return append([]string(nil), typed...)
	case []int:
		return append([]int(nil), typed...)
	case []int64:
		return append([]int64(nil), typed...)
	case []float64:
		return append([]float64(nil), typed...)
	case []bool:
		return append([]bool(nil), typed...)
	default:
		return value
	}
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInstanceFacts(in *spec.InstanceFacts) *spec.InstanceFacts {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneTableSnapshot(in *spec.TableSnapshot) *spec.TableSnapshot {
	if in == nil {
		return nil
	}
	out := *in
	if in.Table != nil {
		table := *in.Table
		out.Table = &table
	}
	out.Columns = append([]spec.Column(nil), in.Columns...)
	if in.PrimaryKey != nil {
		out.PrimaryKey = cloneIndex(in.PrimaryKey)
	}
	out.Indexes = cloneIndexes(in.Indexes)
	out.Constraints = cloneConstraints(in.Constraints)
	if len(in.Options) > 0 {
		out.Options = make(map[string]string, len(in.Options))
		for key, value := range in.Options {
			out.Options[key] = value
		}
	} else {
		out.Options = nil
	}
	return &out
}

func cloneIndexes(in []spec.Index) []spec.Index {
	if len(in) == 0 {
		return nil
	}
	out := make([]spec.Index, len(in))
	for i := range in {
		out[i] = *cloneIndex(&in[i])
	}
	return out
}

func cloneIndex(in *spec.Index) *spec.Index {
	if in == nil {
		return nil
	}
	out := *in
	out.Columns = append([]string(nil), in.Columns...)
	out.Cardinality = cloneInt64Ptr(in.Cardinality)
	return &out
}

func cloneConstraints(in []spec.Constraint) []spec.Constraint {
	if len(in) == 0 {
		return nil
	}
	out := make([]spec.Constraint, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Columns = append([]string(nil), in[i].Columns...)
	}
	return out
}

func fromDomainFindingExplanation(explanation *rule.FindingExplanation) *FindingExplanation {
	if explanation == nil {
		return nil
	}
	mapped := &FindingExplanation{
		Summary:    explanation.Summary,
		Why:        explanation.Why,
		Risk:       explanation.Risk,
		Suggestion: explanation.Suggestion,
	}
	if explanation.Metadata != nil {
		mapped.Metadata = &ExplanationMetadata{
			Status: explanation.Metadata.Status,
			Note:   explanation.Metadata.Note,
		}
	}
	return mapped
}

type publicMetadataProvider struct {
	provider MetadataProvider
}

type internalIndexOwnerResolver interface {
	ResolveTableForIndex(ctx context.Context, dialect Dialect, schema string, index string) (string, error)
}

func (p publicMetadataProvider) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	if p.provider == nil {
		return nil, nil
	}
	instanceFacts, err := p.provider.LoadInstanceFacts(ctx, Dialect(dialect), schema)
	if err != nil {
		return nil, err
	}
	return cloneInstanceFacts(instanceFacts), nil
}

func (p publicMetadataProvider) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	if p.provider == nil {
		return nil, nil
	}
	snapshot, err := p.provider.LoadTableSnapshot(ctx, Dialect(dialect), schema, table)
	if err != nil {
		return nil, err
	}
	return cloneTableSnapshot(snapshot), nil
}

func (p publicMetadataProvider) ResolveTableForIndex(ctx context.Context, dialect spec.Dialect, schema string, index string) (string, error) {
	if p.provider == nil {
		return "", nil
	}
	resolver, ok := p.provider.(internalIndexOwnerResolver)
	if !ok {
		return "", nil
	}
	return resolver.ResolveTableForIndex(ctx, Dialect(dialect), schema, index)
}

func (p publicMetadataProvider) LoadPlanEstimate(ctx context.Context, statement spec.Statement) (*spec.ImpactEstimate, error) {
	if p.provider == nil {
		return nil, nil
	}
	provider, ok := p.provider.(PlanEstimateProvider)
	if !ok {
		return nil, nil
	}
	estimate, err := provider.LoadPlanEstimate(ctx, statement)
	if err != nil {
		return nil, err
	}
	if estimate == nil {
		return nil, nil
	}
	cloned := *estimate
	cloned.EstimatedRows = cloneInt64Ptr(estimate.EstimatedRows)
	cloned.EstimatedRatio = cloneFloat64Ptr(estimate.EstimatedRatio)
	cloned.ReasonCodes = append([]string(nil), estimate.ReasonCodes...)
	cloned.Notes = append([]string(nil), estimate.Notes...)
	return &cloned, nil
}

func (p publicMetadataProvider) ResolveObject(ctx context.Context, dialect spec.Dialect, request spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if p.provider == nil {
		return &spec.ObjectSnapshot{
			Schema: request.Schema,
			Type:   request.Type,
			Name:   request.Name,
			Status: spec.MetadataStatusUnavailable,
		}, nil
	}
	type objectResolver interface {
		ResolveObject(context.Context, spec.Dialect, spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error)
	}
	resolver, ok := p.provider.(objectResolver)
	if !ok {
		return &spec.ObjectSnapshot{
			Schema: request.Schema,
			Type:   request.Type,
			Name:   request.Name,
			Status: spec.MetadataStatusUnavailable,
		}, nil
	}
	return resolver.ResolveObject(ctx, dialect, request)
}
