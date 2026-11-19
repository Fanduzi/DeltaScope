package postgresqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveTableOwner(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	table := req.Qualifiers["table"]
	target := spec.PGTableOwnerTarget{Kind: req.Qualifiers["kind"], Name: req.Name}
	if req.Schema == "" || req.Schema == "pg_temp" || table == "" {
		return nil, nil
	}
	switch target.Kind {
	case "named":
		if target.Name == "" {
			return nil, nil
		}
	case "current_user", "current_role", "session_user", "public":
		if target.Name != "" {
			return nil, nil
		}
	default:
		return nil, nil
	}
	// Resolve the current/session identity in the same observation as the
	// table owner. A PUBLIC request still needs the supported table scope.
	row := p.db.QueryRowContext(ctx, `
		select role.oid is not null, coalesce(role.oid OPERATOR(pg_catalog.=) c.relowner, false)
		from pg_catalog.pg_class c
		join pg_catalog.pg_namespace n on n.oid OPERATOR(pg_catalog.=) c.relnamespace
		left join pg_catalog.pg_roles role on role.rolname OPERATOR(pg_catalog.=)
		  case
		    when $4::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $3::pg_catalog.text
		    when $4::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text
		    when $4::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		  end
		where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		  and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		  and c.relkind OPERATOR(pg_catalog.=) 'r' and c.relpersistence OPERATOR(pg_catalog.=) 'p'
		  and not c.relisshared and not c.relispartition
		  and c.relfilenode OPERATOR(pg_catalog.<>) 0::pg_catalog.oid
		  and c.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		  and not exists (select 1 from pg_catalog.pg_inherits h
		                  where h.inhrelid OPERATOR(pg_catalog.=) c.oid or h.inhparent OPERATOR(pg_catalog.=) c.oid)
	`, req.Schema, table, target.Name, target.Kind)
	facts := &spec.PGTableOwnerState{Schema: req.Schema, Table: table, Target: target}
	if err := row.Scan(&facts.Exists, &facts.Same); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("read table owner target: %w", err)
	}
	return &spec.ObjectSnapshot{PGTableOwner: facts}, nil
}
