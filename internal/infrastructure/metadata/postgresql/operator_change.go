package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveOperatorChange(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	q := req.Qualifiers
	target := spec.PGOperatorChange{
		Source: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}},
		Action: q["action"], Destination: q["destination"], Owner: spec.PGTableOwnerTarget{Kind: q["role_kind"], Name: q["role_name"]},
		Left:     spec.PGColumnTypeTarget{Schema: q["left_schema"], Name: q["left_type"], Array: q["left_array"] == "true"},
		Right:    spec.PGColumnTypeTarget{Schema: q["right_schema"], Name: q["right_type"], Array: q["right_array"] == "true"},
		Restrict: spec.Table{Schema: q["restrict_schema"], Name: q["restrict_name"]},
		Join:     spec.Table{Schema: q["join_schema"], Name: q["join_name"]},
		Unary:    q["unary"] == "true", Signature: true,
	}
	if !target.Source.Qualified() || !target.Right.Qualified() || !target.Unary && !target.Left.Qualified() {
		return nil, nil
	}
	state := spec.PGOperatorChangeState{Target: target}
	// PostgreSQL 17 looks up exact internal/oid/internal estimator signatures and requires float8 results.
	err := p.db.QueryRowContext(ctx, `
		with operands as (
		    select $3::pg_catalog.bool or ln.oid is not null as left_schema,
		           rn.oid is not null as right_schema,
		           case when $3::pg_catalog.bool then 0::pg_catalog.oid
		                when $6::pg_catalog.bool then nullif(lt.typarray, 0::pg_catalog.oid) else lt.oid end as left_oid,
		           case when $9::pg_catalog.bool then nullif(rt.typarray, 0::pg_catalog.oid) else rt.oid end as right_oid
		    from (values (1)) as witness(dummy)
		    left join pg_catalog.pg_namespace ln on ln.nspname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		    left join pg_catalog.pg_type lt on lt.typnamespace OPERATOR(pg_catalog.=) ln.oid and lt.typname OPERATOR(pg_catalog.=) $5::pg_catalog.text
		    left join pg_catalog.pg_namespace rn on rn.nspname OPERATOR(pg_catalog.=) $7::pg_catalog.text
		    left join pg_catalog.pg_type rt on rt.typnamespace OPERATOR(pg_catalog.=) rn.oid and rt.typname OPERATOR(pg_catalog.=) $8::pg_catalog.text
		)
		select a.left_schema and a.right_schema, a.left_oid is not null and a.right_oid is not null,
		       sn.oid is not null, src.oid is not null, r.oid is not null, dn.oid is not null,
		       coalesce(sn.oid OPERATOR(pg_catalog.=) 99 or dn.oid OPERATOR(pg_catalog.=) 99
		           or dn.nspname::pg_catalog.text OPERATOR(pg_catalog.~) '^pg_(toast_)?temp_', false),
		       dst.oid is not null, coalesce(src.oprresult OPERATOR(pg_catalog.=) 16::pg_catalog.oid, false),
		       restriction.matches, restriction.returns_float, join_estimator.matches, join_estimator.returns_float
		from operands a
		left join pg_catalog.pg_namespace sn on sn.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_operator src on src.oprnamespace OPERATOR(pg_catalog.=) sn.oid
		    and src.oprname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		    and src.oprleft OPERATOR(pg_catalog.=) a.left_oid and src.oprright OPERATOR(pg_catalog.=) a.right_oid
		left join pg_catalog.pg_namespace dn on dn.nspname OPERATOR(pg_catalog.=) $10::pg_catalog.text
		left join pg_catalog.pg_operator dst on dst.oprnamespace OPERATOR(pg_catalog.=) dn.oid
		    and dst.oprname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		    and dst.oprleft OPERATOR(pg_catalog.=) a.left_oid and dst.oprright OPERATOR(pg_catalog.=) a.right_oid
		left join pg_catalog.pg_roles r on r.rolname OPERATOR(pg_catalog.=) case
		    when $11::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $12::pg_catalog.text
		    when $11::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		    when $11::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text end
		left join lateral (
		    select count(*) as matches, coalesce(pg_catalog.bool_and(p.prorettype OPERATOR(pg_catalog.=) 701::pg_catalog.oid), false) as returns_float
		    from pg_catalog.pg_proc p join pg_catalog.pg_namespace n on n.oid OPERATOR(pg_catalog.=) p.pronamespace
		    where n.nspname OPERATOR(pg_catalog.=) $13::pg_catalog.text and p.proname OPERATOR(pg_catalog.=) $14::pg_catalog.text
		      and p.prokind OPERATOR(pg_catalog.<>) 'p'
		      and p.proargtypes OPERATOR(pg_catalog.=) '2281 26 2281 23'::pg_catalog.oidvector
		) restriction on true
		left join lateral (
		    select count(*) as matches, coalesce(pg_catalog.bool_and(p.prorettype OPERATOR(pg_catalog.=) 701::pg_catalog.oid), false) as returns_float
		    from pg_catalog.pg_proc p join pg_catalog.pg_namespace n on n.oid OPERATOR(pg_catalog.=) p.pronamespace
		    where n.nspname OPERATOR(pg_catalog.=) $15::pg_catalog.text and p.proname OPERATOR(pg_catalog.=) $16::pg_catalog.text
		      and p.prokind OPERATOR(pg_catalog.<>) 'p'
		      and (p.proargtypes OPERATOR(pg_catalog.=) '2281 26 2281 21'::pg_catalog.oidvector
		        or p.proargtypes OPERATOR(pg_catalog.=) '2281 26 2281 21 2281'::pg_catalog.oidvector)
		) join_estimator on true
	`, req.Schema, req.Name, target.Unary, target.Left.Schema, target.Left.Name, target.Left.Array,
		target.Right.Schema, target.Right.Name, target.Right.Array, target.Destination, target.Owner.Kind, target.Owner.Name,
		target.Restrict.Schema, target.Restrict.Name, target.Join.Schema, target.Join.Name).Scan(
		&state.OperandSchema, &state.OperandTypes, &state.SourceSchema, &state.SourceExists, &state.OwnerExists,
		&state.DestinationSchema, &state.NamespaceIneligible, &state.DestinationOccupied, &state.BooleanResult,
		&state.RestrictMatches, &state.RestrictReturnsFloat, &state.JoinMatches, &state.JoinReturnsFloat)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL operator change premises: %w", err)
	}
	return &spec.ObjectSnapshot{PGOperatorChange: &state}, nil
}
