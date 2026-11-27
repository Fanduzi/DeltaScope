package postgresqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveTableAccessMethod(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	table := req.Qualifiers["table"]
	var useDefault bool
	switch req.Qualifiers["is_default"] {
	case "true":
		useDefault = true
		if req.Name != "default" {
			return nil, nil
		}
	case "false":
	default:
		return nil, nil
	}
	if req.Schema == "" || req.Schema == "pg_temp" || table == "" || req.Name == "" {
		return nil, nil
	}
	row := p.db.QueryRowContext(ctx, `
		select coalesce(dst.amtype::pg_catalog.text, ''),
		       dst.oid OPERATOR(pg_catalog.=) cur.oid
		from pg_catalog.pg_class c
		join pg_catalog.pg_namespace n on n.oid OPERATOR(pg_catalog.=) c.relnamespace
		left join pg_catalog.pg_am cur on cur.oid OPERATOR(pg_catalog.=) c.relam
		cross join lateral (select case when $4::pg_catalog.bool
		  then pg_catalog.current_setting('default_table_access_method', true)
		  else $3::pg_catalog.text end as method_name) selected
		left join pg_catalog.pg_am dst on dst.amname OPERATOR(pg_catalog.=) selected.method_name
		where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		  and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		  and c.relkind OPERATOR(pg_catalog.=) coalesce(nullif($5::pg_catalog.text, ''), 'r')::pg_catalog."char"
		  and c.relpersistence OPERATOR(pg_catalog.=) 'p' and not c.relisshared
		  and c.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		  and ((c.relkind OPERATOR(pg_catalog.=) 'p' and not $4::pg_catalog.bool)
		       or (c.relkind OPERATOR(pg_catalog.=) 'r' and not c.relispartition
		           and c.relfilenode OPERATOR(pg_catalog.<>) 0::pg_catalog.oid
		           and not exists (select 1 from pg_catalog.pg_inherits h
		                           where h.inhrelid OPERATOR(pg_catalog.=) c.oid or h.inhparent OPERATOR(pg_catalog.=) c.oid)))
		  and selected.method_name is not null
		  and selected.method_name OPERATOR(pg_catalog.<>) ''
	`, req.Schema, table, req.Name, useDefault, req.Qualifiers["relation_kind"])
	facts := &spec.PGTableAccessMethod{Schema: req.Schema, Table: table, Name: req.Name, Default: useDefault}
	var same sql.NullBool
	if err := row.Scan(&facts.Kind, &same); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("read table access method: %w", err)
	}
	facts.Same, facts.SameKnown = same.Bool, same.Valid
	return &spec.ObjectSnapshot{PGTableAccessMethod: facts}, nil
}
