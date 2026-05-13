package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

var _ appaudit.ObjectResolver = (*Provider)(nil)

// ResolveObject looks up non-table database object metadata from PostgreSQL catalogs.
func (p *Provider) ResolveObject(ctx context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || strings.TrimSpace(req.Type) == "" {
		return &spec.ObjectSnapshot{
			Schema: req.Schema,
			Type:   req.Type,
			Name:   name,
			Status: spec.MetadataStatusUnavailable,
		}, nil
	}

	switch req.Type {
	case "type":
		return p.resolveType(ctx, req)
	case "domain":
		return p.resolveDomain(ctx, req)
	case "extension":
		return p.resolveExtension(ctx, req)
	case "publication":
		return p.resolvePublication(ctx, req)
	case "subscription":
		return p.resolveSubscription(ctx, req)
	case "foreign_table":
		return p.resolveForeignTable(ctx, req)
	case "foreign_server":
		return p.resolveForeignServer(ctx, req)
	case "user_mapping":
		return p.resolveUserMapping(ctx, req)
	case "foreign_data_wrapper":
		return p.resolveFDW(ctx, req)
	case "event_trigger":
		return p.resolveEventTrigger(ctx, req)
	case "rule":
		return p.resolveRule(ctx, req)
	case "schema":
		return p.resolveSchema(ctx, req)
	case "sequence":
		return p.resolveSequence(ctx, req)
	case "materialized_view":
		return p.resolveMaterializedView(ctx, req)
	case "comment", "security_label":
		return p.resolveAnnotationTarget(ctx, req)
	default:
		return &spec.ObjectSnapshot{
			Schema: req.Schema,
			Type:   req.Type,
			Name:   name,
			Status: spec.MetadataStatusUnavailable,
		}, nil
	}
}

// --- schema-scoped helpers ---

// resolveSchemaScoped handles the common pattern for schema-qualified objects.
// When schema is specified: exact match → confirmed/not_found.
// When schema is empty: enumerate schemas → confirmed/ambiguous/not_found.
func (p *Provider) resolveSchemaScoped(
	ctx context.Context,
	req spec.ObjectLookupRequest,
	findSchemas func(ctx context.Context, name string) ([]string, error),
	confirmInSchema func(ctx context.Context, schema, name string) (map[string]string, error),
) (*spec.ObjectSnapshot, error) {
	if req.Schema != "" {
		attrs, err := confirmInSchema(ctx, req.Schema, req.Name)
		if err != nil {
			return nil, err
		}
		if attrs == nil {
			return &spec.ObjectSnapshot{
				Schema: req.Schema,
				Type:   req.Type,
				Name:   req.Name,
				Status: spec.MetadataStatusNotFound,
			}, nil
		}
		return &spec.ObjectSnapshot{
			Schema:     req.Schema,
			Type:       req.Type,
			Name:       req.Name,
			Status:     spec.MetadataStatusConfirmed,
			Exists:     true,
			Attributes: attrs,
		}, nil
	}

	schemas, err := findSchemas(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	switch len(schemas) {
	case 0:
		return &spec.ObjectSnapshot{
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusNotFound,
		}, nil
	case 1:
		attrs, err := confirmInSchema(ctx, schemas[0], req.Name)
		if err != nil {
			return nil, err
		}
		return &spec.ObjectSnapshot{
			Schema:     schemas[0],
			Type:       req.Type,
			Name:       req.Name,
			Status:     spec.MetadataStatusConfirmed,
			Exists:     true,
			Attributes: attrs,
		}, nil
	default:
		return &spec.ObjectSnapshot{
			Type:                req.Type,
			Name:                req.Name,
			Status:              spec.MetadataStatusAmbiguous,
			AmbiguousCandidates: schemas,
		}, nil
	}
}

func (p *Provider) querySchemas(ctx context.Context, query string, args ...interface{}) ([]string, error) {
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var schemas []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		schemas = append(schemas, s)
	}
	return schemas, rows.Err()
}

// --- type ---

func (p *Provider) resolveType(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return p.resolveSchemaScoped(ctx, req,
		func(ctx context.Context, name string) ([]string, error) {
			return p.querySchemas(ctx, `
				select n.nspname from pg_catalog.pg_type t
				join pg_catalog.pg_namespace n on n.oid = t.typnamespace
				where t.typname = $1 and t.typtype <> 'd'
				  and n.nspname not in ('pg_catalog', 'information_schema')
				order by n.nspname`, name)
		},
		p.confirmTypeInSchema,
	)
}

