package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveCollationChange(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGCollationChangeTarget{
		Source: spec.PGColumnCollation{Schema: req.Schema, Name: req.Name},
		Action: req.Qualifiers["action"], Destination: req.Qualifiers["destination"],
		Owner: spec.PGTableOwnerTarget{Kind: req.Qualifiers["role_kind"], Name: req.Qualifiers["role_name"]},
	}
	if !target.Source.Qualified() {
		return nil, nil
	}
	state := spec.PGCollationChangeState{Target: target}
	err := p.db.QueryRowContext(ctx, `
		select col.oid is null or col.collencoding OPERATOR(pg_catalog.=) enc.encoding
		       or col.collprovider OPERATOR(pg_catalog.<>) 'i'
		       or enc.encoding OPERATOR(pg_catalog.=) pg_catalog.pg_char_to_encoding('UTF8'),
		       col.oid is not null, dn.oid is not null, coalesce(sn.oid OPERATOR(pg_catalog.=) dn.oid, false),
		       coalesce(sn.oid OPERATOR(pg_catalog.=) 99 or dn.oid OPERATOR(pg_catalog.=) 99
		           or dn.nspname::pg_catalog.text OPERATOR(pg_catalog.~) '^pg_(toast_)?temp_', false),
		       exists (select 1 from pg_catalog.pg_collation c where c.collnamespace OPERATOR(pg_catalog.=) dn.oid
		           and c.collname OPERATOR(pg_catalog.=) case when $3::pg_catalog.text OPERATOR(pg_catalog.=) 'rename'
		              then $4::pg_catalog.text else $2::pg_catalog.text end
		           and c.collencoding OPERATOR(pg_catalog.=) any(array[enc.encoding,-1])),
		       role.oid is not null
		from (select pg_catalog.pg_char_to_encoding(pg_catalog.current_setting('server_encoding')) as encoding) enc
		left join pg_catalog.pg_namespace sn on sn.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join lateral (
		    select c.oid, c.collencoding, c.collprovider from pg_catalog.pg_collation c
		    where c.collnamespace OPERATOR(pg_catalog.=) sn.oid and c.collname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		      and c.collencoding OPERATOR(pg_catalog.=) any(array[enc.encoding,-1])
		    order by c.collencoding desc limit 1
		) col on true
		left join pg_catalog.pg_namespace dn on dn.nspname OPERATOR(pg_catalog.=) case
		    when $3::pg_catalog.text OPERATOR(pg_catalog.=) 'set_schema' then $4::pg_catalog.text else $1::pg_catalog.text end
		left join pg_catalog.pg_roles role on role.rolname OPERATOR(pg_catalog.=) case
		    when $5::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $6::pg_catalog.text
		    when $5::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		    when $5::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text end
	`, req.Schema, req.Name, target.Action, target.Destination, target.Owner.Kind, target.Owner.Name).Scan(
		&state.SourceKnown, &state.SourceExists, &state.DestinationExists, &state.SameNamespace,
		&state.NamespaceUnsupported, &state.DestinationOccupied, &state.OwnerExists)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL collation change premises: %w", err)
	}
	return &spec.ObjectSnapshot{PGCollationChange: &state}, nil
}
