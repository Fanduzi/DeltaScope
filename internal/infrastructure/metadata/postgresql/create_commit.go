package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveCreateDestination(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	if !target.Qualified() {
		return nil, nil
	}
	state := spec.PGCreateDestinationState{Target: target}
	err := p.db.QueryRowContext(ctx, `
		select ns.oid is not null, c.oid is not null
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace ns on ns.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) ns.oid
		 and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
	`, req.Schema, req.Name).Scan(&state.SchemaExists, &state.RelationExists)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL CREATE TABLE persistence destination: %w", err)
	}
	return &spec.ObjectSnapshot{PGCreateDestination: &state}, nil
}
