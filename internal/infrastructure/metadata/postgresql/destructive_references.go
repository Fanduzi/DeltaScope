package postgresqlmeta

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveDestructiveReferences(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	targets := req.PGDestructiveTargets
	type namedTarget struct {
		Schema   string `json:"schema_name"`
		Name     string `json:"relation_name"`
		Position int    `json:"position"`
		Only     bool   `json:"only_target"`
	}
	if len(targets) == 0 {
		return nil, nil
	}
	names := make([]namedTarget, 0, len(targets))
	for i, target := range targets {
		if !target.Qualified() {
			return nil, nil
		}
		names = append(names, namedTarget{Schema: target.Table.Schema, Name: target.Table.Name, Position: i + 1, Only: target.Only})
	}
	encoded, err := json.Marshal(names)
	if err != nil {
		return nil, fmt.Errorf("encode PostgreSQL destructive table targets: %w", err)
	}
	var position, onlyPosition sql.NullInt64
	query := `
		with recursive authored as (
		    select * from pg_catalog.jsonb_to_recordset($1::pg_catalog.jsonb)
		      as target(schema_name pg_catalog.text, relation_name pg_catalog.text, position pg_catalog.int4, only_target pg_catalog.bool)
		), bound as (
		    select a.position, a.only_target, c.oid, c.relkind
		    from authored a
		    left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) a.schema_name
		    left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
		     and c.relname OPERATOR(pg_catalog.=) a.relation_name
		     and c.relkind OPERATOR(pg_catalog.=) any(array['r','p']::pg_catalog."char"[])
		     and c.relpersistence OPERATOR(pg_catalog.=) 'p' and not c.relisshared
		), expanded(position, oid, recursive_target) as (
		    select distinct on (oid) position, oid, not only_target from bound where oid is not null order by oid, position
		), members(position, oid, recursive_target) as (
		    select position, oid, recursive_target from expanded
		    union
		    select m.position, h.inhrelid, true from members m
		    join pg_catalog.pg_inherits h on h.inhparent OPERATOR(pg_catalog.=) m.oid
		    where m.recursive_target and not h.inhdetachpending
		), reference_scope(position, oid, parent) as (
		    select position, oid, 0::pg_catalog.oid from members where not $2::pg_catalog.bool
		    union
		    select scope.position,
		           case when c.conparentid OPERATOR(pg_catalog.=) 0::pg_catalog.oid then c.confrelid else scope.oid end,
		           c.conparentid
		    from reference_scope scope
		    join pg_catalog.pg_constraint c on c.contype OPERATOR(pg_catalog.=) 'f' and
		      (c.oid OPERATOR(pg_catalog.=) scope.parent or
		       (scope.parent OPERATOR(pg_catalog.=) 0::pg_catalog.oid and c.confrelid OPERATOR(pg_catalog.=) scope.oid
		        and c.conparentid OPERATOR(pg_catalog.<>) 0::pg_catalog.oid))
		), blockers as (
		    select scope.position from reference_scope scope
		    join pg_catalog.pg_constraint fk on fk.contype OPERATOR(pg_catalog.=) 'f' and fk.confrelid OPERATOR(pg_catalog.=) scope.oid
		    where scope.parent OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		      and not exists (select 1 from members m where m.oid OPERATOR(pg_catalog.=) fk.conrelid)
		), unselected_only as (
		    select pg_catalog.min(b.position) as position from bound b
		    where b.only_target and b.relkind OPERATOR(pg_catalog.=) 'p'
		      and not exists (select 1 from members m where m.oid OPERATOR(pg_catalog.=) b.oid
		                       and m.position OPERATOR(pg_catalog.<) b.position)
		)
		select case when observable.ok then coalesce((select pg_catalog.min(position) from blockers), 0) end,
		       case when observable.ok then coalesce((select position from unselected_only), 0) end
		from (select (select pg_catalog.bool_and(oid is not null) from bound)
		             and not exists (select 1 from members m
		                 left join pg_catalog.pg_class c on c.oid OPERATOR(pg_catalog.=) m.oid
		                 where c.oid is null or c.relkind OPERATOR(pg_catalog.<>) all(array['r','p']::pg_catalog."char"[])
		                    or c.relpersistence OPERATOR(pg_catalog.<>) 'p' or c.relisshared) as ok) observable
	`
	args := []any{string(encoded), req.Qualifiers["cascade"] == "true"}
	if req.Type == "drop_references" {
		query = `
		with authored as (
		    select * from pg_catalog.jsonb_to_recordset($1::pg_catalog.jsonb)
		        as target(schema_name pg_catalog.text, relation_name pg_catalog.text, position pg_catalog.int4)
		), bound as (
		    select a.position, c.oid, c.reltype,
		           case when c.oid is null then $2::pg_catalog.bool
		                else c.relkind OPERATOR(pg_catalog.=) 'r' and c.relpersistence OPERATOR(pg_catalog.=) 'p'
		                     and not c.relispartition and not c.relisshared
		                     and not exists (select 1 from pg_catalog.pg_inherits h
		                                     where h.inhrelid OPERATOR(pg_catalog.=) c.oid or h.inhparent OPERATOR(pg_catalog.=) c.oid) end as eligible
		    from authored a
		    left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) a.schema_name
		    left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
		     and c.relname OPERATOR(pg_catalog.=) a.relation_name
		), known_blockers as (
		    select b.position
		    from bound b
		    where b.oid is not null and (
		        exists (select 1 from pg_catalog.pg_constraint fk
		                where fk.contype OPERATOR(pg_catalog.=) 'f'
		                  and fk.confrelid OPERATOR(pg_catalog.=) b.oid
		                  and not exists (select 1 from bound member where member.oid OPERATOR(pg_catalog.=) fk.conrelid))
		        or exists (select 1 from pg_catalog.pg_depend d
		                   join pg_catalog.pg_rewrite r on r.oid OPERATOR(pg_catalog.=) d.objid
		                   join pg_catalog.pg_class v on v.oid OPERATOR(pg_catalog.=) r.ev_class
		                   where d.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		                     and d.refobjid OPERATOR(pg_catalog.=) b.oid
		                     and d.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_rewrite'::pg_catalog.regclass
		                     and d.objsubid OPERATOR(pg_catalog.=) 0
		                     and d.deptype OPERATOR(pg_catalog.=) 'n'
		                     and v.relkind OPERATOR(pg_catalog.=) any(array['v','m']::pg_catalog."char"[]))
		        or exists (select 1 from pg_catalog.pg_type row_type
		                   join pg_catalog.pg_depend d on d.refobjid OPERATOR(pg_catalog.=) any(array[row_type.oid, row_type.typarray])
		                   join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) d.objid
		                     and a.attnum OPERATOR(pg_catalog.=) d.objsubid
		                     and a.attnum OPERATOR(pg_catalog.>) 0 and not a.attisdropped
		                   where row_type.oid OPERATOR(pg_catalog.=) b.reltype
		                     and d.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_type'::pg_catalog.regclass
		                     and d.refobjsubid OPERATOR(pg_catalog.=) 0
		                     and d.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		                     and d.deptype OPERATOR(pg_catalog.=) 'n'
		                     and not exists (select 1 from bound member where member.oid OPERATOR(pg_catalog.=) d.objid))
		    )
		)
		select case when (select pg_catalog.count(*) OPERATOR(pg_catalog.>) 0 and pg_catalog.bool_and(eligible) from bound)
		            then coalesce((select pg_catalog.min(position) from known_blockers), 0) end, 0::pg_catalog.int4
		`
		args[1] = req.PGDropIfExists
	}
	err = p.db.QueryRowContext(ctx, query, args...).Scan(&position, &onlyPosition)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL destructive table references: %w", err)
	}
	if !position.Valid || !onlyPosition.Valid {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGDestructiveReferences: &spec.PGDestructiveReferences{
		Targets: slices.Clone(targets), BlockedPosition: int(position.Int64), IneligibleOnlyPosition: int(onlyPosition.Int64),
	}}, nil
}
