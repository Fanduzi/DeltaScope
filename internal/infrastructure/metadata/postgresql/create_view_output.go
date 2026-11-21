package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveCreateViewOutput(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	if !target.Qualified() {
		return nil, nil
	}
	state := spec.PGCreateViewTarget{Table: target.Table, Columns: []spec.PGViewOutputColumn{}}
	rows, err := p.db.QueryContext(ctx, `
		select n.oid is not null, coalesce(c.relkind::pg_catalog.text, ''),
		       a.attname, a.atttypid, a.atttypmod, a.attcollation
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
		 and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) c.oid
		 and c.relkind OPERATOR(pg_catalog.=) 'v' and a.attnum OPERATOR(pg_catalog.>) 0 and not a.attisdropped
		order by a.attnum
	`, req.Schema, req.Name)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL view output: %w", err)
	}
	defer rows.Close()
	observed := false
	for rows.Next() {
		var column sql.NullString
		var typeOID, typmod, collation sql.NullInt64
		if err := rows.Scan(&state.SchemaExists, &state.Kind, &column, &typeOID, &typmod, &collation); err != nil {
			return nil, fmt.Errorf("scan PostgreSQL view output: %w", err)
		}
		observed = true
		if column.Valid {
			if !typeOID.Valid || !typmod.Valid || !collation.Valid {
				return nil, fmt.Errorf("incomplete PostgreSQL view output column")
			}
			state.Columns = append(state.Columns, spec.PGViewOutputColumn{
				Name: column.String, TypeOID: typeOID.Int64, Typmod: int32(typmod.Int64), Collation: collation.Int64,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate PostgreSQL view output: %w", err)
	}
	if !observed {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGCreateViewTarget: &state}, nil
}
