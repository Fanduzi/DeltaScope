package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveCreateStorage(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	if !target.Qualified() {
		return nil, nil
	}
	state := spec.PGCreateStorageState{Target: target, Method: req.Qualifiers["method"], Tablespace: req.Qualifiers["tablespace"]}
	err := p.db.QueryRowContext(ctx, `
		select ns.oid is not null, c.oid is not null,
		       coalesce(am.amtype::pg_catalog.text, ''), ts.oid is not null,
		       coalesce(ts.spcname OPERATOR(pg_catalog.=) 'pg_global', false)
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace ns on ns.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) ns.oid
		 and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_am am on am.amname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		left join pg_catalog.pg_tablespace ts on ts.spcname OPERATOR(pg_catalog.=) $4::pg_catalog.text
	`, req.Schema, req.Name, state.Method, state.Tablespace).
		Scan(&state.SchemaExists, &state.TargetExists, &state.MethodKind, &state.TablespaceExists, &state.Global)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL CREATE TABLE storage targets: %w", err)
	}
	return &spec.ObjectSnapshot{PGCreateStorage: &state}, nil
}
