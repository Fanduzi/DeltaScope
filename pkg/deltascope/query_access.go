// Package deltascope exposes the public library surface for consumers.
// input: public query access requests carrying SQL text, dialect, mode, and optional schema resolver
// output: stable query access analysis results for embedding DeltaScope in tools and agents
// pos: public query access API above the internal application service
// note: if this file changes, update this header and module README.md.
//
// Defense in Depth: Query access analysis is one layer in a defense-in-depth authorization strategy.
// It supplements, but does not replace, database authorization, grant evaluation, row-level security,
// and audit logging. Always pair this analysis with proper authentication and authorization checks.
package deltascope

import (
	"context"
	"errors"
	"fmt"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	domainqa "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// QueryAccessMode controls which column references become requirements.
type QueryAccessMode string

const (
	// QueryAccessModeStrict requires all referenced columns to be authorized.
	QueryAccessModeStrict QueryAccessMode = "strict"
	// QueryAccessModeProjectionOnly requires only projected columns to be authorized.
	QueryAccessModeProjectionOnly QueryAccessMode = "projection_only"
)

// QueryAccessReadClassification describes whether SQL is demonstrably read-only.
type QueryAccessReadClassification string

const (
	// QueryAccessReadOnly indicates the statement contains no write operations.
	QueryAccessReadOnly QueryAccessReadClassification = "read_only"
	// QueryAccessNotReadOnly indicates the statement contains at least one write operation.
	QueryAccessNotReadOnly QueryAccessReadClassification = "not_read_only"
	// QueryAccessIndeterminate indicates the read-only status could not be determined.
	QueryAccessIndeterminate QueryAccessReadClassification = "indeterminate"
)

// QueryAccessAdmission describes whether SQL is eligible for caller authorization.
type QueryAccessAdmission string

const (
	// QueryAccessAdmissible indicates the statement is eligible for authorization checks.
	QueryAccessAdmissible QueryAccessAdmission = "admissible"
	// QueryAccessRejected indicates the statement is not eligible for authorization checks.
	QueryAccessRejected QueryAccessAdmission = "rejected"
	// QueryAccessIndeterminateAdmission indicates the admission status could not be determined.
	QueryAccessIndeterminateAdmission QueryAccessAdmission = "indeterminate"
)

// QueryAccessRelationKind describes the type of relation reference.
type QueryAccessRelationKind string

const (
	// QueryAccessRelationTable indicates a base table reference.
	QueryAccessRelationTable QueryAccessRelationKind = "table"
	// QueryAccessRelationView indicates a view reference.
	QueryAccessRelationView QueryAccessRelationKind = "view"
	// QueryAccessRelationCTE indicates a common table expression reference.
	QueryAccessRelationCTE QueryAccessRelationKind = "cte"
	// QueryAccessRelationDerived indicates a derived table (subquery) reference.
	QueryAccessRelationDerived QueryAccessRelationKind = "derived"
)

// QueryAccessSchemaResolver resolves relation metadata for name resolution.
type QueryAccessSchemaResolver interface {
	ResolveRelation(ctx context.Context, dialect, schema, name string) (QueryAccessRelationSchema, error)
}

// QueryAccessRelationSchema contains metadata about a relation for resolution.
type QueryAccessRelationSchema struct {
	Schema  string
	Name    string
	Kind    string
	Columns []QueryAccessColumnSchema
	IsView  bool
}

// QueryAccessColumnSchema contains metadata about a column.
type QueryAccessColumnSchema struct {
	Name    string
	Ordinal int
}

// QueryAccessRequest is the input for query access analysis.
type QueryAccessRequest struct {
	SQL            string
	Dialect        Dialect
	Mode           QueryAccessMode
	DefaultSchema  string
	SchemaResolver QueryAccessSchemaResolver // optional
}

// QueryAccessResult is the output of query access analysis.
type QueryAccessResult struct {
	Dialect            string                         `json:"dialect"`
	Mode               QueryAccessMode                `json:"mode"`
	ReadClassification QueryAccessReadClassification  `json:"read_classification"`
	Admission          QueryAccessAdmission           `json:"admission"`
	ReasonCodes        []string                       `json:"reason_codes,omitempty"`
	Relations          []QueryAccessRelationReference `json:"relations,omitempty"`
	ReferencedColumns  []QueryAccessColumnReference   `json:"referenced_columns,omitempty"`
	Outputs            []QueryAccessOutputColumn      `json:"outputs,omitempty"`
	Requirements       []QueryAccessRequirement       `json:"requirements,omitempty"`
	Unresolved         []QueryAccessUnresolved        `json:"unresolved,omitempty"`
	Warnings           []string                       `json:"warnings,omitempty"`
}

// QueryAccessRelationReference represents a relation read by the query.
type QueryAccessRelationReference struct {
	Schema             string `json:"schema,omitempty"`
	Name               string `json:"name"`
	Alias              string `json:"alias,omitempty"`
	Kind               string `json:"kind"`
	PermissionRequired bool   `json:"permission_required"`
}

// QueryAccessColumnReference represents a source column reference.
type QueryAccessColumnReference struct {
	Schema string   `json:"schema,omitempty"`
	Table  string   `json:"table"`
	Column string   `json:"column"`
	Usages []string `json:"usages"`
}

// QueryAccessOutputColumn represents a final output column.
type QueryAccessOutputColumn struct {
	Name    string   `json:"name"`
	Sources []string `json:"sources"`
}

// QueryAccessRequirement represents a permission requirement.
type QueryAccessRequirement struct {
	Object    string `json:"object"`
	Privilege string `json:"privilege"`
}

// QueryAccessUnresolved represents an unresolved reference.
type QueryAccessUnresolved struct {
	Reference string `json:"reference"`
	Reason    string `json:"reason"`
}

// AnalyzeQueryAccess performs query access analysis.
func AnalyzeQueryAccess(ctx context.Context, req QueryAccessRequest) (*QueryAccessResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("analyze cancelled: %w", err)
	}

	dialect := toDomainQADialect(req.Dialect)
	mode, err := toDomainQAMode(req.Mode)
	if err != nil {
		return nil, err
	}

	var resolver appqa.SchemaResolver
	if req.SchemaResolver != nil {
		resolver = &publicSchemaResolver{resolver: req.SchemaResolver}
	}

	service := &appqa.Service{}
	appResult, err := service.Analyze(ctx, appqa.QueryAccessRequest{
		SQL:            req.SQL,
		Dialect:        dialect,
		Mode:           string(mode),
		DefaultSchema:  req.DefaultSchema,
		SchemaResolver: resolver,
	})
	if err != nil {
		if errors.Is(err, appqa.ErrExtractionFailed) {
			return nil, err
		}
		return nil, fmt.Errorf("query access analysis: %w", err)
	}

	result := fromDomainQAResult(appResult.DomainResult)
	return &result, nil
}

