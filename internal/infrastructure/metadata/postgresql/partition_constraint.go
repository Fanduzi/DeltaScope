package postgresqlmeta

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolvePartitionConstraint(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	if !target.Qualified() {
		return nil, nil
	}
	var columns, system, keys []byte
	err := p.db.QueryRowContext(ctx, `
		with target as (
		    select c.oid, pt.partattrs, pt.partclass, pt.partcollation,
		           case when pt.partstrat OPERATOR(pg_catalog.=) 'h' then 1 else 3 end as equality_strategy
		    from pg_catalog.pg_class c
		    join pg_catalog.pg_namespace n on n.oid OPERATOR(pg_catalog.=) c.relnamespace
		    join pg_catalog.pg_partitioned_table pt on pt.partrelid OPERATOR(pg_catalog.=) c.oid
		    where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		      and c.relpersistence OPERATOR(pg_catalog.=) 'p' and c.relkind OPERATOR(pg_catalog.=) 'p'
		      and not c.relispartition and not c.relisshared
		), members as (
		    select a.attname, a.attnum, a.attcollation, a.atttypid,
		           exists (select 1 from pg_catalog.pg_constraint con where con.conrelid OPERATOR(pg_catalog.=) a.attrelid
		             and con.contype OPERATOR(pg_catalog.=) 'p' and a.attnum OPERATOR(pg_catalog.=) any(con.conkey)) as primary_key
		    from target t join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) t.oid
		    where a.attnum OPERATOR(pg_catalog.<>) 0 and not a.attisdropped
		), partition_keys as (
		    select slot.position, a.attname, slot.attnum,
		           a.attcollation OPERATOR(pg_catalog.=) slot.coll_oid as collation_ok,
		           partition_eq.amopopr is not null and index_eq.amopopr is not null as equality_read,
		           partition_eq.amopopr OPERATOR(pg_catalog.=) index_eq.amopopr as equality_ok
		    from target t
		    cross join lateral rows from (pg_catalog.unnest(t.partattrs), pg_catalog.unnest(t.partclass), pg_catalog.unnest(t.partcollation))
		        with ordinality as slot(attnum, opclass, coll_oid, position)
		    left join members a on a.attnum OPERATOR(pg_catalog.=) slot.attnum
		    left join pg_catalog.pg_opclass pc on pc.oid OPERATOR(pg_catalog.=) slot.opclass
		    left join pg_catalog.pg_amop partition_eq on partition_eq.amopfamily OPERATOR(pg_catalog.=) pc.opcfamily
		      and partition_eq.amoplefttype OPERATOR(pg_catalog.=) pc.opcintype and partition_eq.amoprighttype OPERATOR(pg_catalog.=) pc.opcintype
		      and partition_eq.amopstrategy OPERATOR(pg_catalog.=) t.equality_strategy
		    left join lateral (
		        select minclass.opcfamily, minclass.opcintype from pg_catalog.pg_opclass minclass
		        where minclass.opcmethod OPERATOR(pg_catalog.=) 403 and minclass.opcintype OPERATOR(pg_catalog.=) a.atttypid and minclass.opcdefault
		          and (select pg_catalog.count(*) from pg_catalog.pg_opclass other
		               where other.opcmethod OPERATOR(pg_catalog.=) 403 and other.opcintype OPERATOR(pg_catalog.=) a.atttypid and other.opcdefault) OPERATOR(pg_catalog.=) 1
		    ) dc on true
		    left join pg_catalog.pg_amop index_eq on index_eq.amopfamily OPERATOR(pg_catalog.=) dc.opcfamily
		      and index_eq.amoplefttype OPERATOR(pg_catalog.=) dc.opcintype and index_eq.amoprighttype OPERATOR(pg_catalog.=) dc.opcintype
		      and index_eq.amopstrategy OPERATOR(pg_catalog.=) 3
		)
		select coalesce((select pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('Name',attname,'PrimaryKey',primary_key))
		                 from members where attnum OPERATOR(pg_catalog.>) 0), '[]'::pg_catalog.jsonb),
		       coalesce((select pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('Name',attname,'System',true))
		                 from members where attnum OPERATOR(pg_catalog.<) 0), '[]'::pg_catalog.jsonb),
		       (select pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('name',coalesce(attname,''),
		           'expression',attnum OPERATOR(pg_catalog.=) 0,'collation_ok',coalesce(collation_ok,false),
		           'equality_read',equality_read,'equality_ok',coalesce(equality_ok,false)) order by position) from partition_keys)
		from target
	`, req.Schema, req.Name).Scan(&columns, &system, &keys)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL partition constraint facts: %w", err)
	}
	state := spec.PGPartitionConstraintState{Target: target.Table}
	for _, item := range []struct {
		data []byte
		dest any
	}{{columns, &state.Columns}, {system, &state.SystemColumns}, {keys, &state.PartitionKeys}} {
		if err := json.Unmarshal(item.data, item.dest); err != nil {
			return nil, fmt.Errorf("decode PostgreSQL partition constraint facts: %w", err)
		}
	}
	return &spec.ObjectSnapshot{PGPartitionConstraint: &state}, nil
}
