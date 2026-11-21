package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveIndexDestination(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	state := spec.PGIndexDestinationState{
		Source: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}},
		Action: req.Qualifiers["action"], Destination: req.Qualifiers["destination"],
	}
	if !state.Source.Qualified() || state.Destination == "" || state.Action != "rename" && state.Action != "set_tablespace" {
		return nil, nil
	}
	err := p.db.QueryRowContext(ctx, `
		select coalesce(src.relkind::pg_catalog.text, ''),
		       case when $3::pg_catalog.text OPERATOR(pg_catalog.=) 'rename' then rel.oid is not null else dst.oid is not null end,
		       coalesce(src.reltype OPERATOR(pg_catalog.<>) 0, false),
		       exists (select 1 from pg_catalog.pg_constraint con where con.conindid OPERATOR(pg_catalog.=) src.oid
		                  and con.contype OPERATOR(pg_catalog.=) any(array['p','u','x']::pg_catalog."char"[])),
		       coalesce(dst.oid OPERATOR(pg_catalog.=) case when src.reltablespace OPERATOR(pg_catalog.=) 0
		                       then db.dattablespace else src.reltablespace end, false),
		       coalesce(src.relkind OPERATOR(pg_catalog.=) 'i' and src.relfilenode OPERATOR(pg_catalog.=) 0, false),
		       coalesce(dst.oid OPERATOR(pg_catalog.=) 1664, false)
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace ns on ns.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class src on src.relnamespace OPERATOR(pg_catalog.=) ns.oid
		 and src.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_class rel on $3::pg_catalog.text OPERATOR(pg_catalog.=) 'rename'
		 and rel.relnamespace OPERATOR(pg_catalog.=) ns.oid and rel.relname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		left join pg_catalog.pg_tablespace dst on $3::pg_catalog.text OPERATOR(pg_catalog.=) 'set_tablespace'
		 and dst.spcname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		left join pg_catalog.pg_database db on db.datname OPERATOR(pg_catalog.=) pg_catalog.current_database()
	`, req.Schema, req.Name, state.Action, state.Destination).Scan(&state.Kind, &state.DestinationExists,
		&state.HasRowType, &state.ConstraintBacked, &state.SameTablespace, &state.Mapped, &state.Global)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL index destination: %w", err)
	}
	return &spec.ObjectSnapshot{PGIndexDestination: &state}, nil
}
