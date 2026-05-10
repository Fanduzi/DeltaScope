// Package httpapi exposes the HTTP adapter for DeltaScope.
// input: parsed HTTP audit requests, shared metadata-preparation helpers, and public audit execution functions
// output: additive HTTP audit context plus offline or metadata-aware audit execution results
// pos: HTTP adapter glue between request-scoped metadata inputs and the public DeltaScope audit API
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"context"
	"fmt"
	"os"
	"strings"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	apppolicy "github.com/Fanduzi/DeltaScope/internal/application/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	ifaceconn "github.com/Fanduzi/DeltaScope/internal/interfaces/metadata"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

var prepareHTTPMetadataAudit = auditmeta.Prepare
var loadHTTPPolicy = apppolicy.Load

type auditRunContext struct {
	Mode           string `json:"mode,omitempty"`
	Dialect        string `json:"dialect,omitempty"`
	DialectSource  string `json:"dialect_source,omitempty"`
	Schema         string `json:"schema,omitempty"`
	SchemaSource   string `json:"schema_source,omitempty"`
	MetadataSource string `json:"metadata_source,omitempty"`
}

type auditResponse struct {
	deltascope.Result
	Context *auditRunContext `json:"context,omitempty"`
}

func executeAuditRequest(
	ctx context.Context,
	request auditRequest,
	configPath string,
	auditFn func(context.Context, deltascope.Request) (deltascope.Result, error),
	metadataDefault MetadataConfig,
) (auditResponse, error) {
	if request.Connection == nil {
		return executeOfflineAudit(ctx, request, configPath, auditFn)
	}
	return executeMetadataAwareAudit(ctx, request, configPath, auditFn, metadataDefault)
}

func executeOfflineAudit(
	ctx context.Context,
	request auditRequest,
	configPath string,
	auditFn func(context.Context, deltascope.Request) (deltascope.Result, error),
) (auditResponse, error) {
	dialect, dialectSource := resolveHTTPAuditDialect(string(request.Dialect))
	schema, schemaSource := resolveRequestSchema(request)
	result, err := auditFn(ctx, deltascope.Request{
		SQL:        request.SQL,
		Dialect:    dialect,
		ConfigPath: configPath,
		Schema:     schema,
	})
	if err != nil {
		return auditResponse{}, err
	}
	return auditResponse{
		Result: result,
		Context: &auditRunContext{
			Mode:           "offline",
			Dialect:        string(dialect),
			DialectSource:  dialectSource,
			Schema:         schema,
			SchemaSource:   schemaSource,
			MetadataSource: "none",
		},
	}, nil
}

func executeMetadataAwareAudit(
	ctx context.Context,
	request auditRequest,
	configPath string,
	auditFn func(context.Context, deltascope.Request) (deltascope.Result, error),
	metadataDefault MetadataConfig,
) (auditResponse, error) {
	configSnapshotPath, cleanupConfigSnapshot, err := snapshotHTTPPolicy(configPath)
	if err != nil {
		return auditResponse{}, err
	}
	defer cleanupConfigSnapshot()

	if err := ifaceconn.ValidateConnectionInput(*request.Connection); err != nil {
		return auditResponse{}, err
	}

	password, err := ifaceconn.ResolvePassword(*request.Connection, ifaceconn.ResolveConnectionOptions{})
	if err != nil {
		return auditResponse{}, err
	}

	connectTimeout, set, err := ifaceconn.ParseConnectTimeout(*request.Connection)
	if err != nil {
		return auditResponse{}, err
	}
	if !set {
		connectTimeout = metadataDefault.ConnectTimeout
	}

	explicitDialectValue := strings.TrimSpace(string(request.Dialect))
	if explicitDialectValue == "" {
		explicitDialectValue = strings.TrimSpace(request.Connection.Dialect)
	}
	requestedPublicDialect, _ := resolveHTTPAuditDialect(explicitDialectValue)
	schema, schemaSource := resolveRequestSchema(request)

	prepared, err := prepareHTTPMetadataAudit(ctx, auditmeta.Request{
		SQL: request.SQL,
		Connection: auditmeta.ConnectionConfig{
			Host:           strings.TrimSpace(request.Connection.Host),
			Port:           request.Connection.Port,
			Socket:         strings.TrimSpace(request.Connection.Socket),
			User:           strings.TrimSpace(request.Connection.User),
			Password:       password,
			ConnectTimeout: connectTimeout,
			Dialect: func() spec.Dialect {
				if explicitDialectValue != "" {
					return toMetadataDialect(requestedPublicDialect)
				}
				return ""
			}(),
		},
		RequestedDialect:     toMetadataDialect(requestedPublicDialect),
		ExplicitDialect:      explicitDialectValue != "",
		ExplicitSchema:       schema,
		ExplicitSchemaSource: schemaSource,
		SchemaHint:           "schema",
	})
	if err != nil {
		return auditResponse{}, err
	}
	defer prepared.Client.Close()

	result, err := auditFn(ctx, deltascope.Request{
		SQL:              request.SQL,
		Dialect:          deltascope.Dialect(prepared.Dialect),
		ConfigPath:       configSnapshotPath,
		Schema:           prepared.Schema,
		MetadataProvider: publicMetadataProvider{client: prepared.Client},
	})
	if err != nil {
		return auditResponse{}, err
	}
	return auditResponse{
		Result: result,
		Context: &auditRunContext{
			Mode:           "metadata-aware",
			Dialect:        string(prepared.Dialect),
			DialectSource:  prepared.DialectSource,
			Schema:         prepared.Schema,
			SchemaSource:   prepared.SchemaSource,
			MetadataSource: "direct",
		},
	}, nil
}

