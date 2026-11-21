package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveOperatorFamilyChange(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	t := spec.PGOperatorFamilyChange{
		Source: spec.PGOperatorFamilyTarget{Schema: req.Schema, Name: req.Name, Method: req.Qualifiers["method"]},
		Action: req.Qualifiers["action"], Destination: req.Qualifiers["destination"],
		Owner: spec.PGTableOwnerTarget{Kind: req.Qualifiers["role_kind"], Name: req.Qualifiers["role_name"]},
	}
	if t.Source.Schema == "" || t.Source.Name == "" || t.Source.Method == "" {
		return nil, nil
	}
	state := spec.PGOperatorFamilyChangeState{Target: t}
	err := p.db.QueryRowContext(ctx, `
		select coalesce(am.amtype::pg_catalog.text, ''), sn.oid is not null, src.oid is not null,
		       dn.oid is not null, occupied.oid is not null, coalesce(sn.oid OPERATOR(pg_catalog.=) dn.oid, false),
		       coalesce(sn.oid OPERATOR(pg_catalog.=) 99 or dn.oid OPERATOR(pg_catalog.=) 99
		           or dn.nspname::pg_catalog.text OPERATOR(pg_catalog.~) '^pg_(toast_)?temp_', false),
		       role.oid is not null
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_am am on am.amname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		left join pg_catalog.pg_namespace sn on sn.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_opfamily src on src.opfnamespace OPERATOR(pg_catalog.=) sn.oid
		 and src.opfname OPERATOR(pg_catalog.=) $2::pg_catalog.text and src.opfmethod OPERATOR(pg_catalog.=) am.oid
		left join pg_catalog.pg_namespace dn on dn.nspname OPERATOR(pg_catalog.=) case
		    when $4::pg_catalog.text OPERATOR(pg_catalog.=) 'set_schema' then $5::pg_catalog.text else $1::pg_catalog.text end
		left join pg_catalog.pg_opfamily occupied on occupied.opfnamespace OPERATOR(pg_catalog.=) dn.oid
		 and occupied.opfmethod OPERATOR(pg_catalog.=) am.oid and occupied.opfname OPERATOR(pg_catalog.=) case
		    when $4::pg_catalog.text OPERATOR(pg_catalog.=) 'rename' then $5::pg_catalog.text else $2::pg_catalog.text end
		left join pg_catalog.pg_roles role on role.rolname OPERATOR(pg_catalog.=) case
		    when $6::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $7::pg_catalog.text
		    when $6::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		    when $6::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text end
	`, req.Schema, req.Name, t.Source.Method, t.Action, t.Destination, t.Owner.Kind, t.Owner.Name).Scan(
		&state.MethodKind, &state.SourceSchemaExists, &state.SourceExists, &state.DestinationExists,
		&state.DestinationOccupied, &state.SameNamespace, &state.NamespaceUnsupported, &state.OwnerExists)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL operator family change premises: %w", err)
	}
	return &spec.ObjectSnapshot{PGOperatorFamilyChange: &state}, nil
}
