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
	facts := spec.PGColumnTraversal{Schema: req.Schema, Table: table, Column: req.Name, RenameDestination: req.Qualifiers["destination"]}
	var descendantConflict sql.NullBool
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
               ) else false end,
               case when $5::pg_catalog.text OPERATOR(pg_catalog.=) 'rename_column' then
                   coalesce(pg_catalog.sign(destination.attnum)::pg_catalog.int4, 0) else 0 end,
               case when $5::pg_catalog.text OPERATOR(pg_catalog.=) any(array['rename_column', 'set_data_type']::pg_catalog.text[])
                      and not $6::pg_catalog.bool and a.attnum OPERATOR(pg_catalog.>) 0
                      and ($5::pg_catalog.text OPERATOR(pg_catalog.=) 'set_data_type' or $7::pg_catalog.text OPERATOR(pg_catalog.<>) '') then (
                   with recursive tree(oid) as (
                       select c.oid
                       union
                       select h.inhrelid from tree parent
                       join pg_catalog.pg_inherits h on h.inhparent OPERATOR(pg_catalog.=) parent.oid
                       where not h.inhdetachpending
                   )
                   select case when pg_catalog.count(*) OPERATOR(pg_catalog.=) 0 then false
                               when pg_catalog.bool_and(child.relkind OPERATOR(pg_catalog.=) any(array['r','p']::pg_catalog."char"[])
                                    and child.relpersistence OPERATOR(pg_catalog.=) 'p' and not child.relisshared)
                               then pg_catalog.bool_or(
                                    ($5::pg_catalog.text OPERATOR(pg_catalog.=) 'rename_column'
                                     and (source.attnum is null or target.attnum is not null)) or
                                    source.attinhcount OPERATOR(pg_catalog.>) (
                                        select pg_catalog.count(*) from pg_catalog.pg_inherits h
                                        join tree parent on parent.oid OPERATOR(pg_catalog.=) h.inhparent
                                        where h.inhrelid OPERATOR(pg_catalog.=) d.oid and not h.inhdetachpending)) end
                   from tree d join pg_catalog.pg_class child on child.oid OPERATOR(pg_catalog.=) d.oid
                   left join pg_catalog.pg_attribute source on source.attrelid OPERATOR(pg_catalog.=) d.oid
                    and source.attname OPERATOR(pg_catalog.=) a.attname and source.attnum OPERATOR(pg_catalog.>) 0 and not source.attisdropped
                   left join pg_catalog.pg_attribute target on target.attrelid OPERATOR(pg_catalog.=) d.oid
                    and target.attname OPERATOR(pg_catalog.=) $7::pg_catalog.text and target.attnum OPERATOR(pg_catalog.<>) 0 and not target.attisdropped
                   where d.oid OPERATOR(pg_catalog.<>) c.oid
               ) end,
               case when $5::pg_catalog.text OPERATOR(pg_catalog.=) 'set_not_null'
                    and c.relkind OPERATOR(pg_catalog.=) 'r' and not c.relispartition and not c.relisshared
                    and a.attnum OPERATOR(pg_catalog.>) 0 and not a.attnotnull
                    and a.atttypid OPERATOR(pg_catalog.=) 'pg_catalog.int4'::pg_catalog.regtype
                    and not exists (select 1 from pg_catalog.pg_inherits h
                        where h.inhrelid OPERATOR(pg_catalog.=) c.oid or h.inhparent OPERATOR(pg_catalog.=) c.oid)
               then exists (
                   select 1 from pg_catalog.pg_constraint ck
                   where ck.conrelid OPERATOR(pg_catalog.=) c.oid and ck.contype OPERATOR(pg_catalog.=) 'c'
                     and ck.convalidated and ck.conkey OPERATOR(pg_catalog.=) array[a.attnum]
                     and pg_catalog.pg_get_expr(ck.conbin, ck.conrelid, true) collate pg_catalog."C"
                         OPERATOR(pg_catalog.=) pg_catalog.format('%I IS NOT NULL', a.attname)
               ) else false end,
               ($5::pg_catalog.text OPERATOR(pg_catalog.=) any(array['add_identity','drop_identity','set_generated']::pg_catalog.text[])
                and not c.relisshared
                and ((c.relkind OPERATOR(pg_catalog.=) 'r' and c.relispartition)
                  or (c.relkind OPERATOR(pg_catalog.=) 'p' and not c.relispartition and $6::pg_catalog.bool)))
        from pg_catalog.pg_namespace n
        join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
        left join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) c.oid
         and a.attname OPERATOR(pg_catalog.=) $3::pg_catalog.text
         and a.attnum OPERATOR(pg_catalog.<>) 0 and not a.attisdropped
        left join pg_catalog.pg_partitioned_table pt on pt.partrelid OPERATOR(pg_catalog.=) c.oid
        left join pg_catalog.pg_attribute destination on destination.attrelid OPERATOR(pg_catalog.=) c.oid
         and destination.attname OPERATOR(pg_catalog.=) $7::pg_catalog.text
         and destination.attnum OPERATOR(pg_catalog.<>) 0 and not destination.attisdropped
        where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
          and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
          and ($4::pg_catalog.bool or a.attnum OPERATOR(pg_catalog.>) 0
               or ($5::pg_catalog.text OPERATOR(pg_catalog.=) 'rename_column' and $6::pg_catalog.bool)
               or $5::pg_catalog.text OPERATOR(pg_catalog.=) any(array['add_identity','drop_identity','set_generated']::pg_catalog.text[]))
          and c.relkind OPERATOR(pg_catalog.=) any(array['r','p']::pg_catalog."char"[])
          and c.relpersistence OPERATOR(pg_catalog.=) 'p'
          and c.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
    `, req.Schema, table, req.Name, req.Qualifiers["action"] == "add_column", req.Qualifiers["action"], req.Qualifiers["only"] == "true", facts.RenameDestination).Scan(
		&facts.Inherited, &facts.DirectPartitionKey, &facts.AttachedChildren, &facts.Partition, &facts.ColumnExists, &facts.System, &facts.StoredGenerated, &facts.ParentNotNull, &facts.NullableDescendant, &facts.RenameDestinationKind, &descendantConflict, &facts.ValidatedNotNullCheck, &facts.IdentityChangeBlocked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL column traversal: %w", err)
	}
	if req.Qualifiers["action"] == "set_data_type" {
		facts.TypeDescendantConflict = descendantConflict.Valid && descendantConflict.Bool
	} else {
		facts.RenameDescendantsKnown, facts.RenameDescendantConflict = descendantConflict.Valid, descendantConflict.Bool
	}
	return &spec.ObjectSnapshot{PGColumnTraversal: &facts}, nil
}
