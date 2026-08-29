// Package mcpapi exposes the MCP adapter for DeltaScope.
// input: audit_sql MCP requests, shared DeltaScope public audit API, and resolved run-context metadata
// output: structured MCP audit_sql responses, partial parser-error results, and compact finding summaries with offline existence caveats and resolved catalog-aware metadata audits
// pos: MCP audit tool adapter between tool invocations and the shared audit engine
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	ifaceconn "github.com/Fanduzi/DeltaScope/internal/interfaces/metadata"
	publicapi "github.com/Fanduzi/DeltaScope/pkg/deltascope"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// AuditContext describes how one MCP audit_sql result was produced.
type AuditContext struct {
	Mode           string         `json:"mode,omitempty"`
	Dialect        string         `json:"dialect,omitempty"`
	DialectSource  string         `json:"dialect_source,omitempty"`
	Schema         string         `json:"schema,omitempty"`
	SchemaSource   string         `json:"schema_source,omitempty"`
	MetadataSource MetadataSource `json:"metadata_source,omitempty"`
	Note           string         `json:"note,omitempty"`
	Unproven       []string       `json:"unproven,omitempty"`
}

// AuditSQLResult preserves the public DeltaScope result body and adds MCP context.
type AuditSQLResult struct {
	publicapi.Result
	Context AuditContext `json:"context"`
}

var prepareMetadataAudit = auditmeta.Prepare

func newAuditSQLTool(config Config) func(context.Context, *sdkmcp.CallToolRequest, AuditSQLParams) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, input AuditSQLParams) (*sdkmcp.CallToolResult, any, error) {
		connection, err := ResolveAuditConnection(input, ResolveConnectionOptions{
			ConnectionsPath: strings.TrimSpace(config.ConnectionsPath),
		})
		if err != nil {
			toolResult, toolErr := toolError(mapAuditToolError(err), err.Error())
			return toolResult, nil, toolErr
		}
		if connection.Enabled {
			return auditSQLWithMetadata(ctx, input, connection, config.MetadataConnectTimeout)
		}
		return auditSQLOffline(ctx, input)
	}
}

func resolvePublicDialect(raw string) (publicapi.Dialect, string) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "mysql":
		if strings.TrimSpace(raw) == "" {
			return publicapi.DialectMySQL, "default"
		}
		return publicapi.DialectMySQL, "request"
	case "tidb":
		return publicapi.DialectTiDB, "request"
	case "postgresql":
		return publicapi.DialectPostgreSQL, "request"
	default:
		return publicapi.Dialect(strings.ToLower(strings.TrimSpace(raw))), "request"
	}
}

func auditSQLOffline(ctx context.Context, input AuditSQLParams) (*sdkmcp.CallToolResult, any, error) {
	dialect, dialectSource := resolvePublicDialect(input.Dialect)
	result, err := publicapi.Audit(ctx, publicapi.Request{
		SQL:        input.SQL,
		Dialect:    dialect,
		ConfigPath: strings.TrimSpace(input.ConfigPath),
	})
	if err != nil {
		if len(result.Diagnostics) > 0 {
			return toolDiagnosticError(mapAuditToolError(err), err.Error(), AuditSQLResult{
				Result: result,
				Context: AuditContext{
					Mode:           "offline",
					Dialect:        string(dialect),
					DialectSource:  dialectSource,
					MetadataSource: MetadataSourceNone,
					Note:           ifaceconn.ExistenceNotCheckedNote,
					Unproven:       ifaceconn.OfflineExistenceUnproven(),
				},
			}), nil, nil
		}
		toolResult, toolErr := toolError(mapAuditToolError(err), err.Error())
		return toolResult, nil, toolErr
	}
	toolResult, payload := successAuditResult(result, AuditContext{
		Mode:           "offline",
		Dialect:        string(dialect),
		DialectSource:  dialectSource,
		MetadataSource: MetadataSourceNone,
		Note:           ifaceconn.ExistenceNotCheckedNote,
		Unproven:       ifaceconn.OfflineExistenceUnproven(),
	})
	return toolResult, payload, nil
}