func snapshotHTTPPolicy(configPath string) (string, func(), error) {
	if strings.TrimSpace(configPath) == "" {
		return "", func() {}, nil
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", func() {}, fmt.Errorf("load policy: %w", err)
	}

	tempFile, err := os.CreateTemp("", "deltascope-http-policy-*.yaml")
	if err != nil {
		return "", func() {}, fmt.Errorf("load policy: %w", err)
	}
	tempPath := tempFile.Name()
	cleanup := func() {
		_ = os.Remove(tempPath)
	}

	if _, err := tempFile.Write(content); err != nil {
		_ = tempFile.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("load policy: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("load policy: %w", err)
	}

	if _, err := loadHTTPPolicy(tempPath); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("load policy: %w", err)
	}

	return tempPath, cleanup, nil
}

func resolveHTTPAuditDialect(raw string) (deltascope.Dialect, string) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "mysql":
		if strings.TrimSpace(raw) == "" {
			return deltascope.DialectMySQL, "default"
		}
		return deltascope.DialectMySQL, "request"
	case "tidb":
		return deltascope.DialectTiDB, "request"
	case "postgresql":
		return deltascope.DialectPostgreSQL, "request"
	default:
		return deltascope.Dialect(strings.ToLower(strings.TrimSpace(raw))), "request"
	}
}

func resolveRequestSchema(request auditRequest) (string, string) {
	switch {
	case strings.TrimSpace(request.Schema) != "":
		return strings.TrimSpace(request.Schema), "request"
	case request.Connection != nil && strings.TrimSpace(request.Connection.Schema) != "":
		return strings.TrimSpace(request.Connection.Schema), "connection"
	default:
		return "", ""
	}
}

func toMetadataDialect(dialect deltascope.Dialect) spec.Dialect {
	switch dialect {
	case deltascope.DialectTiDB:
		return spec.DialectTiDB
	case deltascope.DialectMySQL, "":
		return spec.DialectMySQL
	default:
		return spec.Dialect(dialect)
	}
}

type publicMetadataProvider struct {
	client auditmeta.Client
}

type internalIndexOwnerResolver interface {
	ResolveTableForIndex(ctx context.Context, dialect spec.Dialect, schema string, index string) (string, error)
}

func (p publicMetadataProvider) LoadInstanceFacts(ctx context.Context, dialect deltascope.Dialect, schema string) (*deltascope.InstanceFacts, error) {
	return p.client.LoadInstanceFacts(ctx, toMetadataDialect(dialect), schema)
}

func (p publicMetadataProvider) LoadTableSnapshot(ctx context.Context, dialect deltascope.Dialect, schema string, table string) (*deltascope.TableSnapshot, error) {
	return p.client.LoadTableSnapshot(ctx, toMetadataDialect(dialect), schema, table)
}

func (p publicMetadataProvider) ResolveTableForIndex(ctx context.Context, dialect deltascope.Dialect, schema string, index string) (string, error) {
	resolver, ok := p.client.(internalIndexOwnerResolver)
	if !ok {
		return "", nil
	}
	return resolver.ResolveTableForIndex(ctx, toMetadataDialect(dialect), schema, index)
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
