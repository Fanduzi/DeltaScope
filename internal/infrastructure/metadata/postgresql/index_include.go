package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveIndexInclude(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	if !target.Qualified() {
		return nil, nil
	}
	state := spec.PGIndexIncludeState{Target: target}
	var columns, system []byte
	err := p.db.QueryRowContext(ctx, `
		select coalesce(c.relkind::pg_catalog.text, ''),
		       pg_catalog.to_json(array(select a.attname::pg_catalog.text from pg_catalog.pg_attribute a
		           where a.attrelid OPERATOR(pg_catalog.=) c.oid and a.attnum OPERATOR(pg_catalog.>) 0 and not a.attisdropped)),
		       pg_catalog.to_json(array(select a.attname::pg_catalog.text from pg_catalog.pg_attribute a
		           where a.attrelid OPERATOR(pg_catalog.=) c.oid and a.attnum OPERATOR(pg_catalog.<) 0 and not a.attisdropped)),
		       case when am.oid OPERATOR(pg_catalog.=) 403 and am.amname OPERATOR(pg_catalog.=) 'btree' then 'btree'
		            when am.oid OPERATOR(pg_catalog.=) 405 and am.amname OPERATOR(pg_catalog.=) 'hash' then 'hash'
		            when am.oid OPERATOR(pg_catalog.=) 2742 and am.amname OPERATOR(pg_catalog.=) 'gin' then 'gin'
		            else '' end
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
		 and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_am am on am.amname OPERATOR(pg_catalog.=) $3::pg_catalog.text and am.amtype OPERATOR(pg_catalog.=) 'i'
	`, req.Schema, req.Name, req.Qualifiers["method"]).Scan(&state.Kind, &columns, &system, &state.Method)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL index INCLUDE target: %w", err)
	}
	if err := json.Unmarshal(columns, &state.Columns); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL user column directory: %w", err)
	}
	if err := json.Unmarshal(system, &state.System); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL system column directory: %w", err)
	}
	return &spec.ObjectSnapshot{PGIndexInclude: &state}, nil
}
