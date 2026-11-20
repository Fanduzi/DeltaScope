package postgresqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveConstraintUsingIndex(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	table := req.Qualifiers["table"]
	if req.Schema == "" || req.Schema == "pg_temp" || table == "" || req.Name == "" {
		return nil, nil
	}
	row := p.db.QueryRowContext(ctx, `
		select coalesce(idx.relkind::pg_catalog.text, ''), i.indexrelid is not null,
		       coalesce(i.indrelid OPERATOR(pg_catalog.=) tbl.oid, false),
		       coalesce(i.indisvalid, false), coalesce(i.indisunique, false),
		       i.indexprs is not null, i.indpred is not null,
		       coalesce(idx.relam OPERATOR(pg_catalog.=) 403::pg_catalog.oid, false),
		       coalesce(i.indnullsnotdistinct, false),
		       exists (select 1 from pg_catalog.pg_depend dep
		               where dep.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		                 and dep.objid OPERATOR(pg_catalog.=) idx.oid and dep.objsubid OPERATOR(pg_catalog.=) 0
		                 and dep.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_constraint'::pg_catalog.regclass
		                 and dep.refobjsubid OPERATOR(pg_catalog.=) 0 and dep.deptype OPERATOR(pg_catalog.=) 'i'),
		       (select case when i.indnkeyatts OPERATOR(pg_catalog.>) 0
		                         and pg_catalog.count(*) OPERATOR(pg_catalog.=) i.indnkeyatts
		                         and pg_catalog.bool_and(a.attnum is not null and ia.attnum is not null
		                             and slots.collation_oid is not null and slots.flags is not null)
		                    then pg_catalog.bool_and(slots.flags OPERATOR(pg_catalog.=) 0
		                          and slots.collation_oid OPERATOR(pg_catalog.=) a.attcollation and ia.attoptions is null)
		               end
		        from ROWS FROM (pg_catalog.unnest(i.indkey), pg_catalog.unnest(i.indcollation),
		                        pg_catalog.unnest(i.indoption)) with ordinality as slots(attnum, collation_oid, flags, slot)
		        left join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) i.indrelid
		         and a.attnum OPERATOR(pg_catalog.=) slots.attnum and not a.attisdropped
		        left join pg_catalog.pg_attribute ia on ia.attrelid OPERATOR(pg_catalog.=) i.indexrelid
		         and ia.attnum OPERATOR(pg_catalog.=) slots.slot and not ia.attisdropped
		        where slots.slot OPERATOR(pg_catalog.<=) i.indnkeyatts)
		from pg_catalog.pg_namespace n
		join pg_catalog.pg_class tbl on tbl.relnamespace OPERATOR(pg_catalog.=) n.oid
		left join pg_catalog.pg_class idx on idx.relnamespace OPERATOR(pg_catalog.=) n.oid
		 and idx.relname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		left join pg_catalog.pg_index i on i.indexrelid OPERATOR(pg_catalog.=) idx.oid
		where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		  and tbl.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		  and tbl.relkind OPERATOR(pg_catalog.=) 'r' and tbl.relpersistence OPERATOR(pg_catalog.=) 'p'
		  and not tbl.relispartition and not tbl.relisshared
		  and tbl.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		  and not exists (select 1 from pg_catalog.pg_inherits h
		                  where h.inhrelid OPERATOR(pg_catalog.=) tbl.oid or h.inhparent OPERATOR(pg_catalog.=) tbl.oid)
	`, req.Schema, table, req.Name)
	facts := spec.PGConstraintUsingIndex{Schema: req.Schema, Table: table, Name: req.Name}
	var keys sql.NullBool
	err := row.Scan(&facts.Kind, &facts.IndexKnown, &facts.OnTable, &facts.Valid, &facts.Unique,
		&facts.Expression, &facts.Partial, &facts.BTree, &facts.NullsNotDistinct, &facts.ConstraintOwned, &keys)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read constraint index attachment: %w", err)
	}
	facts.KeyDefaults, facts.KeyDefaultsKnown = keys.Bool, keys.Valid
	return &spec.ObjectSnapshot{PGConstraintUsingIndex: &facts}, nil
}