func auditSQLWithMetadata(ctx context.Context, input AuditSQLParams, connection ResolvedConnection, defaultConnectTimeout time.Duration) (*sdkmcp.CallToolResult, any, error) {
	explicitDialectValue := strings.TrimSpace(input.Dialect)
	if explicitDialectValue == "" {
		explicitDialectValue = strings.TrimSpace(connection.Dialect)
	}
	if err := validateSupportedAuditDialect(explicitDialectValue); err != nil {
		toolResult, toolErr := toolError(mapAuditToolError(err), err.Error())
		return toolResult, nil, toolErr
	}
	dialect, _ := resolvePublicDialect(explicitDialectValue)

	connectTimeout, set, err := ifaceconn.ParseConnectTimeout(ifaceconn.ConnectionInput{ConnectTimeout: connection.ConnectTimeout})
	if err != nil {
		toolResult, toolErr := toolError("connection_invalid", err.Error())
		return toolResult, nil, toolErr
	}
	if !set {
		connectTimeout = defaultConnectTimeout
	}

	prepared, err := prepareMetadataAudit(ctx, auditmeta.Request{
		SQL: input.SQL,
		Connection: auditmeta.ConnectionConfig{
			Host:           connection.Host,
			Port:           connection.Port,
			Socket:         connection.Socket,
			User:           connection.User,
			Password:       connection.Password,
			Database:       connection.Database,
			ConnectTimeout: connectTimeout,
			Dialect: func() spec.Dialect {
				if explicitDialectValue != "" {
					return toDomainDialect(dialect)
				}
				return ""
			}(),
		},
		RequestedDialect:     toDomainDialect(dialect),
		ExplicitDialect:      explicitDialectValue != "",
		ExplicitSchema:       connection.Schema,
		ExplicitSchemaSource: metadataSchemaSource(connection),
		SchemaHint:           metadataSchemaHint(connection),
	})
	if err != nil {
		toolResult, toolErr := toolError(mapAuditToolError(err), err.Error())
		return toolResult, nil, toolErr
	}
	defer prepared.Client.Close()

	result, err := publicapi.Audit(ctx, publicapi.Request{
		SQL:              input.SQL,
		Dialect:          publicapi.Dialect(prepared.Dialect),
		ConfigPath:       strings.TrimSpace(input.ConfigPath),
		Schema:           prepared.Schema,
		MetadataProvider: publicMetadataProvider{client: prepared.Client},
	})
	if err != nil {
		if len(result.Diagnostics) > 0 {
			return toolDiagnosticError(mapAuditToolError(err), err.Error(), AuditSQLResult{
				Result: result,
				Context: AuditContext{
					Mode:           "metadata-aware",
					Dialect:        string(prepared.Dialect),
					DialectSource:  prepared.DialectSource,
					Schema:         prepared.Schema,
					SchemaSource:   prepared.SchemaSource,
					MetadataSource: connection.Source,
				},
			}), nil, nil
		}
		toolResult, toolErr := toolError(mapAuditToolError(err), err.Error())
		return toolResult, nil, toolErr
	}
	toolResult, payload := successAuditResult(result, AuditContext{
		Mode:           "metadata-aware",
		Dialect:        string(prepared.Dialect),
		DialectSource:  prepared.DialectSource,
		Schema:         prepared.Schema,
		SchemaSource:   prepared.SchemaSource,
		MetadataSource: connection.Source,
	})
	return toolResult, payload, nil
}

func successAuditResult(result publicapi.Result, context AuditContext) (*sdkmcp.CallToolResult, AuditSQLResult) {
	return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{
				&sdkmcp.TextContent{Text: renderAuditSQLText(result, context)},
			},
		}, AuditSQLResult{
			Result:  result,
			Context: context,
		}
}

