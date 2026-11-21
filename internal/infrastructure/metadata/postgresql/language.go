package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveLanguageDeclaration(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	state := spec.PGLanguageState{Name: req.Name}
	input, err := json.Marshal(req.PGLanguageRoutines)
	if err != nil {
		return nil, fmt.Errorf("encode PostgreSQL language callback identities: %w", err)
	}
	var routines []byte
	err = p.db.QueryRowContext(ctx, `
		select pg_catalog.current_database(), exists (select 1 from pg_catalog.pg_language lan where lan.lanname OPERATOR(pg_catalog.=) $1::pg_catalog.text),
		coalesce((select pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object(
		    'Exists', proc.oid is not null, 'HandlerReturn', coalesce(proc.prorettype OPERATOR(pg_catalog.=) 'pg_catalog.language_handler'::pg_catalog.regtype, false)) order by wanted.position)
		  from rows from (pg_catalog.jsonb_to_recordset($2::pg_catalog.jsonb) as ("Kind" pg_catalog.text, "Catalog" pg_catalog.text,
		    "Schema" pg_catalog.text, "Name" pg_catalog.text)) with ordinality as wanted(kind, catalog_name, schema_name, routine_name, position)
		  left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) wanted.schema_name
		  left join pg_catalog.pg_proc proc on proc.pronamespace OPERATOR(pg_catalog.=) n.oid and proc.proname OPERATOR(pg_catalog.=) wanted.routine_name
		    and proc.prokind OPERATOR(pg_catalog.<>) 'p'
		    and proc.proargtypes OPERATOR(pg_catalog.=) case wanted.kind
		      when 'handler' then ''::pg_catalog.oidvector
		      when 'inline' then '2281'::pg_catalog.oidvector
		      when 'validator' then '26'::pg_catalog.oidvector end
		), '[]'::pg_catalog.jsonb)
	`, req.Name, string(input)).Scan(&state.Database, &state.Exists, &routines)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL language prerequisites: %w", err)
	}
	if err := json.Unmarshal(routines, &state.Routines); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL language prerequisites: %w", err)
	}
	if len(state.Routines) != len(req.PGLanguageRoutines) {
		return nil, fmt.Errorf("unexpected PostgreSQL language callback count")
	}
	for i := range state.Routines {
		state.Routines[i].Target = req.PGLanguageRoutines[i]
	}
	return &spec.ObjectSnapshot{PGLanguage: &state}, nil
}
