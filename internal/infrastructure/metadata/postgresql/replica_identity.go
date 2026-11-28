package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveReplicaIdentityIndex(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	table := req.Qualifiers["table"]
	if req.Schema == "" || req.Schema == "pg_temp" || table == "" || req.Name == "" {
		return nil, nil
	}
	partitionRoot := req.Qualifiers["kind"] == "p"
	// The LEFT JOIN preserves a missing index within a proven target namespace.
	// A missing or out-of-scope table yields no observation at all. Only the
	// built-in btree (BTREE_AM_OID) vouches for can_unique: other methods may
	// override pg_indexam_has_property with their own property callback.
	rows, err := p.db.QueryContext(ctx, `
		select coalesce(idx.relkind::pg_catalog.text, ''), i.indexrelid is not null,
		       coalesce(i.indrelid OPERATOR(pg_catalog.=) tbl.oid, false),
		       case when i.indexrelid is not null and idx.relam OPERATOR(pg_catalog.=) 403::pg_catalog.oid
		            then pg_catalog.pg_indexam_has_property(idx.relam, 'can_unique') end,
		       coalesce(i.indisunique, false), coalesce(i.indimmediate, false),
		       i.indexprs is not null, i.indpred is not null,
		       (select case when i.indnkeyatts OPERATOR(pg_catalog.>) 0
		                    and pg_catalog.count(*) OPERATOR(pg_catalog.=) i.indnkeyatts
		                    and pg_catalog.bool_and(k.attnum OPERATOR(pg_catalog.<=) 0 or a.attnum is not null)
		                    then pg_catalog.bool_and(k.attnum OPERATOR(pg_catalog.>) 0 and a.attnotnull)
		               end
		        from pg_catalog.unnest(i.indkey) with ordinality as k(attnum, position)
		        left join pg_catalog.pg_attribute a
		          on a.attrelid OPERATOR(pg_catalog.=) tbl.oid
		         and a.attnum OPERATOR(pg_catalog.=) k.attnum and not a.attisdropped
		        where k.position OPERATOR(pg_catalog.<=) i.indnkeyatts)
		from pg_catalog.pg_namespace n
		join pg_catalog.pg_class tbl on tbl.relnamespace OPERATOR(pg_catalog.=) n.oid
		left join pg_catalog.pg_class idx on idx.relnamespace OPERATOR(pg_catalog.=) n.oid
		 and idx.relname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		left join pg_catalog.pg_index i on i.indexrelid OPERATOR(pg_catalog.=) idx.oid
		where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		  and tbl.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		  and tbl.relpersistence OPERATOR(pg_catalog.=) 'p' and not tbl.relispartition
		  and tbl.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		  and (($4::pg_catalog.bool and tbl.relkind OPERATOR(pg_catalog.=) 'p' and not tbl.relisshared)
		    or (not $4::pg_catalog.bool and tbl.relkind OPERATOR(pg_catalog.=) 'r'
		        and not exists (select 1 from pg_catalog.pg_inherits h
		                        where h.inhrelid OPERATOR(pg_catalog.=) tbl.oid
		                           or h.inhparent OPERATOR(pg_catalog.=) tbl.oid)))
	`, req.Schema, table, req.Name, partitionRoot)
	if err != nil {
		return nil, fmt.Errorf("query replica identity index: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	facts := &spec.PGReplicaIdentityIndex{Schema: req.Schema, Table: table, Name: req.Name, PartitionRoot: partitionRoot}
	var uniqueSupport, keys sql.NullBool
	if err := rows.Scan(&facts.Kind, &facts.IndexKnown, &facts.OnTable, &uniqueSupport, &facts.Unique, &facts.Immediate, &facts.Expression, &facts.Partial, &keys); err != nil {
		return nil, fmt.Errorf("scan replica identity index: %w", err)
	}
	if rows.Next() {
		return nil, fmt.Errorf("ambiguous replica identity index observation")
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	facts.CanUnique, facts.CanUniqueKnown = uniqueSupport.Bool, uniqueSupport.Valid
	facts.KeyAttributesValid, facts.KeyAttributesKnown = keys.Bool, keys.Valid
	return &spec.ObjectSnapshot{PGReplicaIdentity: facts}, nil
}
