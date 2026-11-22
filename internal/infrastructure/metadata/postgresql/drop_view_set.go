package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveDropViewSet(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	type namedTarget struct {
		Schema string `json:"schema_name"`
		Name   string `json:"relation_name"`
		Order  int    `json:"position"`
	}
	targets := req.PGDestructiveTargets
	if len(targets) < 2 {
		return nil, nil
	}
	names := make([]namedTarget, len(targets))
	for i, target := range targets {
		if !target.Qualified() {
			return nil, nil
		}
		names[i] = namedTarget{Schema: target.Table.Schema, Name: target.Table.Name, Order: i + 1}
	}
	encoded, err := json.Marshal(names)
	if err != nil {
		return nil, fmt.Errorf("encode PostgreSQL DROP VIEW target list: %w", err)
	}
	state := spec.PGDropViewSetState{Targets: slices.Clone(targets), IfExists: req.PGDropIfExists, Cascade: req.Qualifiers["cascade"] == "true"}
	err = p.db.QueryRowContext(ctx, `
		with bound as (
		    select a.position, c.oid, c.relkind, c.reltype
		    from pg_catalog.jsonb_to_recordset($1::pg_catalog.jsonb)
		        as a(schema_name pg_catalog.text, relation_name pg_catalog.text, position pg_catalog.int4)
		    left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) a.schema_name
		    left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
		     and c.relname OPERATOR(pg_catalog.=) a.relation_name
		), refusals as (
		    select 1 as phase, position, case when oid is null then 'drop_view_absent' else 'drop_view_wrong_kind' end as reason
		    from bound where (oid is null and not $2::pg_catalog.bool) or relkind OPERATOR(pg_catalog.<>) 'v'
		    union all
		    select 2 as phase, b.position, 'drop_view_has_direct_dependency'
		    from bound b
		    where not $3::pg_catalog.bool and b.relkind OPERATOR(pg_catalog.=) 'v' and exists (
		        select 1 from pg_catalog.pg_depend d
		        join pg_catalog.pg_rewrite r on r.oid OPERATOR(pg_catalog.=) d.objid
		        join pg_catalog.pg_class v on v.oid OPERATOR(pg_catalog.=) r.ev_class
		        where d.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		          and d.refobjid OPERATOR(pg_catalog.=) b.oid
		          and d.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_rewrite'::pg_catalog.regclass
		          and d.objsubid OPERATOR(pg_catalog.=) 0 and d.deptype OPERATOR(pg_catalog.=) 'n'
		          and v.relkind OPERATOR(pg_catalog.=) any(array['v','m']::pg_catalog."char"[])
		          and not exists (select 1 from bound member where member.oid OPERATOR(pg_catalog.=) v.oid)
		    )
		    union all
		    select 3 as phase, b.position, 'drop_view_has_row_type_dependency'
		    from bound b
		    where not $3::pg_catalog.bool and b.relkind OPERATOR(pg_catalog.=) 'v' and exists (
		        select 1 from pg_catalog.pg_type row_type
		        join pg_catalog.pg_depend d on d.refobjid OPERATOR(pg_catalog.=) any(array[row_type.oid, row_type.typarray])
		        join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) d.objid
		          and a.attnum OPERATOR(pg_catalog.=) d.objsubid and a.attnum OPERATOR(pg_catalog.>) 0 and not a.attisdropped
		        where row_type.oid OPERATOR(pg_catalog.=) b.reltype
		          and d.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_type'::pg_catalog.regclass
		          and d.refobjsubid OPERATOR(pg_catalog.=) 0
		          and d.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass and d.deptype OPERATOR(pg_catalog.=) 'n'
		          and not exists (select 1 from bound member where member.oid OPERATOR(pg_catalog.=) d.objid)
		    )
		)
		select coalesce(f.position, 0), coalesce(f.reason, '')
		from (values (1)) as witness(dummy)
		left join lateral (select position, reason from refusals order by phase, position limit 1) f on true
	`, string(encoded), state.IfExists, state.Cascade).Scan(&state.Position, &state.Reason)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL DROP VIEW target list: %w", err)
	}
	return &spec.ObjectSnapshot{PGDropViewSet: &state}, nil
}
