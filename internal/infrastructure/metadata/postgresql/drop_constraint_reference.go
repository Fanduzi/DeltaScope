package postgresqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveDropConstraintReference(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	table := req.Qualifiers["table"]
	if req.Schema == "" || req.Schema == "pg_temp" || table == "" || req.Name == "" {
		return nil, nil
	}
	var blocked bool
	err := p.db.QueryRowContext(ctx, `
		select exists (
		    select 1
		    from pg_catalog.pg_depend usage
		    join pg_catalog.pg_constraint fk on fk.oid OPERATOR(pg_catalog.=) usage.objid
		    where usage.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_constraint'::pg_catalog.regclass
		      and usage.objsubid OPERATOR(pg_catalog.=) 0
		      and usage.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		      and usage.refobjid OPERATOR(pg_catalog.=) idx.indexrelid
		      and usage.refobjsubid OPERATOR(pg_catalog.=) 0
		      and usage.deptype OPERATOR(pg_catalog.=) 'n'
		      and fk.contype OPERATOR(pg_catalog.=) 'f'
		      and fk.confrelid OPERATOR(pg_catalog.=) tbl.oid
		      and fk.conindid OPERATOR(pg_catalog.=) idx.indexrelid
		)
		from pg_catalog.pg_namespace n
		join pg_catalog.pg_class tbl on tbl.relnamespace OPERATOR(pg_catalog.=) n.oid
		join pg_catalog.pg_constraint c on c.conrelid OPERATOR(pg_catalog.=) tbl.oid
		join pg_catalog.pg_index idx on idx.indexrelid OPERATOR(pg_catalog.=) c.conindid
		 and idx.indrelid OPERATOR(pg_catalog.=) tbl.oid
		where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		  and tbl.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		  and c.conname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		  and c.contype::pg_catalog.text OPERATOR(pg_catalog.=) $4::pg_catalog.text
		  and tbl.relkind OPERATOR(pg_catalog.=) 'r' and tbl.relpersistence OPERATOR(pg_catalog.=) 'p'
		  and tbl.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid and not tbl.relispartition
		  and not exists (select 1 from pg_catalog.pg_inherits h
		                  where h.inhrelid OPERATOR(pg_catalog.=) tbl.oid or h.inhparent OPERATOR(pg_catalog.=) tbl.oid)
		  and exists (select 1 from pg_catalog.pg_depend owner
		              where owner.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		                and owner.objid OPERATOR(pg_catalog.=) idx.indexrelid
		                and owner.objsubid OPERATOR(pg_catalog.=) 0
		                and owner.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_constraint'::pg_catalog.regclass
		                and owner.refobjid OPERATOR(pg_catalog.=) c.oid
		                and owner.refobjsubid OPERATOR(pg_catalog.=) 0
		                and owner.deptype OPERATOR(pg_catalog.=) 'i')
	`, req.Schema, table, req.Name, req.Qualifiers["kind"]).Scan(&blocked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read constraint drop reference: %w", err)
	}
	return &spec.ObjectSnapshot{PGDropConstraintReference: &spec.PGDropConstraintReference{
		Schema: req.Schema, Table: table, Name: req.Name, Kind: req.Qualifiers["kind"], Blocked: blocked,
	}}, nil
}
