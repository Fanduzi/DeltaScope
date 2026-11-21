package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveRefreshMaterializedView(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGRefreshTarget{Schema: req.Schema, Name: req.Name}
	if !target.Qualified() {
		return nil, nil
	}
	facts := spec.PGRefreshState{Target: target}
	err := p.db.QueryRowContext(ctx, `
        select coalesce(c.relkind::pg_catalog.text, ''), coalesce(c.relispopulated, false),
               exists (
                   select 1 from pg_catalog.pg_index i
                   join pg_catalog.pg_class idx on idx.oid OPERATOR(pg_catalog.=) i.indexrelid
                   join pg_catalog.pg_am am on am.oid OPERATOR(pg_catalog.=) idx.relam
                   where i.indrelid OPERATOR(pg_catalog.=) c.oid
                     and i.indislive and i.indisvalid and i.indisunique and i.indimmediate
                     and am.amname OPERATOR(pg_catalog.=) 'btree'::pg_catalog.text
                     and i.indpred is null and i.indnatts OPERATOR(pg_catalog.>) 0
                     and 0 OPERATOR(pg_catalog.<) all(i.indkey)
               )
        from (values (1)) as witness(dummy)
        left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
        left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
         and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
    `, req.Schema, req.Name).Scan(&facts.Kind, &facts.Populated, &facts.UniqueIndex)
	if err != nil {
		return nil, fmt.Errorf("read materialized view refresh prerequisites: %w", err)
	}
	return &spec.ObjectSnapshot{PGRefreshState: &facts}, nil
}
