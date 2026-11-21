package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveViewDefault(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	column := req.Qualifiers["column"]
	if !target.Qualified() || column == "" {
		return nil, nil
	}
	state := &spec.PGViewDefaultState{Schema: req.Schema, View: req.Name, Column: column}
	err := p.db.QueryRowContext(ctx, `
		select coalesce(c.relkind::pg_catalog.text, ''), a.attnum is not null
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
		 and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) c.oid
		 and c.relkind OPERATOR(pg_catalog.=) 'v'
		 and a.attname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		 and a.attnum OPERATOR(pg_catalog.>) 0 and not a.attisdropped
	`, req.Schema, req.Name, column).Scan(&state.Kind, &state.ColumnExists)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL view default target: %w", err)
	}
	return &spec.ObjectSnapshot{PGViewDefault: state}, nil
}
