package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveBulkTablespace(ctx context.Context, target *spec.PGBulkTablespaceTarget) (*spec.ObjectSnapshot, error) {
	if target == nil || target.Source == "" || target.Destination == "" {
		return nil, nil
	}
	roles := make([]spec.PGTableOwnerTarget, 0, len(target.Roles))
	for _, role := range target.Roles {
		if !role.Valid() {
			return nil, nil
		}
		roles = append(roles, role)
	}
	params, err := json.Marshal(roles)
	if err != nil {
		return nil, fmt.Errorf("encode bulk tablespace role selections: %w", err)
	}
	// Both endpoints and the ordered role observations come from one snapshot.
	row := p.db.QueryRowContext(ctx, `
		select src.oid is not null, dst.oid is not null,
		       coalesce(src.oid OPERATOR(pg_catalog.=) 1664::pg_catalog.oid, false),
		       coalesce(dst.oid OPERATOR(pg_catalog.=) 1664::pg_catalog.oid, false),
		       coalesce(src.oid OPERATOR(pg_catalog.=) dst.oid, false),
		       (select coalesce(pg_catalog.jsonb_agg(r.oid is not null order by authored.ordinality), '[]'::pg_catalog.jsonb)
		        from pg_catalog.jsonb_array_elements($3::pg_catalog.jsonb) with ordinality as authored(role, ordinality)
		        left join pg_catalog.pg_roles r on r.rolname OPERATOR(pg_catalog.=)
		          case
		            when (authored.role OPERATOR(pg_catalog.->>) 'Kind') OPERATOR(pg_catalog.=) 'named' then authored.role OPERATOR(pg_catalog.->>) 'Name'
		            when (authored.role OPERATOR(pg_catalog.->>) 'Kind') OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text
		            when (authored.role OPERATOR(pg_catalog.->>) 'Kind') OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		          end)
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_tablespace src on src.spcname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_tablespace dst on dst.spcname OPERATOR(pg_catalog.=) $2::pg_catalog.text
	`, target.Source, target.Destination, string(params))
	facts := spec.PGBulkTablespaceState{Target: *target}
	facts.Target.Roles = slices.Clone(target.Roles)
	var roleResult []byte
	if err := row.Scan(&facts.SourceExists, &facts.DestinationExists, &facts.SourceGlobal, &facts.DestinationGlobal, &facts.Same, &roleResult); err != nil {
		return nil, fmt.Errorf("read bulk tablespace targets: %w", err)
	}
	if err := json.Unmarshal(roleResult, &facts.RoleExists); err != nil {
		return nil, fmt.Errorf("decode bulk tablespace role observations: %w", err)
	}
	if facts.RoleExists == nil || len(facts.RoleExists) != len(roles) {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGBulkTablespace: &facts}, nil
}
