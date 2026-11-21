package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveDropView(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	if !target.Qualified() {
		return nil, nil
	}
	state := spec.PGDropViewState{Target: target}
	err := p.db.QueryRowContext(ctx, `
		select coalesce(c.relkind::pg_catalog.text, ''), exists (
		    select 1 from pg_catalog.pg_depend d
		    join pg_catalog.pg_rewrite r on r.oid OPERATOR(pg_catalog.=) d.objid
		    join pg_catalog.pg_class dependent on dependent.oid OPERATOR(pg_catalog.=) r.ev_class
		    where d.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		      and d.refobjid OPERATOR(pg_catalog.=) c.oid
		      and d.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_rewrite'::pg_catalog.regclass
		      and d.objsubid OPERATOR(pg_catalog.=) 0
		      and d.deptype OPERATOR(pg_catalog.=) 'n'
		      and dependent.oid OPERATOR(pg_catalog.<>) c.oid
		      and dependent.relkind OPERATOR(pg_catalog.=) any(array['v','m']::pg_catalog."char"[])
		)
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
		 and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
	`, req.Schema, req.Name).Scan(&state.Kind, &state.DependentView)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL DROP VIEW target: %w", err)
	}
	return &spec.ObjectSnapshot{PGDropViewState: &state}, nil
}
