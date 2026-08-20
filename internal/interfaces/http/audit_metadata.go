// Package httpapi exposes the HTTP adapter for DeltaScope.
// input: parsed HTTP audit requests, shared metadata-preparation helpers, and public audit execution functions
// output: additive HTTP audit context including offline existence caveats plus offline or registry-based metadata-aware audit execution results
// pos: HTTP adapter glue between request-scoped metadata inputs and the public DeltaScope audit API
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/application/online"
	apppolicy "github.com/Fanduzi/DeltaScope/internal/application/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
	ifaceconn "github.com/Fanduzi/DeltaScope/internal/interfaces/metadata"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

var prepareHTTPMetadataAudit = auditmeta.Prepare
var loadHTTPPolicy = apppolicy.Load

type auditRunContext struct {
	Mode           string   `json:"mode,omitempty"`
	Dialect        string   `json:"dialect,omitempty"`
	DialectSource  string   `json:"dialect_source,omitempty"`
	Schema         string   `json:"schema,omitempty"`
	SchemaSource   string   `json:"schema_source,omitempty"`
	MetadataSource string   `json:"metadata_source,omitempty"`
	Note           string   `json:"note,omitempty"`
	Unproven       []string `json:"unproven,omitempty"`
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
	registry *runtimeconfig.Registry,
	principalID string,
) (auditResponse, error) {
	if request.ConnectionID == "" {
		return executeOfflineAudit(ctx, request, configPath, auditFn)
	}
	return executeRegistryAwareAudit(ctx, request, configPath, auditFn, registry, principalID, metadataDefault)
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
		if len(result.Diagnostics) > 0 {
			return auditResponse{Result: result}, err
		}
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
			Note:           ifaceconn.ExistenceNotCheckedNote,
			Unproven:       ifaceconn.OfflineExistenceUnproven(),
		},
	}, nil
}

func executeRegistryAwareAudit(
	ctx context.Context,
	request auditRequest,
	configPath string,
	auditFn func(context.Context, deltascope.Request) (deltascope.Result, error),
	registry *runtimeconfig.Registry,
	principalID string,
	metadataDefault MetadataConfig,
) (auditResponse, error) {
	if registry == nil {
		return auditResponse{}, online.ErrConnectionNotFound
	}

	conn, ok := registry.LookupConnection(request.ConnectionID)
	if !ok {
		return auditResponse{}, online.ErrConnectionNotFound
	}

	if err := registry.Authorize(principalID, request.ConnectionID, "audit"); err != nil {
		return auditResponse{}, mapRegistryAuthorizeError(err)
	}

	configSnapshotPath, cleanupConfigSnapshot, err := snapshotHTTPPolicy(configPath)
	if err != nil {
		return auditResponse{}, err
	}
	defer cleanupConfigSnapshot()

	connectTimeout, _, _ := runtimeconfig.ParseConnectTimeout(conn.ConnectTimeout)
	if connectTimeout == 0 {
		connectTimeout = metadataDefault.ConnectTimeout
	}

	connDialect := deltascope.Dialect(strings.ToLower(strings.TrimSpace(conn.Dialect)))
	schema := strings.TrimSpace(conn.Schema)
	if strings.TrimSpace(request.Schema) != "" {
		schema = strings.TrimSpace(request.Schema)
	}

	prepared, err := prepareHTTPMetadataAudit(ctx, auditmeta.Request{
		SQL: request.SQL,
		Connection: auditmeta.ConnectionConfig{
			Host:           strings.TrimSpace(conn.Host),
			Port:           conn.Port,
			Socket:         strings.TrimSpace(conn.Socket),
			User:           strings.TrimSpace(conn.User),
			Password:       conn.ResolvedPassword(),
			Database:       strings.TrimSpace(conn.Database),
			ConnectTimeout: connectTimeout,
			Dialect:        toMetadataDialect(connDialect),
			TLSMode:        strings.ToLower(strings.TrimSpace(conn.TLSMode)),
			CACert:         conn.ResolvedCACert(),
		},
		RequestedDialect:     toMetadataDialect(connDialect),
		ExplicitDialect:      true,
		ExplicitSchema:       schema,
		ExplicitSchemaSource: "registry",
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
		if len(result.Diagnostics) > 0 {
			return auditResponse{Result: result}, err
		}
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
			MetadataSource: "registry",
		},
	}, nil
}

func mapRegistryAuthorizeError(err error) error {
	if errors.Is(err, runtimeconfig.ErrConnectionNotFound) {
		return online.ErrConnectionNotFound
	}
	if errors.Is(err, runtimeconfig.ErrPurposeNotAllowed) {
		return online.ErrPurposeNotAllowed
	}
	if errors.Is(err, runtimeconfig.ErrPrincipalNotAllowed) {
		return online.ErrPrincipalNotAllowed
	}
	return err
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
	if strings.TrimSpace(request.Schema) != "" {
		return strings.TrimSpace(request.Schema), "request"
	}
	return "", ""
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
