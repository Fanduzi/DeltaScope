package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveGrantDeclaration(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	d := req.PGGrant
	if d == nil {
		return nil, nil
	}
	state := spec.PGGrantState{ObjectKind: d.ObjectKind, AllInSchema: d.AllInSchema,
		Grantees: slices.Clone(d.Grantees), Grantor: d.Grantor}
	roles, err := json.Marshal(d.Grantees)
	if err != nil {
		return nil, fmt.Errorf("encode PostgreSQL grantee identities: %w", err)
	}
	targets := make([]spec.Table, len(d.Targets))
	for i, target := range d.Targets {
		targets[i] = target.Table
	}
	targetJSON, err := json.Marshal(targets)
	if err != nil {
		return nil, fmt.Errorf("encode PostgreSQL grant targets: %w", err)
	}
	var targetFacts, roleFacts []byte
	err = p.db.QueryRowContext(ctx, `
		with target(result) as (
		  select coalesce(pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object(
		    'Exists', case
		      when $3::pg_catalog.bool or $4::pg_catalog.text OPERATOR(pg_catalog.=) 'schema' then n.oid is not null
		      when $4::pg_catalog.text OPERATOR(pg_catalog.=) 'database' then db.oid is not null
		      when $4::pg_catalog.text OPERATOR(pg_catalog.=) 'tablespace' then ts.oid is not null
		      else c.oid is not null end,
		    'Kind', coalesce(c.relkind::pg_catalog.text,''),
		    'Columns', coalesce((select pg_catalog.jsonb_agg(a.attname order by a.attnum)
		      from pg_catalog.pg_attribute a where a.attrelid OPERATOR(pg_catalog.=) c.oid and not a.attisdropped),'[]'::pg_catalog.jsonb)
		  ) order by wanted.position),'[]'::pg_catalog.jsonb)
		  from rows from (pg_catalog.jsonb_to_recordset($1::pg_catalog.jsonb) as (schema pg_catalog.text,name pg_catalog.text))
		       with ordinality as wanted(schema_name,object_name,position)
		  left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) case when $3::pg_catalog.bool
		    or $4::pg_catalog.text OPERATOR(pg_catalog.=) 'schema' then wanted.object_name else wanted.schema_name end
		  left join pg_catalog.pg_class c on not $3::pg_catalog.bool and $4::pg_catalog.text OPERATOR(pg_catalog.=) any(array['table','sequence']::pg_catalog.text[])
		    and c.relnamespace OPERATOR(pg_catalog.=) n.oid and c.relname OPERATOR(pg_catalog.=) wanted.object_name
		  left join pg_catalog.pg_database db on $4::pg_catalog.text OPERATOR(pg_catalog.=) 'database' and db.datname OPERATOR(pg_catalog.=) wanted.object_name
		  left join pg_catalog.pg_tablespace ts on $4::pg_catalog.text OPERATOR(pg_catalog.=) 'tablespace' and ts.spcname OPERATOR(pg_catalog.=) wanted.object_name
		), grantee(result) as (
		  select coalesce(pg_catalog.jsonb_agg(wanted.kind OPERATOR(pg_catalog.=) 'public' or r.oid is not null order by wanted.position),'[]'::pg_catalog.jsonb)
		  from rows from (pg_catalog.jsonb_to_recordset($2::pg_catalog.jsonb) as ("Kind" pg_catalog.text,"Name" pg_catalog.text))
		       with ordinality as wanted(kind,role_name,position)
		  left join pg_catalog.pg_roles r on r.rolname OPERATOR(pg_catalog.=) case
		    when wanted.kind OPERATOR(pg_catalog.=) 'named' then wanted.role_name
		    when wanted.kind OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		    when wanted.kind OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text end
		)
		select pg_catalog.current_database(), target.result, grantee.result, grantor.oid is not null,
		       coalesce(grantor.rolname OPERATOR(pg_catalog.=) current_user,false)
		from target cross join grantee
		left join pg_catalog.pg_roles grantor on grantor.rolname OPERATOR(pg_catalog.=) case
		    when $5::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $6::pg_catalog.text
		    when $5::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		    when $5::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text end
	`, string(targetJSON), string(roles), d.AllInSchema, d.ObjectKind, d.Grantor.Kind, d.Grantor.Name).Scan(
		&state.Database, &targetFacts, &roleFacts, &state.GrantorExists, &state.GrantorCurrent)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL grant identities: %w", err)
	}
	if err := json.Unmarshal(targetFacts, &state.Targets); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL grant targets: %w", err)
	}
	if err := json.Unmarshal(roleFacts, &state.GranteeExists); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL grant role facts: %w", err)
	}
	if len(state.Targets) != len(d.Targets) || len(state.GranteeExists) != len(d.Grantees) {
		return nil, fmt.Errorf("unexpected PostgreSQL grant fact count")
	}
	for i := range state.Targets {
		state.Targets[i].Target = d.Targets[i]
	}
	return &spec.ObjectSnapshot{PGGrant: &state}, nil
}
