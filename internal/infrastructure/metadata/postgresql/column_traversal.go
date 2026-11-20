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
        select coalesce(a.attinhcount OPERATOR(pg_catalog.>) 0, false),
               coalesce(a.attnum OPERATOR(pg_catalog.=) any(pt.partattrs), false),
               exists (
                   select 1 from pg_catalog.pg_inherits h
                   where h.inhparent OPERATOR(pg_catalog.=) c.oid and not h.inhdetachpending),
               c.relispartition, a.attnum is not null,
               coalesce(a.attnum OPERATOR(pg_catalog.<) 0, false),
               coalesce(a.attgenerated OPERATOR(pg_catalog.=) 's', false),
               case when $5::pg_catalog.text OPERATOR(pg_catalog.=) 'drop_not_null' and c.relispartition then exists (
                   select 1 from pg_catalog.pg_inherits h
                   join pg_catalog.pg_attribute parent on parent.attrelid OPERATOR(pg_catalog.=) h.inhparent
                    and parent.attname OPERATOR(pg_catalog.=) a.attname
                    and parent.attnum OPERATOR(pg_catalog.>) 0 and not parent.attisdropped
                   where h.inhrelid OPERATOR(pg_catalog.=) c.oid and not h.inhdetachpending and parent.attnotnull
               ) else false end,
               case when $5::pg_catalog.text OPERATOR(pg_catalog.=) 'set_not_null' and $6::pg_catalog.bool
                    and c.relkind OPERATOR(pg_catalog.=) 'p' and not a.attnotnull then exists (
                   with recursive descendants(oid) as (
                       select c.oid
                       union
                       select h.inhrelid from pg_catalog.pg_inherits h join descendants d on d.oid OPERATOR(pg_catalog.=) h.inhparent
                       where not h.inhdetachpending
                   )
                   select 1 from descendants d join pg_catalog.pg_attribute child on child.attrelid OPERATOR(pg_catalog.=) d.oid
                    and child.attname OPERATOR(pg_catalog.=) a.attname and child.attnum OPERATOR(pg_catalog.>) 0 and not child.attisdropped
                   where d.oid OPERATOR(pg_catalog.<>) c.oid and not child.attnotnull
               ) else false end
        from pg_catalog.pg_namespace n
        join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
        left join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) c.oid
         and a.attname OPERATOR(pg_catalog.=) $3::pg_catalog.text
         and a.attnum OPERATOR(pg_catalog.<>) 0 and not a.attisdropped
        left join pg_catalog.pg_partitioned_table pt on pt.partrelid OPERATOR(pg_catalog.=) c.oid
        where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
          and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
          and ($4::pg_catalog.bool or a.attnum OPERATOR(pg_catalog.>) 0)
          and c.relkind OPERATOR(pg_catalog.=) any(array['r','p']::pg_catalog."char"[])
          and c.relpersistence OPERATOR(pg_catalog.=) 'p'
          and c.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
    `, req.Schema, table, req.Name, req.Qualifiers["action"] == "add_column", req.Qualifiers["action"], req.Qualifiers["only"] == "true").Scan(
		&facts.Inherited, &facts.DirectPartitionKey, &facts.AttachedChildren, &facts.Partition, &facts.ColumnExists, &facts.System, &facts.StoredGenerated, &facts.ParentNotNull, &facts.NullableDescendant)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL column traversal: %w", err)
	}
	return &spec.ObjectSnapshot{PGColumnTraversal: &facts}, nil
}
