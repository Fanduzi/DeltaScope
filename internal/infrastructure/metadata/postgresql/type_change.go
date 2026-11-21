package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveTypeChange(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGTypeChange{Source: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}},
		Action: req.Qualifiers["action"], Destination: req.Qualifiers["destination"],
		Owner: spec.PGTableOwnerTarget{Kind: req.Qualifiers["role_kind"], Name: req.Qualifiers["role_name"]}}
	if !target.Source.Qualified() {
		return nil, nil
	}
	move := target.Action == "set_schema"
	schema, name := req.Schema, target.Destination
	if move {
		schema, name = target.Destination, req.Name
	}
	state := spec.PGTypeChangeState{Target: target}
	err := p.db.QueryRowContext(ctx, `
		select t.oid is not null, coalesce(t.typisdefined, false),
		       coalesce(t.typsubscript OPERATOR(pg_catalog.=) 6179::pg_catalog.oid and t.typelem OPERATOR(pg_catalog.<>) 0::pg_catalog.oid, false),
		       coalesce(t.typtype OPERATOR(pg_catalog.=) 'c' and c.relkind OPERATOR(pg_catalog.<>) 'c', false),
		       coalesce(t.typtype OPERATOR(pg_catalog.=) 'c' and c.relkind OPERATOR(pg_catalog.=) 'c', false),
		       dn.oid is not null,
		       coalesce(sn.oid OPERATOR(pg_catalog.=) 99 or dn.oid OPERATOR(pg_catalog.=) 99
		           or dn.nspname::pg_catalog.text OPERATOR(pg_catalog.~) '^pg_(toast_)?temp_', false),
		       dt.oid is not null and ($5::pg_catalog.bool or not coalesce(dt.typisdefined and de.typarray OPERATOR(pg_catalog.=) dt.oid, false)),
		       dc.oid is not null,
		       exists (select 1 from pg_catalog.pg_type a join pg_catalog.pg_type clash on clash.typname OPERATOR(pg_catalog.=) a.typname
		           and clash.typnamespace OPERATOR(pg_catalog.=) dn.oid where a.oid OPERATOR(pg_catalog.=) t.typarray),
		       owner_role.oid is not null
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace sn on sn.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_type t on t.typnamespace OPERATOR(pg_catalog.=) sn.oid and t.typname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_class c on c.oid OPERATOR(pg_catalog.=) t.typrelid
		left join pg_catalog.pg_namespace dn on dn.nspname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		left join pg_catalog.pg_type dt on dt.typnamespace OPERATOR(pg_catalog.=) dn.oid and dt.typname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		left join pg_catalog.pg_type de on de.oid OPERATOR(pg_catalog.=) dt.typelem and dt.typsubscript OPERATOR(pg_catalog.=) 6179::pg_catalog.oid
		left join pg_catalog.pg_class dc on dc.relnamespace OPERATOR(pg_catalog.=) dn.oid and dc.relname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		left join pg_catalog.pg_roles owner_role on owner_role.rolname OPERATOR(pg_catalog.=) case
		    when $6::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $7::pg_catalog.text
		    when $6::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		    when $6::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text end
	`, req.Schema, req.Name, schema, name, move, target.Owner.Kind, target.Owner.Name).Scan(&state.SourceExists, &state.Defined, &state.Array, &state.RowType, &state.Composite,
		&state.DestinationSchema, &state.NamespaceIneligible, &state.DestinationType, &state.DestinationRelation, &state.CompanionOccupied, &state.OwnerExists)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL type change facts: %w", err)
	}
	return &spec.ObjectSnapshot{PGTypeChange: &state}, nil
}
