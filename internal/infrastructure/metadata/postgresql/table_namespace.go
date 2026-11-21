package postgresqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveTableNamespace(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	table, action := req.Qualifiers["table"], req.Qualifiers["action"]
	if req.Schema == "" || req.Schema == "pg_temp" || table == "" || req.Name == "" {
		return nil, nil
	}
	var rename bool
	switch action {
	case "rename_table":
		rename = true
	case "set_schema":
		if req.Name == "pg_temp" {
			return nil, nil // A session alias is not a literal catalog namespace.
		}
	default:
		return nil, nil
	}
	facts := &spec.PGTableNamespace{Schema: req.Schema, Table: table, Action: action, Destination: req.Name}
	kind := "r"
	if req.Type == "view_namespace" {
		kind = req.Qualifiers["kind"]
		if kind != "v" && kind != "m" {
			return nil, nil
		}
		err := p.db.QueryRowContext(ctx, `
			select c.relkind::pg_catalog.text from pg_catalog.pg_class c
			join pg_catalog.pg_namespace n on n.oid OPERATOR(pg_catalog.=) c.relnamespace
			where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
			  and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		`, req.Schema, table).Scan(&facts.SourceKind)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("read view namespace source: %w", err)
		}
		facts.SourceKindKnown = true
		if facts.SourceKind != kind {
			return &spec.ObjectSnapshot{PGTableNamespace: facts}, nil
		}
	}
	// RenameTypeInternal can displace a defined true array when its element's
	// typarray identifies it. F_ARRAY_SUBSCRIPT_HANDLER is 6179 on PG17.
	row := p.db.QueryRowContext(ctx, `
		select dst.oid is not null, coalesce(dst.oid OPERATOR(pg_catalog.=) n.oid, false),
		       other.oid is not null,
		       case when typ.oid is null then false
		            when not $4::pg_catalog.bool or not typ.typisdefined then true
		            when typ.typsubscript OPERATOR(pg_catalog.<>) 6179::pg_catalog.oid
		              or typ.typelem OPERATOR(pg_catalog.=) 0::pg_catalog.oid then true
		            else elem.typarray OPERATOR(pg_catalog.<>) typ.oid end,
		         case when not $4::pg_catalog.bool and dst.oid OPERATOR(pg_catalog.<>) n.oid then exists (
		    select 1 from pg_catalog.pg_index i
		    join pg_catalog.pg_class idx on idx.oid OPERATOR(pg_catalog.=) i.indexrelid
		    join pg_catalog.pg_class occupied on occupied.relnamespace OPERATOR(pg_catalog.=) dst.oid
		      and occupied.relname OPERATOR(pg_catalog.=) idx.relname
		    where i.indrelid OPERATOR(pg_catalog.=) c.oid and i.indislive
		      and idx.relnamespace OPERATOR(pg_catalog.=) n.oid
		  ) else false end,
		  case when not $4::pg_catalog.bool and dst.oid OPERATOR(pg_catalog.<>) n.oid then exists (
		    select 1 from pg_catalog.pg_depend d
		    join pg_catalog.pg_class seq on seq.oid OPERATOR(pg_catalog.=) d.objid
		    join pg_catalog.pg_class occupied on occupied.relnamespace OPERATOR(pg_catalog.=) dst.oid
		      and occupied.relname OPERATOR(pg_catalog.=) seq.relname
		    where d.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		      and d.refobjid OPERATOR(pg_catalog.=) c.oid and d.refobjsubid OPERATOR(pg_catalog.<>) 0
		      and d.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		      and d.objsubid OPERATOR(pg_catalog.=) 0
		      and d.deptype OPERATOR(pg_catalog.=) any (array['a','i']::pg_catalog."char"[])
		      and seq.relkind OPERATOR(pg_catalog.=) 'S' and seq.relnamespace OPERATOR(pg_catalog.=) n.oid
		  ) else false end,
		  case when $4::pg_catalog.bool or dst.oid is null or dst.oid OPERATOR(pg_catalog.=) n.oid then null
		       when own_type.typarray OPERATOR(pg_catalog.=) 0::pg_catalog.oid then false
		       when associated.oid is null then null
		       else exists (select 1 from pg_catalog.pg_type occupied
		                    where occupied.typnamespace OPERATOR(pg_catalog.=) dst.oid
		                      and occupied.typname OPERATOR(pg_catalog.=) associated.typname) end
		from pg_catalog.pg_class c
		join pg_catalog.pg_namespace n on n.oid OPERATOR(pg_catalog.=) c.relnamespace
		join pg_catalog.pg_type own_type on own_type.oid OPERATOR(pg_catalog.=) c.reltype
		left join pg_catalog.pg_namespace dst on dst.nspname OPERATOR(pg_catalog.=)
		  case when $4::pg_catalog.bool then $1::pg_catalog.text else $3::pg_catalog.text end
		left join pg_catalog.pg_class other on other.relnamespace OPERATOR(pg_catalog.=) dst.oid
		  and other.relname OPERATOR(pg_catalog.=) case when $4::pg_catalog.bool then $3::pg_catalog.text else $2::pg_catalog.text end
		left join pg_catalog.pg_type typ on typ.typnamespace OPERATOR(pg_catalog.=) dst.oid
		  and typ.typname OPERATOR(pg_catalog.=) case when $4::pg_catalog.bool then $3::pg_catalog.text else own_type.typname::pg_catalog.text end
		left join pg_catalog.pg_type elem on elem.oid OPERATOR(pg_catalog.=) typ.typelem
		left join pg_catalog.pg_type associated on associated.oid OPERATOR(pg_catalog.=) own_type.typarray
		  and associated.typnamespace OPERATOR(pg_catalog.=) n.oid
		where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		  and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		  and c.relkind OPERATOR(pg_catalog.=) $5::pg_catalog."char" and c.relpersistence OPERATOR(pg_catalog.=) 'p'
		  and not c.relisshared and not c.relispartition
		  and c.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		  and not exists (select 1 from pg_catalog.pg_inherits h
		                  where h.inhrelid OPERATOR(pg_catalog.=) c.oid or h.inhparent OPERATOR(pg_catalog.=) c.oid)
	`, req.Schema, table, req.Name, rename, kind)
	var conflict, arrayConflict sql.NullBool
	if err := row.Scan(&facts.SchemaExists, &facts.SameSchema, &facts.RelationConflict, &conflict, &facts.IndexConflict, &facts.OwnedSequenceConflict, &arrayConflict); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("read table namespace destination: %w", err)
	}
	facts.RowTypeConflict, facts.RowTypeConflictKnown = conflict.Bool, conflict.Valid
	facts.ArrayTypeConflict, facts.ArrayTypeConflictKnown = arrayConflict.Bool, arrayConflict.Valid
	return &spec.ObjectSnapshot{PGTableNamespace: facts}, nil
}
