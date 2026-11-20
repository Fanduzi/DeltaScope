package postgresqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveColumnTraversal(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	table := req.Qualifiers["table"]
	if req.Schema == "" || req.Schema == "pg_temp" || req.Name == "" || table == "" {
		return nil, nil
	}
	facts := spec.PGColumnTraversal{Schema: req.Schema, Table: table, Column: req.Name}
	err := p.db.QueryRowContext(ctx, `
        select a.attinhcount OPERATOR(pg_catalog.>) 0,
               coalesce(a.attnum OPERATOR(pg_catalog.=) any(pt.partattrs), false),
               exists (
                   select 1 from pg_catalog.pg_inherits h
                   where h.inhparent OPERATOR(pg_catalog.=) c.oid and not h.inhdetachpending)
        from pg_catalog.pg_namespace n
        join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
        join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) c.oid
        left join pg_catalog.pg_partitioned_table pt on pt.partrelid OPERATOR(pg_catalog.=) c.oid
        where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
          and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
          and a.attname OPERATOR(pg_catalog.=) $3::pg_catalog.text
          and a.attnum OPERATOR(pg_catalog.>) 0 and not a.attisdropped
          and c.relkind OPERATOR(pg_catalog.=) any(array['r','p']::pg_catalog."char"[])
          and c.relpersistence OPERATOR(pg_catalog.=) 'p'
          and c.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
    `, req.Schema, table, req.Name).Scan(&facts.Inherited, &facts.DirectPartitionKey, &facts.AttachedChildren)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL column traversal: %w", err)
	}
	return &spec.ObjectSnapshot{PGColumnTraversal: &facts}, nil
}
