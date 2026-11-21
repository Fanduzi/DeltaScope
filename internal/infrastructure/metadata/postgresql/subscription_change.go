package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveSubscriptionChange(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	q := req.Qualifiers
	state := spec.PGSubscriptionChangeState{Name: req.Name, RenameTo: q["rename_to"],
		Owner: spec.PGTableOwnerTarget{Kind: q["role_kind"], Name: q["role_name"]}}
	var publications []byte
	err := p.db.QueryRowContext(ctx, `
		select s.oid is not null, coalesce(s.subenabled,false), s.subslotname is not null,
		       coalesce(s.subtwophasestate OPERATOR(pg_catalog.=) 'e',false), taken.oid is not null, r.oid is not null,
		       coalesce(pg_catalog.to_jsonb(s.subpublications), '[]'::pg_catalog.jsonb)
		from pg_catalog.pg_database d
		left join pg_catalog.pg_subscription s on s.subdbid OPERATOR(pg_catalog.=) d.oid
		  and s.subname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_subscription taken on taken.subdbid OPERATOR(pg_catalog.=) d.oid
		  and taken.subname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_roles r on r.rolname OPERATOR(pg_catalog.=) case
		    when $3::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $4::pg_catalog.text
		    when $3::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		    when $3::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text end
		where d.datname OPERATOR(pg_catalog.=) pg_catalog.current_database()
	`, req.Name, state.RenameTo, state.Owner.Kind, state.Owner.Name).Scan(&state.Exists, &state.Enabled, &state.SlotPresent,
		&state.TwoPhaseEnabled, &state.DestinationOccupied, &state.OwnerExists, &publications)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL subscription facts: %w", err)
	}
	if err := json.Unmarshal(publications, &state.Publications); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL subscription publication list: %w", err)
	}
	return &spec.ObjectSnapshot{PGSubscriptionChange: &state}, nil
}