func (p *Provider) confirmTypeInSchema(ctx context.Context, schema, name string) (map[string]string, error) {
	var typtype string
	err := p.db.QueryRowContext(ctx, `
		select t.typtype from pg_catalog.pg_type t
		join pg_catalog.pg_namespace n on n.oid = t.typnamespace
		where t.typname = $1 and n.nspname = $2 and t.typtype <> 'd'`,
		name, schema).Scan(&typtype)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm type %s.%s: %w", schema, name, err)
	}
	return map[string]string{"type_kind": typeKindName(typtype)}, nil
}

func typeKindName(typtype string) string {
	switch typtype {
	case "c":
		return "composite"
	case "e":
		return "enum"
	case "r":
		return "range"
	case "b":
		return "base"
	default:
		return typtype
	}
}

// --- domain ---

func (p *Provider) resolveDomain(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return p.resolveSchemaScoped(ctx, req,
		func(ctx context.Context, name string) ([]string, error) {
			return p.querySchemas(ctx, `
				select n.nspname from pg_catalog.pg_type t
				join pg_catalog.pg_namespace n on n.oid = t.typnamespace
				where t.typname = $1 and t.typtype = 'd'
				  and n.nspname not in ('pg_catalog', 'information_schema')
				order by n.nspname`, name)
		},
		p.confirmDomainInSchema,
	)
}

func (p *Provider) confirmDomainInSchema(ctx context.Context, schema, name string) (map[string]string, error) {
	var dummy int
	err := p.db.QueryRowContext(ctx, `
		select 1 from pg_catalog.pg_type t
		join pg_catalog.pg_namespace n on n.oid = t.typnamespace
		where t.typname = $1 and n.nspname = $2 and t.typtype = 'd'`,
		name, schema).Scan(&dummy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm domain %s.%s: %w", schema, name, err)
	}
	return map[string]string{}, nil
}

// --- extension (global) ---

