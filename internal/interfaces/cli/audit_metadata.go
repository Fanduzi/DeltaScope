// Package cli exposes the command-line adapter for DeltaScope.
// input: metadata-aware audit connection options, SQL text, and the shared auditmeta opener
// output: resolved dialect/schema context including MySQL/TiDB catalog aliases, offline existence caveat fields, and provider wiring for metadata-aware CLI audits
// pos: CLI metadata-aware audit preparation between command flags and application requests
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"strings"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	ifaceconn "github.com/Fanduzi/DeltaScope/internal/interfaces/metadata"
)

type metadataClient = auditmeta.Client

var newMetadataClient = openMetadataClient

type auditRunContext struct {
	Mode          string   `json:"mode,omitempty"`
	Dialect       string   `json:"dialect,omitempty"`
	DialectSource string   `json:"dialect_source,omitempty"`
	Schema        string   `json:"schema,omitempty"`
	SchemaSource  string   `json:"schema_source,omitempty"`
	Note          string   `json:"note,omitempty"`
	Unproven      []string `json:"unproven,omitempty"`
}

const existenceNotCheckedNote = ifaceconn.ExistenceNotCheckedNote

func offlineExistenceUnproven() []string {
	return ifaceconn.OfflineExistenceUnproven()
}

func openMetadataClient(options auditConnectionOptions) (metadataClient, error) {
	return auditmeta.OpenClient(auditmeta.ConnectionConfig{
		Host:           options.Host,
		Port:           options.Port,
		Socket:         options.Socket,
		User:           options.User,
		Password:       options.Password,
		Database:       options.Database,
		Dialect:        spec.Dialect(strings.ToLower(strings.TrimSpace(options.Dialect))),
		ConnectTimeout: options.ConnectTimeout,
		TLSMode:        options.TLSMode,
		CACert:         options.CACert,
	})
}

func prepareMetadataAudit(ctx context.Context, sqlText string, options auditConnectionOptions, requestedDialect spec.Dialect, explicitDialect bool) (metadataClient, spec.Dialect, string, *auditRunContext, error) {
	prepared, err := auditmeta.Prepare(ctx, auditmeta.Request{
		SQL:                  sqlText,
		Connection:           toAuditMetaConnection(options, requestedDialect, explicitDialect),
		RequestedDialect:     requestedDialect,
		ExplicitDialect:      explicitDialect,
		ExplicitSchema:       options.Schema,
		ExplicitSchemaSource: "flag",
		OpenClient: func(config auditmeta.ConnectionConfig) (auditmeta.Client, error) {
			return newMetadataClient(auditConnectionOptions{
				Host:           config.Host,
				Port:           config.Port,
				Socket:         config.Socket,
				User:           config.User,
				Password:       config.Password,
				Database:       config.Database,
				Dialect:        string(config.Dialect),
				ConnectTimeout: config.ConnectTimeout,
				TLSMode:        config.TLSMode,
				CACert:         config.CACert,
			})
		},
	})
	if err != nil {
		return nil, "", "", nil, err
	}

	return prepared.Client, prepared.Dialect, prepared.Schema, &auditRunContext{
		Mode:          "metadata-aware",
		Dialect:       string(prepared.Dialect),
		DialectSource: prepared.DialectSource,
		Schema:        prepared.Schema,
		SchemaSource:  prepared.SchemaSource,
	}, nil
}

func toAuditMetaConnection(options auditConnectionOptions, requestedDialect spec.Dialect, explicitDialect bool) auditmeta.ConnectionConfig {
	connection := auditmeta.ConnectionConfig{
		Host:           options.Host,
		Port:           options.Port,
		Socket:         options.Socket,
		User:           options.User,
		Password:       options.Password,
		Database:       options.Database,
		ConnectTimeout: options.ConnectTimeout,
		TLSMode:        options.TLSMode,
		CACert:         options.CACert,
	}
	if explicitDialect {
		connection.Dialect = requestedDialect
	}
	return connection
}

// cliMetadataProvider wraps an auditmeta.Client to forward all metadata
// capabilities including optional interface-asserted methods like ResolveObject.
type cliMetadataProvider struct {
	client auditmeta.Client
}

func (p cliMetadataProvider) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	return p.client.LoadInstanceFacts(ctx, dialect, schema)
}

func (p cliMetadataProvider) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	return p.client.LoadTableSnapshot(ctx, dialect, schema, table)
}

type cliIndexOwnerResolver interface {
	ResolveTableForIndex(ctx context.Context, dialect spec.Dialect, schema string, index string) (string, error)
}

func (p cliMetadataProvider) ResolveTableForIndex(ctx context.Context, dialect spec.Dialect, schema string, index string) (string, error) {
	resolver, ok := p.client.(cliIndexOwnerResolver)
	if !ok {
		return "", nil
	}
	return resolver.ResolveTableForIndex(ctx, dialect, schema, index)
}

func (p cliMetadataProvider) LoadPlanEstimate(ctx context.Context, statement spec.Statement) (*spec.ImpactEstimate, error) {
	provider, ok := p.client.(interface {
		LoadPlanEstimate(context.Context, spec.Statement) (*spec.ImpactEstimate, error)
	})
	if !ok {
		return nil, nil
	}
	return provider.LoadPlanEstimate(ctx, statement)
}

func (p cliMetadataProvider) ResolveObject(ctx context.Context, dialect spec.Dialect, request spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
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
