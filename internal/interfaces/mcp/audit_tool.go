// Package mcpapi exposes the MCP adapter for DeltaScope.
// input: audit_sql MCP requests, shared DeltaScope public audit API, and resolved run-context metadata
// output: structured MCP audit_sql responses that preserve DeltaScope's public audit result body
// pos: MCP audit tool adapter between tool invocations and the shared audit engine
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"fmt"
	"strings"

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
			return auditSQLWithMetadata(ctx, input, connection)
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
		toolResult, toolErr := toolError(mapAuditToolError(err), err.Error())
		return toolResult, nil, toolErr
	}
	toolResult, payload := successAuditResult(result, AuditContext{
		Mode:           "offline",
		Dialect:        string(dialect),
		DialectSource:  dialectSource,
		MetadataSource: MetadataSourceNone,
	})
	return toolResult, payload, nil
}

func auditSQLWithMetadata(ctx context.Context, input AuditSQLParams, connection ResolvedConnection) (*sdkmcp.CallToolResult, any, error) {
	explicitDialectValue := strings.TrimSpace(input.Dialect)
	if explicitDialectValue == "" {
		explicitDialectValue = strings.TrimSpace(connection.Dialect)
	}
	if err := validateSupportedAuditDialect(explicitDialectValue); err != nil {
		toolResult, toolErr := toolError(mapAuditToolError(err), err.Error())
		return toolResult, nil, toolErr
	}
	dialect, _ := resolvePublicDialect(explicitDialectValue)

	connectTimeout, _, err := ifaceconn.ParseConnectTimeout(ifaceconn.ConnectionInput{ConnectTimeout: connection.ConnectTimeout})
	if err != nil {
		toolResult, toolErr := toolError("connection_invalid", err.Error())
		return toolResult, nil, toolErr
	}

	prepared, err := prepareMetadataAudit(ctx, auditmeta.Request{
		SQL: input.SQL,
		Connection: auditmeta.ConnectionConfig{
			Host:           connection.Host,
			Port:           connection.Port,
			Socket:         connection.Socket,
			User:           connection.User,
			Password:       connection.Password,
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
				&sdkmcp.TextContent{Text: fmt.Sprintf("Audit verdict: %s", result.Verdict)},
			},
		}, AuditSQLResult{
			Result:  result,
			Context: context,
		}
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
