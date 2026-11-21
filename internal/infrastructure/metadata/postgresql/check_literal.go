package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveCheckIntegerCoercion(ctx context.Context) (*spec.ObjectSnapshot, error) {
	coercion := spec.PGCheckIntegerCoercion{}
	err := p.db.QueryRowContext(ctx, `
		select exists (
		    select 1 from pg_catalog.pg_cast c
		    where c.castsource OPERATOR(pg_catalog.=) 'pg_catalog.int4'::pg_catalog.regtype
		      and c.casttarget OPERATOR(pg_catalog.=) 'pg_catalog.bool'::pg_catalog.regtype
		      and c.castcontext OPERATOR(pg_catalog.=) any(array['i','a']::pg_catalog."char"[])
		)
	`).Scan(&coercion.Allowed)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL CHECK integer coercion: %w", err)
	}
	return &spec.ObjectSnapshot{PGCheckIntegerCoercion: &coercion}, nil
}
