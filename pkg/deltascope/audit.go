// Package deltascope exposes the public library surface for consumers.
// input: public audit requests carrying SQL text, dialect, optional config path, and optional metadata providers
// output: stable audit results for embedding DeltaScope in tools and agents
// pos: public audit API above the internal application service
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// Dialect identifies the SQL dialect for public callers.
type Dialect string

const (
	DialectMySQL Dialect = "mysql"
	DialectTiDB  Dialect = "tidb"
)

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

// Finding is the stable public finding shape.
type Finding struct {
	RuleID         string         `json:"rule_id"`
	Level          Level          `json:"level"`
	Message        string         `json:"message"`
	StatementIndex int            `json:"statement_index,omitempty"`
	StatementKind  string         `json:"statement_kind,omitempty"`
	Location       *Location      `json:"location,omitempty"`
	Suggestion     string         `json:"suggestion,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// StatementResult stores public findings for a single SQL statement.
type StatementResult struct {
	Index         int       `json:"index"`
	Kind          string    `json:"kind"`
	RawSQL        string    `json:"raw_sql,omitempty"`
	NormalizedSQL string    `json:"normalized_sql,omitempty"`
	Findings      []Finding `json:"findings,omitempty"`
}

// Result is the stable public audit output.
type Result struct {
	Verdict        Verdict           `json:"verdict"`
	Summary        Summary           `json:"summary"`
	Statements     []StatementResult `json:"statements,omitempty"`
	GlobalFindings []Finding         `json:"global_findings,omitempty"`
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
	if err != nil {
		return Result{}, err
	}

	return fromDomainResult(result), nil
}

func toDomainDialect(dialect Dialect) spec.Dialect {
	switch dialect {
	case DialectTiDB:
		return spec.DialectTiDB
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
	}

	for _, stmt := range result.Statements {
		public.Statements = append(public.Statements, StatementResult{
			Index:         stmt.Index,
			Kind:          stmt.Kind,
			RawSQL:        stmt.RawSQL,
			NormalizedSQL: stmt.NormalizedSQL,
			Findings:      fromDomainFindings(stmt.Findings),
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
			Metadata:       finding.Metadata,
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

type publicMetadataProvider struct {
	provider MetadataProvider
}

func (p publicMetadataProvider) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	if p.provider == nil {
		return nil, nil
	}
	return p.provider.LoadInstanceFacts(ctx, Dialect(dialect), schema)
}

func (p publicMetadataProvider) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	if p.provider == nil {
		return nil, nil
	}
	return p.provider.LoadTableSnapshot(ctx, Dialect(dialect), schema, table)
}
