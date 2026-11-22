package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveCheckIntegerCoercion(ctx context.Context, source string) (*spec.ObjectSnapshot, error) {
	typeName := map[string]string{"integer": "pg_catalog.int4", "bigint": "pg_catalog.int8", "numeric": "pg_catalog.numeric"}[source]
	if typeName == "" {
		return nil, nil
	}
	coercion := spec.PGCheckIntegerCoercion{Source: source}
	err := p.db.QueryRowContext(ctx, `
		select exists (
		    select 1 from pg_catalog.pg_cast c
		    where c.castsource OPERATOR(pg_catalog.=) $1::pg_catalog.regtype
		      and c.casttarget OPERATOR(pg_catalog.=) 'pg_catalog.bool'::pg_catalog.regtype
		      and c.castcontext OPERATOR(pg_catalog.=) any(array['i','a']::pg_catalog."char"[])
		)
	`, typeName).Scan(&coercion.Allowed)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL CHECK numeric literal coercion: %w", err)
	}
	return &spec.ObjectSnapshot{PGCheckIntegerCoercion: &coercion}, nil
}