// ErrQueryAccessUnsupportedDialect is returned when the dialect is not supported for query access analysis.
var ErrQueryAccessUnsupportedDialect = errors.New("unsupported dialect for query access analysis")

// ErrInvalidQueryAccessMode is returned when the mode is not a recognized value.
var ErrInvalidQueryAccessMode = errors.New("invalid query access mode: must be strict or projection_only")

func toDomainQADialect(d Dialect) string {
	switch d {
	case DialectMySQL, DialectTiDB:
		return string(d)
	case DialectPostgreSQL:
		return "postgresql"
	default:
		return string(d)
	}
}

func toDomainQAMode(m QueryAccessMode) (domainqa.Mode, error) {
	switch m {
	case QueryAccessModeStrict, "":
		return domainqa.ModeStrict, nil
	case QueryAccessModeProjectionOnly:
		return domainqa.ModeProjectionOnly, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidQueryAccessMode, m)
	}
}

func fromDomainQAResult(result domainqa.Result) QueryAccessResult {
	public := QueryAccessResult{
		Dialect:            result.Dialect,
		Mode:               QueryAccessMode(result.Mode),
		ReadClassification: QueryAccessReadClassification(result.ReadClassification),
		Admission:          QueryAccessAdmission(result.Admission),
	}
	if len(result.ReasonCodes) > 0 {
		public.ReasonCodes = make([]string, len(result.ReasonCodes))
		for i, rc := range result.ReasonCodes {
			public.ReasonCodes[i] = string(rc)
		}
	}
	if len(result.Relations) > 0 {
		public.Relations = make([]QueryAccessRelationReference, len(result.Relations))
		for i, r := range result.Relations {
			public.Relations[i] = QueryAccessRelationReference{
				Schema:             r.Schema,
				Name:               r.Name,
				Alias:              r.Alias,
				Kind:               string(r.Kind),
				PermissionRequired: r.PermissionRequired,
			}
		}
	}
	if len(result.ReferencedColumns) > 0 {
		public.ReferencedColumns = make([]QueryAccessColumnReference, len(result.ReferencedColumns))
		for i, c := range result.ReferencedColumns {
			usages := make([]string, len(c.Usages))
			for j, u := range c.Usages {
				usages[j] = string(u)
			}
			public.ReferencedColumns[i] = QueryAccessColumnReference{
				Schema: c.Schema,
				Table:  c.Table,
				Column: c.Column,
				Usages: usages,
			}
		}
	}
	if len(result.Outputs) > 0 {
		public.Outputs = make([]QueryAccessOutputColumn, len(result.Outputs))
		for i, o := range result.Outputs {
			public.Outputs[i] = QueryAccessOutputColumn{
				Name:    o.Name,
				Sources: append([]string(nil), o.Sources...),
			}
		}
	}
	if len(result.Requirements) > 0 {
		public.Requirements = make([]QueryAccessRequirement, len(result.Requirements))
		for i, r := range result.Requirements {
			public.Requirements[i] = QueryAccessRequirement{
				Object:    r.Object,
				Privilege: r.Privilege,
			}
		}
	}
	if len(result.Unresolved) > 0 {
		public.Unresolved = make([]QueryAccessUnresolved, len(result.Unresolved))
		for i, u := range result.Unresolved {
			public.Unresolved[i] = QueryAccessUnresolved{
				Reference: u.Reference,
				Reason:    string(u.Reason),
			}
		}
	}
	if len(result.Warnings) > 0 {
		public.Warnings = make([]string, len(result.Warnings))
		for i, w := range result.Warnings {
			public.Warnings[i] = string(w)
		}
	}
	return public
}

type publicSchemaResolver struct {
	resolver QueryAccessSchemaResolver
}

func (p *publicSchemaResolver) ResolveRelation(ctx context.Context, dialect string, schema, name string) (appqa.RelationSchema, error) {
	publicSchema, err := p.resolver.ResolveRelation(ctx, dialect, schema, name)
	if err != nil {
		return appqa.RelationSchema{}, err
	}
	columns := make([]appqa.ColumnSchema, len(publicSchema.Columns))
	for i, c := range publicSchema.Columns {
		columns[i] = appqa.ColumnSchema{
			Name:    c.Name,
			Ordinal: c.Ordinal,
		}
	}
	return appqa.RelationSchema{
		Schema:  publicSchema.Schema,
		Name:    publicSchema.Name,
		Kind:    publicSchema.Kind,
		Columns: columns,
		IsView:  publicSchema.IsView,
	}, nil
}
