package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveIndexOptions(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	if !target.Qualified() {
		return nil, nil
	}
	state := spec.PGIndexOptionsState{Target: target}
	err := p.db.QueryRowContext(ctx, `
		select coalesce(c.relkind::pg_catalog.text, ''),
		       case when am.oid OPERATOR(pg_catalog.=) 403 and am.amname OPERATOR(pg_catalog.=) 'btree' then 'btree'
		            when am.oid OPERATOR(pg_catalog.=) 405 and am.amname OPERATOR(pg_catalog.=) 'hash' then 'hash'
		            else '' end
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
		 and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_am am on am.oid OPERATOR(pg_catalog.=) c.relam and am.amtype OPERATOR(pg_catalog.=) 'i'
	`, req.Schema, req.Name).Scan(&state.Kind, &state.Method)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL index option target: %w", err)
	}
	return &spec.ObjectSnapshot{PGIndexOptions: &state}, nil
}
