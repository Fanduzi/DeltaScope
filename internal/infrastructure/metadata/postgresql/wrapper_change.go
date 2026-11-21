package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveWrapperChange(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	q := req.Qualifiers
	state := spec.PGWrapperChangeState{Name: req.Name, RenameTo: q["rename_to"], Owner: spec.PGTableOwnerTarget{Kind: q["role_kind"], Name: q["role_name"]}}
	type function struct {
		Position int    `json:"position"`
		Kind     string `json:"kind"`
		Schema   string `json:"schema_name"`
		Name     string `json:"routine_name"`
	}
	functions := make([]function, len(req.PGWrapperFunctions))
	for i, fn := range req.PGWrapperFunctions {
		functions[i] = function{Position: i, Kind: fn.Kind}
		target := spec.PGDestructiveTableTarget{Table: fn.Target}
		if !fn.Clear && target.Qualified() && fn.Target.Schema != "pg_toast_temp" {
			functions[i].Schema, functions[i].Name = fn.Target.Schema, fn.Target.Name
		}
	}
	encoded, err := json.Marshal(functions)
	if err != nil {
		return nil, fmt.Errorf("encode PostgreSQL wrapper routine identities: %w", err)
	}
	var options, routineFacts []byte
	err = p.db.QueryRowContext(ctx, `
		select src.oid is not null, dst.oid is not null, r.oid is not null, coalesce(r.rolsuper, false),
		       coalesce(src.fdwvalidator OPERATOR(pg_catalog.<>) 0::pg_catalog.oid, false),
		       pg_catalog.to_jsonb(array(select o.option_name from pg_catalog.pg_options_to_table(src.fdwoptions) o order by o.option_name)),
		       (select coalesce(pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('Exists', f.oid is not null, 'Handler',
		           coalesce(rt.typname OPERATOR(pg_catalog.=) 'fdw_handler' and rtn.nspname OPERATOR(pg_catalog.=) 'pg_catalog', false)) order by fn.position), '[]'::pg_catalog.jsonb)
		        from pg_catalog.jsonb_to_recordset($5::pg_catalog.jsonb) as fn(position pg_catalog.int4, kind pg_catalog.text, schema_name pg_catalog.text, routine_name pg_catalog.text)
		        left join pg_catalog.pg_namespace ns on ns.nspname OPERATOR(pg_catalog.=) fn.schema_name
		        left join pg_catalog.pg_proc f on f.pronamespace OPERATOR(pg_catalog.=) ns.oid and f.proname OPERATOR(pg_catalog.=) fn.routine_name
		           and f.prokind OPERATOR(pg_catalog.<>) 'p'
		           and f.proargtypes OPERATOR(pg_catalog.=) case when fn.kind OPERATOR(pg_catalog.=) 'handler' then ''::pg_catalog.oidvector else '1009 26'::pg_catalog.oidvector end
		        left join pg_catalog.pg_type rt on rt.oid OPERATOR(pg_catalog.=) f.prorettype
		        left join pg_catalog.pg_namespace rtn on rtn.oid OPERATOR(pg_catalog.=) rt.typnamespace)
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_foreign_data_wrapper src on src.fdwname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_foreign_data_wrapper dst on dst.fdwname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_roles r on r.rolname OPERATOR(pg_catalog.=) case
		    when $3::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $4::pg_catalog.text
		    when $3::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		    when $3::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text end
	`, req.Name, state.RenameTo, state.Owner.Kind, state.Owner.Name, string(encoded)).Scan(
		&state.Exists, &state.DestinationOccupied, &state.OwnerExists, &state.OwnerSuperuser, &state.HasValidator, &options, &routineFacts)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL foreign-data wrapper premises: %w", err)
	}
	if err := json.Unmarshal(options, &state.Options); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL wrapper option names: %w", err)
	}
	if err := json.Unmarshal(routineFacts, &state.Functions); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL wrapper routine facts: %w", err)
	}
	if len(state.Functions) != len(req.PGWrapperFunctions) {
		return nil, nil
	}
	for i, fn := range req.PGWrapperFunctions {
		state.Functions[i].Kind, state.Functions[i].Target = fn.Kind, fn.Target
	}
	return &spec.ObjectSnapshot{PGWrapperChange: &state}, nil
}
