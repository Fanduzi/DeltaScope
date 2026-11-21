package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveMaterializedViewOptions(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	if !target.Qualified() {
		return nil, nil
	}
	state := spec.PGDropViewState{Target: target}
	err := p.db.QueryRowContext(ctx, `
		select coalesce(c.relkind::pg_catalog.text, ''),
		       coalesce(am.amname OPERATOR(pg_catalog.=) 'heap' and am.amtype OPERATOR(pg_catalog.=) 't', false),
		       coalesce(c.reltoastrelid OPERATOR(pg_catalog.<>) 0, false)
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
		 and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_am am on am.oid OPERATOR(pg_catalog.=) c.relam
	`, req.Schema, req.Name).Scan(&state.Kind, &state.Heap, &state.HasToast)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL materialized view option target: %w", err)
	}
	return &spec.ObjectSnapshot{PGDropViewState: &state}, nil
}
