package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveColumnType(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGColumnTypeTarget{Schema: req.Schema, Name: req.Name, Array: req.Qualifiers["array"] == "true"}
	if !target.Qualified() {
		return nil, nil
	}
	// LookupTypeName follows typarray for bracket syntax. CheckAttributeType
	// also checks the element of a true array (handler OID 6179 on PG17).
	row := p.db.QueryRowContext(ctx, `
		select n.oid is not null, base.oid is not null, resolved.oid is not null,
		       coalesce(resolved.typisdefined, false),
		       coalesce(resolved.typtype OPERATOR(pg_catalog.=) 'p', false)
		       or coalesce(elem.typtype OPERATOR(pg_catalog.=) 'p', false)
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_type base on base.typnamespace OPERATOR(pg_catalog.=) n.oid
		 and base.typname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_type resolved on resolved.oid OPERATOR(pg_catalog.=)
		 case when $3::pg_catalog.bool then base.typarray else base.oid end
		left join pg_catalog.pg_type elem on elem.oid OPERATOR(pg_catalog.=) resolved.typelem
		 and resolved.typsubscript OPERATOR(pg_catalog.=) 6179::pg_catalog.oid
	`, target.Schema, target.Name, target.Array)
	facts := spec.PGColumnTypeState{Target: target}
	if err := row.Scan(&facts.SchemaExists, &facts.BaseExists, &facts.Resolved, &facts.Defined, &facts.Pseudo); err != nil {
		return nil, fmt.Errorf("read column type destination: %w", err)
	}
	return &spec.ObjectSnapshot{PGColumnType: &facts}, nil
}
