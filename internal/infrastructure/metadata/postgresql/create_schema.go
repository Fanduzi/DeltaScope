package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveCreateSchema(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	owner := spec.PGTableOwnerTarget{Kind: req.Qualifiers["role_kind"], Name: req.Qualifiers["role_name"]}
	if owner != (spec.PGTableOwnerTarget{}) && !owner.Valid() || req.Name == "" && !owner.Valid() {
		return nil, nil
	}
	state := spec.PGCreateSchemaState{Target: spec.PGCreateSchemaTarget{Name: req.Name, Owner: owner}}
	err := p.db.QueryRowContext(ctx, `
		with selected as (
		    select role.oid, case when $1::pg_catalog.text OPERATOR(pg_catalog.<>) '' then $1::pg_catalog.text
		                          else role.rolname::pg_catalog.text end as name
		    from (values (1)) as witness(dummy)
		    left join pg_catalog.pg_roles role on role.rolname OPERATOR(pg_catalog.=) case
		        when $2::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $3::pg_catalog.text
		        when $2::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		        when $2::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_role','current_user']::pg_catalog.text[]) then current_user::pg_catalog.text
		    end
		)
		select s.oid is not null, s.name is not null, n.oid is not null,
		       coalesce(pg_catalog.left(s.name,3) OPERATOR(pg_catalog.=) 'pg_', false),
		       coalesce(pg_catalog.current_setting('allow_system_table_mods', true)::pg_catalog.bool, false),
		       pg_catalog.current_setting('allow_system_table_mods', true) is not null
		from selected s
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) s.name
	`, req.Name, owner.Kind, owner.Name).Scan(&state.OwnerExists, &state.NameKnown, &state.Exists, &state.Reserved, &state.SystemSchemaAllowed, &state.SystemSchemaSettingKnown)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL schema creation premises: %w", err)
	}
	return &spec.ObjectSnapshot{PGCreateSchema: &state}, nil
}
