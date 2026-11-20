package postgresqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveDropColumnDependency(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	table := req.Qualifiers["table"]
	if req.Schema == "" || req.Schema == "pg_temp" || table == "" || req.Name == "" {
		return nil, nil
	}
	facts := spec.PGDropColumnDependency{Schema: req.Schema, Table: table, Column: req.Name}
	err := p.db.QueryRowContext(ctx, `
		select case
		  when exists (
		    select 1 from pg_catalog.pg_depend d
		    join pg_catalog.pg_rewrite w on w.oid OPERATOR(pg_catalog.=) d.objid
		    join pg_catalog.pg_class v on v.oid OPERATOR(pg_catalog.=) w.ev_class
		    where d.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		      and d.refobjid OPERATOR(pg_catalog.=) c.oid
		      and d.refobjsubid OPERATOR(pg_catalog.=) a.attnum
		      and d.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_rewrite'::pg_catalog.regclass
		      and d.objsubid OPERATOR(pg_catalog.=) 0
		      and d.deptype OPERATOR(pg_catalog.=) 'n'
		      and v.relkind OPERATOR(pg_catalog.=) any (array['v', 'm']::pg_catalog."char"[])
		  ) then 'view'
		  when exists (
		    select 1 from pg_catalog.pg_depend d
		    join pg_catalog.pg_attrdef ad on ad.oid OPERATOR(pg_catalog.=) d.objid
		    join pg_catalog.pg_attribute g on g.attrelid OPERATOR(pg_catalog.=) ad.adrelid
		      and g.attnum OPERATOR(pg_catalog.=) ad.adnum
		    where d.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		      and d.refobjid OPERATOR(pg_catalog.=) c.oid
		      and d.refobjsubid OPERATOR(pg_catalog.=) a.attnum
		      and d.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_attrdef'::pg_catalog.regclass
		      and d.objsubid OPERATOR(pg_catalog.=) 0
		      and d.deptype OPERATOR(pg_catalog.=) 'n'
		      and g.attrelid OPERATOR(pg_catalog.=) c.oid
		      and g.attnum OPERATOR(pg_catalog.<>) a.attnum
		      and g.attgenerated OPERATOR(pg_catalog.=) 's'
		      and not g.attisdropped
		  ) then 'generated_column'
		  when $4::pg_catalog.bool and exists (
		    select 1 from pg_catalog.pg_depend d
		    where d.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		      and d.refobjid OPERATOR(pg_catalog.=) c.oid
		      and d.refobjsubid OPERATOR(pg_catalog.=) a.attnum
		      and d.classid OPERATOR(pg_catalog.=) any(array[
		          'pg_catalog.pg_proc'::pg_catalog.regclass,
		          'pg_catalog.pg_rewrite'::pg_catalog.regclass,
		          'pg_catalog.pg_trigger'::pg_catalog.regclass,
		          'pg_catalog.pg_policy'::pg_catalog.regclass,
		          'pg_catalog.pg_publication_rel'::pg_catalog.regclass
		      ])
		  ) then 'type_change'
		  when exists (
		    select 1 from pg_catalog.pg_depend d
		    join pg_catalog.pg_constraint fk on fk.oid OPERATOR(pg_catalog.=) d.objid
		    where d.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		      and d.refobjid OPERATOR(pg_catalog.=) c.oid
		      and d.refobjsubid OPERATOR(pg_catalog.=) a.attnum
		      and d.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_constraint'::pg_catalog.regclass
		      and d.objsubid OPERATOR(pg_catalog.=) 0
		      and d.deptype OPERATOR(pg_catalog.=) 'n'
		      and fk.contype OPERATOR(pg_catalog.=) 'f'
		      and fk.confrelid OPERATOR(pg_catalog.=) c.oid
		      and not (fk.conrelid OPERATOR(pg_catalog.=) c.oid and a.attnum OPERATOR(pg_catalog.=) any(fk.conkey))
		  ) then 'foreign_key'
		  else '' end
		from pg_catalog.pg_namespace n
		join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid
		join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) c.oid
		where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		  and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		  and a.attname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		  and a.attnum OPERATOR(pg_catalog.>) 0 and not a.attisdropped
		  and c.relkind OPERATOR(pg_catalog.=) 'r' and c.relpersistence OPERATOR(pg_catalog.=) 'p'
		  and c.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid and not c.relispartition
		  and not exists (select 1 from pg_catalog.pg_inherits h
		                  where h.inhrelid OPERATOR(pg_catalog.=) c.oid or h.inhparent OPERATOR(pg_catalog.=) c.oid)
	`, req.Schema, table, req.Name, req.Qualifiers["type_change"] == "true").Scan(&facts.Kind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read drop column dependency: %w", err)
	}
	return &spec.ObjectSnapshot{PGDropColumnDependency: &facts}, nil
}