func renderAuditSQLText(result publicapi.Result, runContext AuditContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Audit verdict: %s\n", result.Verdict)
	fmt.Fprintf(&b, "Statements: %d\n", result.Summary.Statements)
	fmt.Fprintf(&b, "Blockers: %d\n", result.Summary.Blockers)
	fmt.Fprintf(&b, "Warnings: %d\n", result.Summary.Warnings)
	fmt.Fprintf(&b, "Notices: %d\n", result.Summary.Notices)
	if runContext.Note != "" {
		fmt.Fprintf(&b, "%s\n", runContext.Note)
	}

	findings := collectAuditSQLFindings(result)
	if len(findings) == 0 {
		return b.String()
	}

	b.WriteByte('\n')
	for _, finding := range findings {
		fmt.Fprintf(&b, "[%s] %s: %s\n", finding.Level, finding.RuleID, finding.Message)
		if suggestion := findingSuggestionText(finding); suggestion != "" {
			fmt.Fprintf(&b, "  Suggestion: %s\n", suggestion)
		}
	}
	return b.String()
}

func collectAuditSQLFindings(result publicapi.Result) []publicapi.Finding {
	findings := make([]publicapi.Finding, 0)
	for _, statement := range result.Statements {
		findings = append(findings, statement.Findings...)
	}
	findings = append(findings, result.GlobalFindings...)
	return findings
}

func findingSuggestionText(finding publicapi.Finding) string {
	if suggestion := strings.TrimSpace(finding.Suggestion); suggestion != "" {
		return suggestion
	}
	if finding.Explanation != nil {
		return strings.TrimSpace(finding.Explanation.Suggestion)
	}
	return ""
}

func toDomainDialect(dialect publicapi.Dialect) spec.Dialect {
	switch dialect {
	case publicapi.DialectTiDB:
		return spec.DialectTiDB
	case publicapi.DialectMySQL, "":
		return spec.DialectMySQL
	default:
		return spec.Dialect(dialect)
	}
}

func metadataSchemaHint(connection ResolvedConnection) string {
	if connection.Source == MetadataSourceConnectionRef && strings.TrimSpace(connection.RefName) != "" {
		return fmt.Sprintf(`connections.%s.schema in %s`, connection.RefName, connection.RefPath)
	}
	return "connection.schema"
}

func metadataSchemaSource(connection ResolvedConnection) string {
	if connection.Source == MetadataSourceConnectionRef {
		return "config"
	}
	return "request"
}

func validateSupportedAuditDialect(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "mysql", "tidb", "postgresql":
		return nil
	default:
		return appaudit.ErrUnknownDialect
	}
}

type publicMetadataProvider struct {
	client auditmeta.Client
}

type internalIndexOwnerResolver interface {
	ResolveTableForIndex(ctx context.Context, dialect spec.Dialect, schema string, index string) (string, error)
}

func (p publicMetadataProvider) LoadInstanceFacts(ctx context.Context, dialect publicapi.Dialect, schema string) (*publicapi.InstanceFacts, error) {
	return p.client.LoadInstanceFacts(ctx, toDomainDialect(dialect), schema)
}

func (p publicMetadataProvider) LoadTableSnapshot(ctx context.Context, dialect publicapi.Dialect, schema string, table string) (*publicapi.TableSnapshot, error) {
	return p.client.LoadTableSnapshot(ctx, toDomainDialect(dialect), schema, table)
}

func (p publicMetadataProvider) ResolveTableForIndex(ctx context.Context, dialect publicapi.Dialect, schema string, index string) (string, error) {
	resolver, ok := p.client.(internalIndexOwnerResolver)
	if !ok {
		return "", nil
	}
	return resolver.ResolveTableForIndex(ctx, toDomainDialect(dialect), schema, index)
}

func (p publicMetadataProvider) LoadPlanEstimate(ctx context.Context, statement spec.Statement) (*spec.ImpactEstimate, error) {
	provider, ok := p.client.(interface {
		LoadPlanEstimate(context.Context, spec.Statement) (*spec.ImpactEstimate, error)
	})
	if !ok {
		return nil, nil
	}
	return provider.LoadPlanEstimate(ctx, statement)
}

func (p publicMetadataProvider) ResolveObject(ctx context.Context, dialect spec.Dialect, request spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	type objectResolver interface {
		ResolveObject(context.Context, spec.Dialect, spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error)
	}
	resolver, ok := p.client.(objectResolver)
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
