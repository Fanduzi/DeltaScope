package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveViewOwner(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Qualifiers["table"]}}
	owner := spec.PGTableOwnerTarget{Kind: req.Qualifiers["kind"], Name: req.Name}
	kind := req.Qualifiers["relation_kind"]
	if !target.Qualified() || !owner.Valid() || (kind != "v" && kind != "m") {
		return nil, nil
	}
	state := &spec.PGTableOwnerState{Schema: target.Table.Schema, Table: target.Table.Name, Target: owner}
	var kindObserved sql.NullString
	err := p.db.QueryRowContext(ctx, `
		select c.relkind::pg_catalog.text, role.oid is not null,
		       coalesce(role.oid OPERATOR(pg_catalog.=) c.relowner, false)
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
		 and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_roles role on c.relkind OPERATOR(pg_catalog.=) $5::pg_catalog."char"
		 and role.rolname OPERATOR(pg_catalog.=) case
		    when $4::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $3::pg_catalog.text
		    when $4::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text
		    when $4::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		 end
	`, target.Table.Schema, target.Table.Name, owner.Name, owner.Kind, kind).Scan(&kindObserved, &state.Exists, &state.Same)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL view owner: %w", err)
	}
	state.SourceKind, state.SourceKindKnown = kindObserved.String, true
	return &spec.ObjectSnapshot{PGTableOwner: state}, nil
}
