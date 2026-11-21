package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveMaterializedViewExtension(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	state := spec.PGMaterializedViewExtensionState{Schema: req.Schema, View: req.Name, Extension: req.Qualifiers["extension"]}
	if state.Schema == "" || state.View == "" || state.Extension == "" {
		return nil, nil
	}
	err := p.db.QueryRowContext(ctx, `
		select coalesce(c.relkind::pg_catalog.text, ''), e.oid is not null
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
		 and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_extension e on e.extname OPERATOR(pg_catalog.=) $3::pg_catalog.text
	`, state.Schema, state.View, state.Extension).Scan(&state.Kind, &state.ExtensionExists)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL materialized view extension state: %w", err)
	}
	return &spec.ObjectSnapshot{PGMaterializedViewExtension: &state}, nil
}
