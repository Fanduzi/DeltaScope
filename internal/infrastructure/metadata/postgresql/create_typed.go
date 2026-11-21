package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveCreateTyped(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	typeName := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Qualifiers["type_schema"], Name: req.Qualifiers["type_name"]}}
	if !target.Qualified() || !typeName.Qualified() {
		return nil, nil
	}
	state := spec.PGCreateTypedState{Target: target, Type: typeName}
	var columns []byte
	err := p.db.QueryRowContext(ctx, `
		select dst_ns.oid is not null, dst.oid is not null,
		       type_ns.oid is not null, t.oid is not null, coalesce(t.typisdefined, false),
		       coalesce(t.typtype OPERATOR(pg_catalog.=) 'c' and row_type.relkind OPERATOR(pg_catalog.=) 'c', false),
		       pg_catalog.to_jsonb(array(
		           select a.attname::pg_catalog.text from pg_catalog.pg_attribute a
		           where a.attrelid OPERATOR(pg_catalog.=) row_type.oid and a.attnum OPERATOR(pg_catalog.>) 0 and not a.attisdropped
		           order by a.attnum))
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace dst_ns on dst_ns.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class dst on dst.relnamespace OPERATOR(pg_catalog.=) dst_ns.oid
		 and dst.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_namespace type_ns on type_ns.nspname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		left join pg_catalog.pg_type t on t.typnamespace OPERATOR(pg_catalog.=) type_ns.oid
		 and t.typname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		left join pg_catalog.pg_class row_type on row_type.oid OPERATOR(pg_catalog.=) t.typrelid
	`, target.Table.Schema, target.Table.Name, typeName.Table.Schema, typeName.Table.Name).
		Scan(&state.TargetSchema, &state.TargetExists, &state.TypeSchema, &state.TypeExists, &state.Defined, &state.Composite, &columns)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL typed table source and destination: %w", err)
	}
	if err := json.Unmarshal(columns, &state.Columns); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL typed table columns: %w", err)
	}
	return &spec.ObjectSnapshot{PGCreateTyped: &state}, nil
}
