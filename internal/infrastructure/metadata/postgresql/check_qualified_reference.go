package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveCheckQualifiedReference(ctx context.Context, member string) (*spec.ObjectSnapshot, error) {
	state := spec.PGCheckQualifiedReference{Member: member}
	err := p.db.QueryRowContext(ctx, `
		select pg_catalog.current_database(),
		       exists(select 1 from pg_catalog.pg_proc p where p.proname OPERATOR(pg_catalog.=) $1::pg_catalog.text)
		       or exists(select 1 from pg_catalog.pg_type t where t.typname OPERATOR(pg_catalog.=) $1::pg_catalog.text)
	`, member).Scan(&state.Database, &state.FallbackPossible)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL CHECK qualified reference facts: %w", err)
	}
	return &spec.ObjectSnapshot{PGCheckQualifiedReference: &state}, nil
}