func (p *Provider) resolveExtension(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	var version string
	err := p.db.QueryRowContext(ctx,
		`select e.extversion from pg_catalog.pg_extension e where e.extname = $1`,
		req.Name).Scan(&version)
	if err == sql.ErrNoRows {
		return &spec.ObjectSnapshot{
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusNotFound,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm extension %s: %w", req.Name, err)
	}
	return &spec.ObjectSnapshot{
		Type:       req.Type,
		Name:       req.Name,
		Status:     spec.MetadataStatusConfirmed,
		Exists:     true,
		Attributes: map[string]string{"extension_version": version},
	}, nil
}

// --- publication (global) ---

func (p *Provider) resolvePublication(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	var dummy int
	err := p.db.QueryRowContext(ctx,
		`select 1 from pg_catalog.pg_publication where pubname = $1`,
		req.Name).Scan(&dummy)
	if err == sql.ErrNoRows {
		return &spec.ObjectSnapshot{
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusNotFound,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm publication %s: %w", req.Name, err)
	}
	return &spec.ObjectSnapshot{
		Type:   req.Type,
		Name:   req.Name,
		Status: spec.MetadataStatusConfirmed,
		Exists: true,
	}, nil
}

// --- subscription (global, no conninfo) ---

func (p *Provider) resolveSubscription(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	var enabled bool
	err := p.db.QueryRowContext(ctx,
		`select subenabled from pg_catalog.pg_subscription where subname = $1`,
		req.Name).Scan(&enabled)
	if err == sql.ErrNoRows {
		return &spec.ObjectSnapshot{
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusNotFound,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm subscription %s: %w", req.Name, err)
	}
	return &spec.ObjectSnapshot{
		Type:       req.Type,
		Name:       req.Name,
		Status:     spec.MetadataStatusConfirmed,
		Exists:     true,
		Attributes: map[string]string{"enabled": fmt.Sprintf("%t", enabled)},
	}, nil
}

// --- foreign_table (schema-scoped, with server attribute) ---

func (p *Provider) resolveForeignTable(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return p.resolveSchemaScoped(ctx, req,
		func(ctx context.Context, name string) ([]string, error) {
			return p.querySchemas(ctx, `
				select n.nspname from pg_catalog.pg_class c
				join pg_catalog.pg_namespace n on n.oid = c.relnamespace
				where c.relname = $1 and c.relkind = 'f'
				  and n.nspname not in ('pg_catalog', 'information_schema')
				order by n.nspname`, name)
		},
		p.confirmForeignTableInSchema,
	)
}

func (p *Provider) confirmForeignTableInSchema(ctx context.Context, schema, name string) (map[string]string, error) {
	var serverName string
	err := p.db.QueryRowContext(ctx, `
		select fs.srvname from pg_catalog.pg_class c
		join pg_catalog.pg_namespace n on n.oid = c.relnamespace
		join pg_catalog.pg_foreign_table ft on ft.ftrelid = c.oid
		join pg_catalog.pg_foreign_server fs on fs.oid = ft.ftserver
		where c.relname = $1 and n.nspname = $2 and c.relkind = 'f'`,
		name, schema).Scan(&serverName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm foreign table %s.%s: %w", schema, name, err)
	}
	return map[string]string{"server": serverName}, nil
}

// --- foreign_server (global, has_options only) ---

func (p *Provider) resolveForeignServer(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	var hasOptions bool
	err := p.db.QueryRowContext(ctx, `
		select srvoptions is not null from pg_catalog.pg_foreign_server
		where srvname = $1`, req.Name).Scan(&hasOptions)
	if err == sql.ErrNoRows {
		return &spec.ObjectSnapshot{
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusNotFound,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm foreign server %s: %w", req.Name, err)
	}
	return &spec.ObjectSnapshot{
		Type:       req.Type,
		Name:       req.Name,
		Status:     spec.MetadataStatusConfirmed,
		Exists:     true,
		Attributes: map[string]string{"has_options": fmt.Sprintf("%t", hasOptions)},
	}, nil
}

// --- user_mapping (global, user@server, has_options only) ---

func (p *Provider) resolveUserMapping(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	user, server := splitUserMappingName(req.Name)
	if user == "" || server == "" {
		return &spec.ObjectSnapshot{
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusUnavailable,
		}, nil
	}
	var hasOptions bool
	err := p.db.QueryRowContext(ctx, `
		select um.umoptions is not null from pg_catalog.pg_user_mapping um
		join pg_catalog.pg_authid a on a.oid = um.umuser
		join pg_catalog.pg_foreign_server srv on srv.oid = um.umserver
		where a.rolname = $1 and srv.srvname = $2`,
		user, server).Scan(&hasOptions)
	if err == sql.ErrNoRows {
		return &spec.ObjectSnapshot{
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusNotFound,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm user mapping %s: %w", req.Name, err)
	}
	return &spec.ObjectSnapshot{
		Type:       req.Type,
		Name:       req.Name,
		Status:     spec.MetadataStatusConfirmed,
		Exists:     true,
		Attributes: map[string]string{"has_options": fmt.Sprintf("%t", hasOptions)},
	}, nil
}

func splitUserMappingName(name string) (user, server string) {
	parts := strings.SplitN(name, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return name, ""
}

// --- foreign_data_wrapper (global, has_options only) ---

func (p *Provider) resolveFDW(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	var hasOptions bool
	err := p.db.QueryRowContext(ctx, `
		select fdwoptions is not null from pg_catalog.pg_foreign_data_wrapper
		where fdwname = $1`, req.Name).Scan(&hasOptions)
	if err == sql.ErrNoRows {
		return &spec.ObjectSnapshot{
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusNotFound,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm fdw %s: %w", req.Name, err)
	}
	return &spec.ObjectSnapshot{
		Type:       req.Type,
		Name:       req.Name,
		Status:     spec.MetadataStatusConfirmed,
		Exists:     true,
		Attributes: map[string]string{"has_options": fmt.Sprintf("%t", hasOptions)},
	}, nil
}

// --- event_trigger (global, enabled only) ---

func (p *Provider) resolveEventTrigger(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	var evtenabled string
	err := p.db.QueryRowContext(ctx, `
		select evtenabled from pg_catalog.pg_event_trigger
		where evtname = $1`, req.Name).Scan(&evtenabled)
	if err == sql.ErrNoRows {
		return &spec.ObjectSnapshot{
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusNotFound,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm event trigger %s: %w", req.Name, err)
	}
	return &spec.ObjectSnapshot{
		Type:       req.Type,
		Name:       req.Name,
		Status:     spec.MetadataStatusConfirmed,
		Exists:     true,
		Attributes: map[string]string{"enabled": eventTriggerEnabled(evtenabled)},
	}, nil
}

func eventTriggerEnabled(code string) string {
	if code == "O" {
		return "true"
	}
	return "false"
}

// --- rule (schema + table scoped) ---

func (p *Provider) resolveRule(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	tableName := ""
	if req.Qualifiers != nil {
		tableName = strings.TrimSpace(req.Qualifiers["table"])
	}

	if req.Schema != "" && tableName != "" {
		return p.resolveRuleExact(ctx, req, tableName)
	}
	return p.resolveRuleAmbiguous(ctx, req, tableName)
}

func (p *Provider) resolveRuleExact(ctx context.Context, req spec.ObjectLookupRequest, tableName string) (*spec.ObjectSnapshot, error) {
	var dummy int
	err := p.db.QueryRowContext(ctx, `
		select 1 from pg_catalog.pg_rewrite r
		join pg_catalog.pg_class c on c.oid = r.ev_class
		join pg_catalog.pg_namespace n on n.oid = c.relnamespace
		where r.rulename = $1 and n.nspname = $2 and c.relname = $3`,
		req.Name, req.Schema, tableName).Scan(&dummy)
	if err == sql.ErrNoRows {
		return &spec.ObjectSnapshot{
			Schema: req.Schema,
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusNotFound,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm rule %s on %s.%s: %w", req.Name, req.Schema, tableName, err)
	}
	return &spec.ObjectSnapshot{
		Schema:     req.Schema,
		Type:       req.Type,
		Name:       req.Name,
		Status:     spec.MetadataStatusConfirmed,
		Exists:     true,
		Attributes: map[string]string{"table": tableName},
	}, nil
}

func (p *Provider) resolveRuleAmbiguous(ctx context.Context, req spec.ObjectLookupRequest, tableName string) (*spec.ObjectSnapshot, error) {
	query := `
		select distinct n.nspname from pg_catalog.pg_rewrite r
		join pg_catalog.pg_class c on c.oid = r.ev_class
		join pg_catalog.pg_namespace n on n.oid = c.relnamespace
		where r.rulename = $1
		  and n.nspname not in ('pg_catalog', 'information_schema')`
	args := []interface{}{req.Name}

	if req.Schema != "" {
		query += ` and n.nspname = $2`
		args = append(args, req.Schema)
	}
	if tableName != "" {
		query += fmt.Sprintf(` and c.relname = $%d`, len(args)+1)
		args = append(args, tableName)
	}
	query += ` order by n.nspname`

	schemas, err := p.querySchemas(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find schemas for rule %s: %w", req.Name, err)
	}

	switch len(schemas) {
	case 0:
		return &spec.ObjectSnapshot{
			Schema: req.Schema,
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusNotFound,
		}, nil
	case 1:
		return &spec.ObjectSnapshot{
			Schema: schemas[0],
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusConfirmed,
			Exists: true,
		}, nil
	default:
		return &spec.ObjectSnapshot{
			Type:                req.Type,
			Name:                req.Name,
			Status:              spec.MetadataStatusAmbiguous,
			AmbiguousCandidates: schemas,
		}, nil
	}
}

// --- schema (exact name) ---

func (p *Provider) resolveSchema(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	var dummy int
	err := p.db.QueryRowContext(ctx, `
		select 1 from pg_catalog.pg_namespace
		where nspname = $1 and nspname not in ('pg_catalog', 'information_schema')`,
		req.Name).Scan(&dummy)
	if err == sql.ErrNoRows {
		return &spec.ObjectSnapshot{
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusNotFound,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("confirm schema %s: %w", req.Name, err)
	}
	return &spec.ObjectSnapshot{
		Type:   req.Type,
		Name:   req.Name,
		Status: spec.MetadataStatusConfirmed,
		Exists: true,
	}, nil
}

// --- sequence (schema-scoped) ---

func (p *Provider) resolveSequence(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return p.resolveSchemaScoped(ctx, req,
		func(ctx context.Context, name string) ([]string, error) {
			return p.querySchemas(ctx, `
				select n.nspname from pg_catalog.pg_class c
				join pg_catalog.pg_namespace n on n.oid = c.relnamespace
				where c.relname = $1 and c.relkind = 'S'
				  and n.nspname not in ('pg_catalog', 'information_schema')
				order by n.nspname`, name)
		},
		func(ctx context.Context, schema, name string) (map[string]string, error) {
			var dummy int
			err := p.db.QueryRowContext(ctx, `
				select 1 from pg_catalog.pg_class c
				join pg_catalog.pg_namespace n on n.oid = c.relnamespace
				where c.relname = $1 and n.nspname = $2 and c.relkind = 'S'`,
				name, schema).Scan(&dummy)
			if err == sql.ErrNoRows {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return map[string]string{}, nil
		},
	)
}

// --- materialized_view (schema-scoped) ---

func (p *Provider) resolveMaterializedView(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return p.resolveSchemaScoped(ctx, req,
		func(ctx context.Context, name string) ([]string, error) {
			return p.querySchemas(ctx, `
				select n.nspname from pg_catalog.pg_class c
				join pg_catalog.pg_namespace n on n.oid = c.relnamespace
				where c.relname = $1 and c.relkind = 'm'
				  and n.nspname not in ('pg_catalog', 'information_schema')
				order by n.nspname`, name)
		},
		func(ctx context.Context, schema, name string) (map[string]string, error) {
			var dummy int
			err := p.db.QueryRowContext(ctx, `
				select 1 from pg_catalog.pg_class c
				join pg_catalog.pg_namespace n on n.oid = c.relnamespace
				where c.relname = $1 and n.nspname = $2 and c.relkind = 'm'`,
				name, schema).Scan(&dummy)
			if err == sql.ErrNoRows {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return map[string]string{}, nil
		},
	)
}

// --- annotation targets (comment / security_label) ---

func (p *Provider) resolveAnnotationTarget(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Schema != "" {
		return p.resolveAnnotationInSchema(ctx, req)
	}
	return p.resolveAnnotationUnqualified(ctx, req)
}

func (p *Provider) resolveAnnotationInSchema(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	var relkind string
	err := p.db.QueryRowContext(ctx, `
		select c.relkind from pg_catalog.pg_class c
		join pg_catalog.pg_namespace n on n.oid = c.relnamespace
		where c.relname = $1 and n.nspname = $2
		  and n.nspname not in ('pg_catalog', 'information_schema')`,
		req.Name, req.Schema).Scan(&relkind)

	if err == nil {
		return &spec.ObjectSnapshot{
			Schema:     req.Schema,
			Type:       req.Type,
			Name:       req.Name,
			Status:     spec.MetadataStatusConfirmed,
			Exists:     true,
			Attributes: map[string]string{"target_type": classTargetType(relkind)},
		}, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("confirm annotation target %s.%s: %w", req.Schema, req.Name, err)
	}

	// Not in pg_class — check pg_namespace (schema target).
	var dummy int
	err = p.db.QueryRowContext(ctx, `
		select 1 from pg_catalog.pg_namespace
		where nspname = $1 and nspname not in ('pg_catalog', 'information_schema')`,
		req.Name).Scan(&dummy)
	if err == nil {
		return &spec.ObjectSnapshot{
			Schema:     req.Schema,
			Type:       req.Type,
			Name:       req.Name,
			Status:     spec.MetadataStatusConfirmed,
			Exists:     true,
			Attributes: map[string]string{"target_type": "schema"},
		}, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("confirm annotation schema target %s: %w", req.Name, err)
	}

	return &spec.ObjectSnapshot{
		Schema: req.Schema,
		Type:   req.Type,
		Name:   req.Name,
		Status: spec.MetadataStatusNotFound,
	}, nil
}

func (p *Provider) resolveAnnotationUnqualified(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	schemas, err := p.querySchemas(ctx, `
		select n.nspname from pg_catalog.pg_class c
		join pg_catalog.pg_namespace n on n.oid = c.relnamespace
		where c.relname = $1
		  and n.nspname not in ('pg_catalog', 'information_schema')
		order by n.nspname`, req.Name)
	if err != nil {
		return nil, fmt.Errorf("find annotation target schemas: %w", err)
	}

	// Also check pg_namespace.
	var nsSchema string
	_ = p.db.QueryRowContext(ctx, `
		select nspname from pg_catalog.pg_namespace
		where nspname = $1 and nspname not in ('pg_catalog', 'information_schema')`,
		req.Name).Scan(&nsSchema)
	if nsSchema != "" {
		schemas = append(schemas, nsSchema)
	}

	switch len(schemas) {
	case 0:
		return &spec.ObjectSnapshot{
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusNotFound,
		}, nil
	case 1:
		return &spec.ObjectSnapshot{
			Schema: schemas[0],
			Type:   req.Type,
			Name:   req.Name,
			Status: spec.MetadataStatusConfirmed,
			Exists: true,
		}, nil
	default:
		return &spec.ObjectSnapshot{
			Type:                req.Type,
			Name:                req.Name,
			Status:              spec.MetadataStatusAmbiguous,
			AmbiguousCandidates: schemas,
		}, nil
	}
}

func classTargetType(relkind string) string {
	switch relkind {
	case "r", "p":
		return "table"
	case "f":
		return "foreign_table"
	case "S":
		return "sequence"
	case "m":
		return "materialized_view"
	case "v":
		return "view"
	default:
		return relkind
	}
}
