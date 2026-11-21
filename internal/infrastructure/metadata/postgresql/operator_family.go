package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveOperatorFamily(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	state := spec.PGOperatorFamilyState{Target: spec.PGOperatorFamilyTarget{Schema: req.Schema, Name: req.Name, Method: req.Qualifiers["method"]}}
	if req.Schema == "" || req.Name == "" || state.Target.Method == "" {
		return nil, nil
	}
	err := p.db.QueryRowContext(ctx, `
		select n.oid is not null, coalesce(am.amtype::pg_catalog.text, ''), f.oid is not null
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_am am on am.amname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		left join pg_catalog.pg_opfamily f on f.opfnamespace OPERATOR(pg_catalog.=) n.oid
		 and f.opfname OPERATOR(pg_catalog.=) $2::pg_catalog.text and f.opfmethod OPERATOR(pg_catalog.=) am.oid
	`, req.Schema, req.Name, state.Target.Method).Scan(&state.SchemaExists, &state.MethodKind, &state.Exists)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL operator family target: %w", err)
	}
	return &spec.ObjectSnapshot{PGOperatorFamily: &state}, nil
}
