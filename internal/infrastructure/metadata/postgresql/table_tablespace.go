package postgresqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveTableTablespace(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	table := req.Qualifiers["table"]
	if req.Schema == "" || req.Schema == "pg_temp" || table == "" || req.Name == "" {
		return nil, nil
	}
	// Ordinary storage is rechecked in the same observation as the destination.
	// A zero relfilenode names mapped relations on this PostgreSQL series.
	row := p.db.QueryRowContext(ctx, `
		select dst.oid is not null,
		       coalesce(dst.oid OPERATOR(pg_catalog.=) 1664::pg_catalog.oid, false),
		       dst.oid OPERATOR(pg_catalog.=) cur.oid
		from pg_catalog.pg_class c
		join pg_catalog.pg_namespace n on n.oid OPERATOR(pg_catalog.=) c.relnamespace
		left join pg_catalog.pg_database db on db.datname OPERATOR(pg_catalog.=) pg_catalog.current_database()
		left join pg_catalog.pg_tablespace cur on cur.oid OPERATOR(pg_catalog.=)
		  case when c.reltablespace OPERATOR(pg_catalog.=) 0::pg_catalog.oid then db.dattablespace else c.reltablespace end
		left join pg_catalog.pg_tablespace dst on dst.spcname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		  and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		  and c.relkind OPERATOR(pg_catalog.=) coalesce(nullif($4::pg_catalog.text, ''), 'r')::pg_catalog."char"
		  and c.relpersistence OPERATOR(pg_catalog.=) 'p' and not c.relisshared
		  and c.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		  and (c.relkind OPERATOR(pg_catalog.=) 'p'
		       or (c.relkind OPERATOR(pg_catalog.=) 'r' and not c.relispartition
		           and c.relfilenode OPERATOR(pg_catalog.<>) 0::pg_catalog.oid
		           and not exists (select 1 from pg_catalog.pg_inherits h
		                           where h.inhrelid OPERATOR(pg_catalog.=) c.oid or h.inhparent OPERATOR(pg_catalog.=) c.oid)))
	`, req.Schema, table, req.Name, req.Qualifiers["relation_kind"])
	facts := &spec.PGTableTablespace{Schema: req.Schema, Table: table, Name: req.Name}
	var same sql.NullBool
	if err := row.Scan(&facts.Exists, &facts.Global, &same); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("read table tablespace: %w", err)
	}
	facts.Same, facts.SameKnown = same.Bool, same.Valid
	return &spec.ObjectSnapshot{PGTableTablespace: facts}, nil
}
