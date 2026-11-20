package postgresqlmeta

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveForeignKeyKeys(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Schema == "" || req.Schema == "pg_temp" || req.Name == "" {
		return nil, nil
	}
	rows, err := p.db.QueryContext(ctx, `
		select i.indexrelid is not null, i.indisprimary, i.indimmediate, i.indnkeyatts,
		       (select pg_catalog.jsonb_agg(a.attname::pg_catalog.text order by k.slot)
		        from pg_catalog.unnest(i.indkey) with ordinality as k(attnum, slot)
		        left join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) tbl.oid
		          and a.attnum OPERATOR(pg_catalog.=) k.attnum and not a.attisdropped
		        where k.slot OPERATOR(pg_catalog.<=) i.indnkeyatts)
		from pg_catalog.pg_namespace n
		join pg_catalog.pg_class tbl on tbl.relnamespace OPERATOR(pg_catalog.=) n.oid
		left join pg_catalog.pg_index i on i.indrelid OPERATOR(pg_catalog.=) tbl.oid
		 and i.indislive and i.indisvalid and i.indisunique
		 and i.indpred is null and i.indexprs is null
		where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		  and tbl.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		  and tbl.relkind OPERATOR(pg_catalog.=) 'r' and tbl.relpersistence OPERATOR(pg_catalog.=) 'p'
		  and tbl.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid and not tbl.relispartition
		  and not exists (select 1 from pg_catalog.pg_inherits h
		                  where h.inhrelid OPERATOR(pg_catalog.=) tbl.oid or h.inhparent OPERATOR(pg_catalog.=) tbl.oid)
		order by i.indexrelid
	`, req.Schema, req.Name)
	if err != nil {
		return nil, fmt.Errorf("query foreign key reference keys: %w", err)
	}
	defer rows.Close()
	facts := &spec.PGForeignKeyKeys{Schema: req.Schema, Table: req.Name}
	seenTable := false
	for rows.Next() {
		seenTable = true
		var hasIndex bool
		var primary, immediate sql.NullBool
		var width sql.NullInt32
		var encoded []byte
		if err := rows.Scan(&hasIndex, &primary, &immediate, &width, &encoded); err != nil {
			return nil, fmt.Errorf("scan foreign key reference key: %w", err)
		}
		if !hasIndex {
			continue
		}
		if !primary.Valid || !immediate.Valid || !width.Valid || width.Int32 <= 0 || len(encoded) == 0 {
			return nil, nil
		}
		var columns []string
		if err := json.Unmarshal(encoded, &columns); err != nil {
			return nil, fmt.Errorf("decode foreign key reference key: %w", err)
		}
		if len(columns) != int(width.Int32) {
			return nil, nil
		}
		for _, name := range columns {
			if name == "" {
				return nil, nil
			}
		}
		facts.Keys = append(facts.Keys, spec.PGForeignKeyKey{Columns: columns, Primary: primary.Bool, Immediate: immediate.Bool})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read foreign key reference keys: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close foreign key reference keys: %w", err)
	}
	if !seenTable {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGForeignKeyKeys: facts}, nil
}
