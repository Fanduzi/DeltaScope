package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveTextSearchChange(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	q := req.Qualifiers
	state := spec.PGTextSearchChangeState{
		Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}},
		Kind:   q["kind"], Action: q["action"], Destination: q["destination"],
		Owner:        spec.PGTableOwnerTarget{Kind: q["role_kind"], Name: q["role_name"]},
		Dictionaries: slices.Clone(req.PGTextSearchDictionaries),
	}
	schema, name := req.Schema, state.Destination
	if state.Action == "set_schema" {
		schema, name = state.Destination, req.Name
	}
	type dictionary struct {
		Position int    `json:"position"`
		Schema   string `json:"schema_name"`
		Name     string `json:"dict_name"`
	}
	dictionaries := make([]dictionary, len(state.Dictionaries))
	for i, dict := range state.Dictionaries {
		dictionaries[i].Position = i
		if (spec.PGDestructiveTableTarget{Table: dict}).Qualified() && dict.Schema != "pg_toast_temp" {
			dictionaries[i].Schema, dictionaries[i].Name = dict.Schema, dict.Name
		}
	}
	input, err := json.Marshal(dictionaries)
	if err != nil {
		return nil, fmt.Errorf("encode PostgreSQL dictionary identities: %w", err)
	}
	var mappings, resolved []byte
	err = p.db.QueryRowContext(ctx, `
		select n.oid is not null, cfg.oid is not null or d.oid is not null, dn.oid is not null,
		       dstcfg.oid is not null or dstd.oid is not null,
		       coalesce(n.oid OPERATOR(pg_catalog.=) 99 or dn.oid OPERATOR(pg_catalog.=) 99
		           or dn.nspname::pg_catalog.text OPERATOR(pg_catalog.~) '^pg_(toast_)?temp_', false),
		       r.oid is not null, coalesce(cfg.cfgparser OPERATOR(pg_catalog.=) builtin.oid, false),
		       coalesce(d.dicttemplate OPERATOR(pg_catalog.=) simple.oid, false),
		       case when cfg.cfgparser OPERATOR(pg_catalog.=) builtin.oid and $9::pg_catalog.bool then
		          (select pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('Token', tok.alias, 'Mapped', exists (
		               select 1 from pg_catalog.pg_ts_config_map m where m.mapcfg OPERATOR(pg_catalog.=) cfg.oid
		                 and m.maptokentype OPERATOR(pg_catalog.=) tok.tokid)))
		           from pg_catalog.ts_token_type(builtin.oid) tok) end,
		       coalesce((select pg_catalog.jsonb_agg(lookup.oid is not null order by wanted.position)
		          from pg_catalog.jsonb_to_recordset($8::pg_catalog.jsonb) as wanted(position pg_catalog.int4, schema_name pg_catalog.text, dict_name pg_catalog.text)
		          left join pg_catalog.pg_namespace dns on dns.nspname OPERATOR(pg_catalog.=) wanted.schema_name
		          left join pg_catalog.pg_ts_dict lookup on lookup.dictnamespace OPERATOR(pg_catalog.=) dns.oid and lookup.dictname OPERATOR(pg_catalog.=) wanted.dict_name), '[]'::pg_catalog.jsonb)
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_namespace dn on dn.nspname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		left join pg_catalog.pg_ts_config cfg on $5::pg_catalog.bool and cfg.cfgnamespace OPERATOR(pg_catalog.=) n.oid and cfg.cfgname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_ts_config dstcfg on $5::pg_catalog.bool and dstcfg.cfgnamespace OPERATOR(pg_catalog.=) dn.oid and dstcfg.cfgname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		left join pg_catalog.pg_ts_dict d on not $5::pg_catalog.bool and d.dictnamespace OPERATOR(pg_catalog.=) n.oid and d.dictname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_ts_dict dstd on not $5::pg_catalog.bool and dstd.dictnamespace OPERATOR(pg_catalog.=) dn.oid and dstd.dictname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		left join pg_catalog.pg_ts_parser builtin on builtin.prsnamespace OPERATOR(pg_catalog.=) 'pg_catalog'::pg_catalog.regnamespace
		   and builtin.prsname OPERATOR(pg_catalog.=) 'default' and builtin.prslextype OPERATOR(pg_catalog.=) 3721::pg_catalog.oid
		left join pg_catalog.pg_ts_template simple on simple.tmplnamespace OPERATOR(pg_catalog.=) 'pg_catalog'::pg_catalog.regnamespace
		   and simple.tmplname OPERATOR(pg_catalog.=) 'simple' and simple.tmplinit OPERATOR(pg_catalog.=) 3725::pg_catalog.oid
		left join pg_catalog.pg_roles r on r.rolname OPERATOR(pg_catalog.=) case
		    when $6::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $7::pg_catalog.text
		    when $6::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		    when $6::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text end
	`, req.Schema, req.Name, schema, name, state.Kind == "configuration", state.Owner.Kind, state.Owner.Name, string(input),
		state.Action == "add_mapping" || state.Action == "alter_mapping" || state.Action == "drop_mapping" || state.Action == "replace_mapping").Scan(
		&state.SourceSchema, &state.SourceExists, &state.DestinationSchema, &state.DestinationOccupied, &state.DestinationForbidden,
		&state.OwnerExists, &state.BuiltinParser, &state.SimpleTemplate, &mappings, &resolved)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL text search facts: %w", err)
	}
	if mappings != nil {
		if err := json.Unmarshal(mappings, &state.Mappings); err != nil {
			return nil, fmt.Errorf("decode PostgreSQL text search token facts: %w", err)
		}
	}
	if err := json.Unmarshal(resolved, &state.DictionaryExists); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL text search dictionary facts: %w", err)
	}
	return &spec.ObjectSnapshot{PGTextSearchChange: &state}, nil
}
