package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveForeignTableChange(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	q := req.Qualifiers
	state := spec.PGForeignTableChangeState{
		Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}},
		Action: q["action"], Column: q["column"], Destination: q["destination"],
		Owner: spec.PGTableOwnerTarget{Kind: q["role_kind"], Name: q["role_name"]},
	}
	schema, name := req.Schema, state.Destination
	if state.Action == "set_schema" {
		schema, name = state.Destination, req.Name
	}
	var options []byte
	err := p.db.QueryRowContext(ctx, `
		select coalesce(c.relkind::pg_catalog.text, ''), att.attnum is not null, coalesce(att.attnum OPERATOR(pg_catalog.<) 0, false),
		       dn.oid is not null, dst.oid is not null or (dt.oid is not null and
		           ($8::pg_catalog.bool or not coalesce(dt.typisdefined and elem.typarray OPERATOR(pg_catalog.=) dt.oid, false))),
		       coalesce(n.oid OPERATOR(pg_catalog.=) 99 or dn.oid OPERATOR(pg_catalog.=) 99
		           or dn.nspname::pg_catalog.text OPERATOR(pg_catalog.~) '^pg_(toast_)?temp_', false),
		       r.oid is not null, coalesce(w.fdwvalidator OPERATOR(pg_catalog.<>) 0::pg_catalog.oid, false),
		       exists (select 1 from pg_catalog.pg_attribute a where a.attrelid OPERATOR(pg_catalog.=) c.oid
		           and a.attname OPERATOR(pg_catalog.=) $9::pg_catalog.text),
		       pg_catalog.to_jsonb(array(select o.option_name from pg_catalog.pg_options_to_table(ft.ftoptions) o order by o.option_name))
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_attribute att on att.attrelid OPERATOR(pg_catalog.=) c.oid and att.attname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		    and ($10::pg_catalog.bool or not att.attisdropped)
		left join pg_catalog.pg_foreign_table ft on ft.ftrelid OPERATOR(pg_catalog.=) c.oid
		left join pg_catalog.pg_foreign_server srv on srv.oid OPERATOR(pg_catalog.=) ft.ftserver
		left join pg_catalog.pg_foreign_data_wrapper w on w.oid OPERATOR(pg_catalog.=) srv.srvfdw
		left join pg_catalog.pg_namespace dn on dn.nspname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		left join pg_catalog.pg_class dst on dst.relnamespace OPERATOR(pg_catalog.=) dn.oid and dst.relname OPERATOR(pg_catalog.=) $5::pg_catalog.text
		left join pg_catalog.pg_type dt on dt.typnamespace OPERATOR(pg_catalog.=) dn.oid and dt.typname OPERATOR(pg_catalog.=) $5::pg_catalog.text
		left join pg_catalog.pg_type elem on elem.oid OPERATOR(pg_catalog.=) dt.typelem and dt.typsubscript OPERATOR(pg_catalog.=) 6179::pg_catalog.oid
		left join pg_catalog.pg_roles r on r.rolname OPERATOR(pg_catalog.=) case
		    when $6::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $7::pg_catalog.text
		    when $6::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		    when $6::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text end
	`, req.Schema, req.Name, state.Column, schema, name, state.Owner.Kind, state.Owner.Name, state.Action == "set_schema", state.Destination, state.Action == "add_column").Scan(
		&state.RelationKind, &state.ColumnExists, &state.SystemColumn, &state.DestinationSchema, &state.DestinationOccupied,
		&state.DestinationForbidden, &state.OwnerExists, &state.HasValidator, &state.DestinationColumnExists, &options)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL foreign table premises: %w", err)
	}
	if err := json.Unmarshal(options, &state.Options); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL foreign table option names: %w", err)
	}
	return &spec.ObjectSnapshot{PGForeignTableChange: &state}, nil
}
