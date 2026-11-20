package postgresqlmeta

import (
	"context"
	"database/sql"
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
	collation := spec.PGColumnCollation{Schema: req.Qualifiers["collation_schema"], Name: req.Qualifiers["collation_name"]}
	schema, table, column := req.Qualifiers["source_schema"], req.Qualifiers["source_table"], req.Qualifiers["source_column"]
	row := p.db.QueryRowContext(ctx, `
		select n.oid is not null, base.oid is not null, resolved.oid is not null,
		       coalesce(resolved.typisdefined, false),
		       coalesce(resolved.typtype OPERATOR(pg_catalog.=) 'p', false)
		       or coalesce(elem.typtype OPERATOR(pg_catalog.=) 'p', false),
		       case when source_type.oid is null or resolved.oid is null then null
		            when source_type.typtype OPERATOR(pg_catalog.=) any(array['b','e']::pg_catalog."char"[])
		              and resolved.typtype OPERATOR(pg_catalog.=) any(array['b','e']::pg_catalog."char"[])
		              and source_type.typelem OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		              and resolved.typelem OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		            then source_type.oid OPERATOR(pg_catalog.=) resolved.oid or
		                 case when cast_entry.oid is null then resolved.typcategory OPERATOR(pg_catalog.=) 'S'
		                      else cast_entry.castcontext OPERATOR(pg_catalog.=) any(array['i','a']::pg_catalog."char"[]) end
		       end,
		       coalesce(resolved.typcollation OPERATOR(pg_catalog.<>) 0::pg_catalog.oid, false),
		       case when not $6::pg_catalog.bool then null
		            when exists (
		                 select 1 from pg_catalog.pg_collation col
		                 where col.collnamespace OPERATOR(pg_catalog.=) cn.oid
		                   and col.collname OPERATOR(pg_catalog.=) $5::pg_catalog.text
		                   and (col.collencoding OPERATOR(pg_catalog.=) enc.encoding
		                     or (col.collencoding OPERATOR(pg_catalog.=) -1
		                       and (col.collprovider OPERATOR(pg_catalog.<>) 'i'
		                         or enc.encoding OPERATOR(pg_catalog.=) pg_catalog.pg_char_to_encoding('UTF8'))))) then true
		            when exists (
		                 select 1 from pg_catalog.pg_collation col
		                 where col.collnamespace OPERATOR(pg_catalog.=) cn.oid
		                   and col.collname OPERATOR(pg_catalog.=) $5::pg_catalog.text
		                   and col.collencoding OPERATOR(pg_catalog.=) -1
		                   and col.collprovider OPERATOR(pg_catalog.=) 'i') then null
		            else false end
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_type base on base.typnamespace OPERATOR(pg_catalog.=) n.oid
		 and base.typname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_type resolved on resolved.oid OPERATOR(pg_catalog.=)
		 case when $3::pg_catalog.bool then base.typarray else base.oid end
		left join pg_catalog.pg_type elem on elem.oid OPERATOR(pg_catalog.=) resolved.typelem
		 and resolved.typsubscript OPERATOR(pg_catalog.=) 6179::pg_catalog.oid
		left join pg_catalog.pg_namespace cn on cn.nspname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		cross join (select pg_catalog.pg_char_to_encoding(pg_catalog.current_setting('server_encoding')) as encoding) enc
		left join pg_catalog.pg_namespace src on src.nspname OPERATOR(pg_catalog.=) $7::pg_catalog.text
		left join pg_catalog.pg_class tbl on tbl.relnamespace OPERATOR(pg_catalog.=) src.oid
		 and tbl.relname OPERATOR(pg_catalog.=) $8::pg_catalog.text
		 and tbl.relkind OPERATOR(pg_catalog.=) any(array['r','p']::pg_catalog."char"[])
		 and tbl.relpersistence OPERATOR(pg_catalog.=) 'p' and not tbl.relispartition
		 and tbl.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		 and ($10::pg_catalog.bool and exists (
		          select 1 from pg_catalog.pg_inherits h
		          where h.inhparent OPERATOR(pg_catalog.=) tbl.oid and not h.inhdetachpending)
		      or not $10::pg_catalog.bool and tbl.relkind OPERATOR(pg_catalog.=) 'r' and not exists (
		          select 1 from pg_catalog.pg_inherits h
		          where h.inhrelid OPERATOR(pg_catalog.=) tbl.oid or h.inhparent OPERATOR(pg_catalog.=) tbl.oid))
		left join pg_catalog.pg_attribute a on a.attrelid OPERATOR(pg_catalog.=) tbl.oid
		 and a.attname OPERATOR(pg_catalog.=) $9::pg_catalog.text
		 and a.attnum OPERATOR(pg_catalog.>) 0 and not a.attisdropped
		left join pg_catalog.pg_type source_type on source_type.oid OPERATOR(pg_catalog.=) a.atttypid
		left join pg_catalog.pg_cast cast_entry on cast_entry.castsource OPERATOR(pg_catalog.=) source_type.oid
		 and cast_entry.casttarget OPERATOR(pg_catalog.=) resolved.oid
	`, target.Schema, target.Name, target.Array, collation.Schema, collation.Name, collation.Qualified(), schema, table, column,
		req.Qualifiers["only_parent"] == "true" || req.Qualifiers["recursive_parent"] == "true")
	facts := spec.PGColumnTypeState{Target: target, Collation: collation, SourceSchema: schema, SourceTable: table, SourceColumn: column}
	var exists, assignment sql.NullBool
	if err := row.Scan(&facts.SchemaExists, &facts.BaseExists, &facts.Resolved, &facts.Defined, &facts.Pseudo, &assignment, &facts.Collatable, &exists); err != nil {
		return nil, fmt.Errorf("read column type destination: %w", err)
	}
	facts.CollationKnown, facts.CollationExists = exists.Valid, exists.Bool
	facts.AssignmentKnown, facts.Assignable = assignment.Valid, assignment.Bool
	return &spec.ObjectSnapshot{PGColumnType: &facts}, nil
}
