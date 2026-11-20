package postgresqlmeta

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveForeignKeyEquality(ctx context.Context, target *spec.PGForeignKeyEqualityTarget) (*spec.ObjectSnapshot, error) {
	if target == nil || target.Source.Schema == "" || target.Source.Name == "" || target.Reference.Schema == "" || target.Reference.Name == "" || len(target.Columns) == 0 {
		return nil, nil
	}
	names, err := json.Marshal(target.Columns)
	if err != nil {
		return nil, fmt.Errorf("encode foreign key source columns: %w", err)
	}
	ref, err := json.Marshal(append([]string{}, target.ReferencedColumns...))
	if err != nil {
		return nil, fmt.Errorf("encode foreign key referenced columns: %w", err)
	}
	var position sql.NullInt64
	err = p.db.QueryRowContext(ctx, `
		with recursive request as (
		    select $1::pg_catalog.text as source_schema, $2::pg_catalog.text as source_table,
		           $3::pg_catalog.text as referenced_schema, $4::pg_catalog.text as referenced_table,
		           $5::pg_catalog.jsonb as source_columns, $6::pg_catalog.jsonb as referenced_columns
		), pair as (
		    select source.oid as source_oid, referenced.oid as referenced_oid, r.source_columns, r.referenced_columns
		    from request r
		    join pg_catalog.pg_namespace sn on sn.nspname OPERATOR(pg_catalog.=) r.source_schema
		    join pg_catalog.pg_class source on source.relnamespace OPERATOR(pg_catalog.=) sn.oid
		     and source.relname OPERATOR(pg_catalog.=) r.source_table
		    join pg_catalog.pg_namespace rn on rn.nspname OPERATOR(pg_catalog.=) r.referenced_schema
		    join pg_catalog.pg_class referenced on referenced.relnamespace OPERATOR(pg_catalog.=) rn.oid
		     and referenced.relname OPERATOR(pg_catalog.=) r.referenced_table
		    where source.relkind OPERATOR(pg_catalog.=) 'r' and referenced.relkind OPERATOR(pg_catalog.=) 'r'
		      and source.relpersistence OPERATOR(pg_catalog.=) 'p' and referenced.relpersistence OPERATOR(pg_catalog.=) 'p'
		      and not source.relispartition and not referenced.relispartition
		      and source.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		      and referenced.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		      and not exists (select 1 from pg_catalog.pg_inherits h
		                      where h.inhrelid OPERATOR(pg_catalog.=) any(array[source.oid, referenced.oid])
		                         or h.inhparent OPERATOR(pg_catalog.=) any(array[source.oid, referenced.oid]))
		), key_slots as (
		    select p.source_oid, p.source_columns, p.referenced_columns, i.indexrelid, i.indnkeyatts, i.indisprimary,
		           a.attname::pg_catalog.text as column_name, a.atttypid as pk_type, slots.opclass, slots.position
		    from pair p
		    join pg_catalog.pg_index i on i.indrelid OPERATOR(pg_catalog.=) p.referenced_oid
		     and i.indislive and i.indisvalid and i.indisunique and i.indimmediate
		     and i.indpred is null and i.indexprs is null
		    cross join lateral rows from (pg_catalog.unnest(i.indkey), pg_catalog.unnest(i.indclass))
		        with ordinality as slots(attnum, opclass, position)
		    join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) p.referenced_oid
		     and a.attnum OPERATOR(pg_catalog.=) slots.attnum and not a.attisdropped
		    where slots.position OPERATOR(pg_catalog.<=) i.indnkeyatts and a.attnum OPERATOR(pg_catalog.>) 0
		), selected as (
		    select k.indexrelid
		    from key_slots k
		    group by k.indexrelid, k.source_columns, k.referenced_columns, k.indnkeyatts, k.indisprimary
		    having pg_catalog.count(*) OPERATOR(pg_catalog.=) k.indnkeyatts
		       and k.indnkeyatts OPERATOR(pg_catalog.=) pg_catalog.jsonb_array_length(k.source_columns)
		       and (case when pg_catalog.jsonb_array_length(k.referenced_columns) OPERATOR(pg_catalog.=) 0 then k.indisprimary
		                 else pg_catalog.jsonb_array_length(k.referenced_columns) OPERATOR(pg_catalog.=) k.indnkeyatts
		                  and pg_catalog.jsonb_agg(k.column_name) OPERATOR(pg_catalog.@>) k.referenced_columns
		                  and k.referenced_columns OPERATOR(pg_catalog.@>) pg_catalog.jsonb_agg(k.column_name) end)
		    order by k.indexrelid limit 1
		), slots_with_types as (
		    select pos.position, k.pk_type, a.atttypid as fk_type, k.opclass
		    from selected i
		    join key_slots k on k.indexrelid OPERATOR(pg_catalog.=) i.indexrelid
		    cross join lateral pg_catalog.jsonb_array_elements_text(k.source_columns) with ordinality as pos(name, position)
		    join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) k.source_oid
		     and a.attname OPERATOR(pg_catalog.=) pos.name and a.attnum OPERATOR(pg_catalog.>) 0 and not a.attisdropped
		    where case when pg_catalog.jsonb_array_length(k.referenced_columns) OPERATOR(pg_catalog.=) 0
		              then k.position OPERATOR(pg_catalog.=) pos.position
		              else k.column_name OPERATOR(pg_catalog.=) (k.referenced_columns OPERATOR(pg_catalog.->>) cast(pos.position OPERATOR(pg_catalog.-) 1 as pg_catalog.int4)) end
		), types(original, oid, base) as (
		    select t.oid, t.oid, t.typbasetype
		    from pg_catalog.pg_type t join (select pk_type as oid from slots_with_types union select fk_type from slots_with_types) s
		      on s.oid OPERATOR(pg_catalog.=) t.oid
		    union
		    select c.original, t.oid, t.typbasetype
		    from types c join pg_catalog.pg_type t on t.oid OPERATOR(pg_catalog.=) c.base
		), comparison as (
		    select s.position,
		        case when opc.opcmethod OPERATOR(pg_catalog.=) 403::pg_catalog.oid and ppeq.oid is not null
		             then case when pfeq.oid is not null and ffeq.oid is not null then true
		                  when ot.oid OPERATOR(pg_catalog.=) 'pg_catalog.anyenum'::pg_catalog.regtype
		                    and ft.typtype OPERATOR(pg_catalog.=) 'e' and pt.typtype OPERATOR(pg_catalog.=) 'e'
		                  then s.fk_type OPERATOR(pg_catalog.=) s.pk_type and s.fk_type OPERATOR(pg_catalog.=) ft.oid
		                  when ot.oid OPERATOR(pg_catalog.=) 'pg_catalog.anyarray'::pg_catalog.regtype
		                    and ft.typelem OPERATOR(pg_catalog.<>) 0::pg_catalog.oid and pt.typelem OPERATOR(pg_catalog.<>) 0::pg_catalog.oid
		                    and ft.typsubscript OPERATOR(pg_catalog.=) 6179::pg_catalog.oid and pt.typsubscript OPERATOR(pg_catalog.=) 6179::pg_catalog.oid
		                  then ft.oid OPERATOR(pg_catalog.=) pt.oid
		                  when ft.typtype OPERATOR(pg_catalog.=) any(array['b','e']::pg_catalog."char"[])
		                    and pt.typtype OPERATOR(pg_catalog.=) any(array['b','e']::pg_catalog."char"[])
		                    and ot.typtype OPERATOR(pg_catalog.=) 'b'
		                    and ft.typelem OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		                    and pt.typelem OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		                    and ot.typelem OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		                  then (pt.oid OPERATOR(pg_catalog.=) ot.oid or pk_cast.oid is not null)
		                   and (ft.oid OPERATOR(pg_catalog.=) ot.oid or fk_cast.oid is not null)
		                  end
		        end as allowed
		    from slots_with_types s
		    left join pg_catalog.pg_opclass opc on opc.oid OPERATOR(pg_catalog.=) s.opclass
		    left join types fb on fb.original OPERATOR(pg_catalog.=) s.fk_type and fb.base OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		    left join types pb on pb.original OPERATOR(pg_catalog.=) s.pk_type and pb.base OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		    left join pg_catalog.pg_type ft on ft.oid OPERATOR(pg_catalog.=) fb.oid
		    left join pg_catalog.pg_type pt on pt.oid OPERATOR(pg_catalog.=) pb.oid
		    left join pg_catalog.pg_type ot on ot.oid OPERATOR(pg_catalog.=) opc.opcintype
		    left join pg_catalog.pg_amop ppeq on ppeq.amopfamily OPERATOR(pg_catalog.=) opc.opcfamily
		     and ppeq.amopstrategy OPERATOR(pg_catalog.=) 3 and ppeq.amoplefttype OPERATOR(pg_catalog.=) ot.oid and ppeq.amoprighttype OPERATOR(pg_catalog.=) ot.oid
		    left join pg_catalog.pg_amop pfeq on pfeq.amopfamily OPERATOR(pg_catalog.=) opc.opcfamily
		     and pfeq.amopstrategy OPERATOR(pg_catalog.=) 3 and pfeq.amoplefttype OPERATOR(pg_catalog.=) ot.oid and pfeq.amoprighttype OPERATOR(pg_catalog.=) ft.oid
		    left join pg_catalog.pg_amop ffeq on ffeq.amopfamily OPERATOR(pg_catalog.=) opc.opcfamily
		     and ffeq.amopstrategy OPERATOR(pg_catalog.=) 3 and ffeq.amoplefttype OPERATOR(pg_catalog.=) ft.oid and ffeq.amoprighttype OPERATOR(pg_catalog.=) ft.oid
		    left join pg_catalog.pg_cast pk_cast on pk_cast.castsource OPERATOR(pg_catalog.=) pt.oid
		     and pk_cast.casttarget OPERATOR(pg_catalog.=) ot.oid and pk_cast.castcontext OPERATOR(pg_catalog.=) 'i'
		    left join pg_catalog.pg_cast fk_cast on fk_cast.castsource OPERATOR(pg_catalog.=) ft.oid
		     and fk_cast.casttarget OPERATOR(pg_catalog.=) ot.oid and fk_cast.castcontext OPERATOR(pg_catalog.=) 'i'
		)
		select case when pg_catalog.count(*) OPERATOR(pg_catalog.=) pg_catalog.jsonb_array_length($5::pg_catalog.jsonb)
		             and pg_catalog.count(allowed) OPERATOR(pg_catalog.=) pg_catalog.count(*)
		             and pg_catalog.count(*) OPERATOR(pg_catalog.>) 0
		            then coalesce(pg_catalog.min(position) filter (where not allowed), 0) end
		from comparison
    `, target.Source.Schema, target.Source.Name, target.Reference.Schema, target.Reference.Name, string(names), string(ref)).Scan(&position)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL foreign key equality: %w", err)
	}
	if !position.Valid {
		return nil, nil
	}
	fact := spec.PGForeignKeyEquality{Target: *target, IncompatiblePosition: int(position.Int64)}
	fact.Target.Columns = slices.Clone(target.Columns)
	fact.Target.ReferencedColumns = slices.Clone(target.ReferencedColumns)
	return &spec.ObjectSnapshot{PGForeignKeyEquality: &fact}, nil
}
