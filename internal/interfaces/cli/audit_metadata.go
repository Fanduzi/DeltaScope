// Package cli exposes the command-line adapter for DeltaScope.
// input: metadata-aware audit connection options, SQL text, and metadata provider clients
// output: resolved dialect/schema context and provider wiring for metadata-aware CLI audits
// pos: CLI metadata-aware audit preparation between command flags and application requests
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

type metadataClient interface {
	appaudit.MetadataProvider
	DetectDialect(ctx context.Context) (spec.Dialect, error)
	FindSchemasForTable(ctx context.Context, table string) ([]string, error)
	Close() error
}

var newMetadataClient = openMetadataClient

type auditRunContext struct {
	Mode          string `json:"mode,omitempty"`
	Dialect       string `json:"dialect,omitempty"`
	DialectSource string `json:"dialect_source,omitempty"`
	Schema        string `json:"schema,omitempty"`
	SchemaSource  string `json:"schema_source,omitempty"`
}

func openMetadataClient(options auditConnectionOptions) (metadataClient, error) {
	db, err := mysqlmeta.OpenDB(mysqlmeta.ConnectionConfig{
		Host:     options.Host,
		Port:     options.Port,
		Socket:   options.Socket,
		User:     options.User,
		Password: options.Password,
	})
	if err != nil {
		return nil, err
	}
	return mysqlMetadataClient{
		db:       db,
		provider: mysqlmeta.NewProvider(db),
	}, nil
}

type mysqlMetadataClient struct {
	db       *sql.DB
	provider *mysqlmeta.Provider
}

func (c mysqlMetadataClient) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	return c.provider.LoadInstanceFacts(ctx, dialect, schema)
}

func (c mysqlMetadataClient) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	return c.provider.LoadTableSnapshot(ctx, dialect, schema, table)
}

func (c mysqlMetadataClient) DetectDialect(ctx context.Context) (spec.Dialect, error) {
	return c.provider.DetectDialect(ctx)
}

func (c mysqlMetadataClient) FindSchemasForTable(ctx context.Context, table string) ([]string, error) {
	return c.provider.FindSchemasForTable(ctx, table)
}

func (c mysqlMetadataClient) Close() error {
	return c.db.Close()
}

func prepareMetadataAudit(ctx context.Context, sqlText string, options auditConnectionOptions, requestedDialect spec.Dialect, explicitDialect bool) (metadataClient, spec.Dialect, string, *auditRunContext, error) {
	client, err := newMetadataClient(options)
	if err != nil {
		return nil, "", "", nil, newUserError(fmt.Sprintf("open metadata connection: %v", err))
	}

	closeOnError := true
	defer func() {
		if closeOnError {
			_ = client.Close()
		}
	}()

	detectedDialect, err := client.DetectDialect(ctx)
	if err != nil {
		return nil, "", "", nil, newUserError(fmt.Sprintf("detect dialect: %v", err))
	}
	if explicitDialect && requestedDialect != detectedDialect {
		return nil, "", "", nil, newUserError(fmt.Sprintf("detected dialect %q does not match --dialect %q", detectedDialect, requestedDialect))
	}

	schema, schemaSource, err := resolveAuditSchema(ctx, client, sqlText, detectedDialect, options.Schema)
	if err != nil {
		return nil, "", "", nil, err
	}

	closeOnError = false
	return client, detectedDialect, schema, &auditRunContext{
		Mode:          "metadata-aware",
		Dialect:       string(detectedDialect),
		DialectSource: "detected",
		Schema:        schema,
		SchemaSource:  schemaSource,
	}, nil
}

func resolveAuditSchema(ctx context.Context, client metadataClient, sqlText string, dialect spec.Dialect, explicitSchema string) (string, string, error) {
	if strings.TrimSpace(explicitSchema) != "" {
		return strings.TrimSpace(explicitSchema), "flag", nil
	}

	targets, err := collectTargetTables(sqlText, dialect)
	if err != nil {
		return "", "", newUserError(fmt.Sprintf("resolve schema targets: %v", err))
	}
	if len(targets) == 0 {
		return "", "", nil
	}

	resolvedSchemas := make(map[string]struct{})
	for _, target := range targets {
		schemas, err := client.FindSchemasForTable(ctx, target.Name)
		if err != nil {
			return "", "", newUserError(fmt.Sprintf("resolve schema for table %q: %v", target.Name, err))
		}
		switch len(schemas) {
		case 0:
			if target.RequiresExisting {
				return "", "", newUserError(fmt.Sprintf("could not infer schema for table %q; pass --schema", target.Name))
			}
		case 1:
			resolvedSchemas[schemas[0]] = struct{}{}
		default:
			return "", "", newUserError(fmt.Sprintf("schema inference for table %q is ambiguous; pass --schema", target.Name))
		}
	}

	if len(resolvedSchemas) == 0 {
		return "", "", nil
	}
	if len(resolvedSchemas) > 1 {
		schemas := make([]string, 0, len(resolvedSchemas))
		for schema := range resolvedSchemas {
			schemas = append(schemas, schema)
		}
		sort.Strings(schemas)
		return "", "", newUserError(fmt.Sprintf("resolved multiple schemas (%s); pass --schema", strings.Join(schemas, ", ")))
	}
	for schema := range resolvedSchemas {
		return schema, "inferred", nil
	}
	return "", "", nil
}

type schemaTarget struct {
	Name             string
	RequiresExisting bool
}

func collectTargetTables(sqlText string, dialect spec.Dialect) ([]schemaTarget, error) {
	parsed, err := appaudit.Parse(sqlText, dialect)
	if err != nil {
		return nil, err
	}
	statements, err := appaudit.Extract(parsed)
	if err != nil {
		return nil, err
	}

	targetsByName := make(map[string]schemaTarget)
	order := make([]string, 0)
	for _, statement := range statements {
		name, requiresExisting := statementTarget(statement)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		existing, ok := targetsByName[key]
		if ok {
			existing.RequiresExisting = existing.RequiresExisting || requiresExisting
			targetsByName[key] = existing
			continue
		}
		targetsByName[key] = schemaTarget{Name: name, RequiresExisting: requiresExisting}
		order = append(order, key)
	}

	targets := make([]schemaTarget, 0, len(order))
	for _, key := range order {
		targets = append(targets, targetsByName[key])
	}
	return targets, nil
}

func statementTarget(statement spec.Statement) (string, bool) {
	if statement.DDL != nil && statement.DDL.Table != nil {
		name := strings.TrimSpace(statement.DDL.Table.Name)
		if name == "" {
			return "", false
		}
		return name, statement.DDL.Operation != spec.DDLOperationCreateTable
	}
	if statement.DML != nil && len(statement.DML.Tables) > 0 {
		name := strings.TrimSpace(statement.DML.Tables[0].Name)
		if name == "" {
			return "", false
		}
		return name, true
	}
	return "", false
}
