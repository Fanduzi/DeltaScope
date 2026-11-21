package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveSequenceChange(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	state := spec.PGSequenceChangeState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}}
	var start, increment, minValue, maxValue, cache sql.NullInt64
	q := req.Qualifiers
	state.RenameTo, state.MoveTo = q["rename_to"], q["move_to"]
	destinationSchema, destinationName := req.Schema, state.RenameTo
	if state.MoveTo != "" {
		destinationSchema, destinationName = state.MoveTo, req.Name
	}
	owner := spec.PGSequenceOwnershipState{Sequence: state.Target.Table,
		Table: spec.Table{Schema: q["owner_schema"], Name: q["owner_table"]}, Column: q["owner_column"]}
	err := p.db.QueryRowContext(ctx, `
		select n.oid is not null, coalesce(c.relkind::pg_catalog.text, ''),
		       case q.seqtypid when 21 then 16 when 23 then 32 when 20 then 64 else 0 end,
		       q.seqstart,q.seqincrement,q.seqmin,q.seqmax,q.seqcache,
		       coalesce(owned.relkind::pg_catalog.text, ''),
		       coalesce(case when $3::pg_catalog.bool then r.oid OPERATOR(pg_catalog.=) owned.relowner else c.relowner OPERATOR(pg_catalog.=) owned.relowner end, false),
		       exists (select 1 from pg_catalog.pg_attribute a where a.attrelid OPERATOR(pg_catalog.=) owned.oid
		           and a.attname OPERATOR(pg_catalog.=) $6::pg_catalog.text and not a.attisdropped),
		       dep.identity, dep.owned, dn.oid is not null, dst.oid is not null,
		       coalesce(n.oid OPERATOR(pg_catalog.=) 99 or dn.oid OPERATOR(pg_catalog.=) 99
		           or dn.nspname::pg_catalog.text OPERATOR(pg_catalog.~) '^pg_(toast_)?temp_', false)
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_sequence q on q.seqrelid OPERATOR(pg_catalog.=) c.oid
		left join pg_catalog.pg_namespace ons on ons.nspname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		left join pg_catalog.pg_class owned on owned.relnamespace OPERATOR(pg_catalog.=) ons.oid and owned.relname OPERATOR(pg_catalog.=) $5::pg_catalog.text
		left join pg_catalog.pg_roles r on $3::pg_catalog.bool and r.rolname OPERATOR(pg_catalog.=) current_user
		left join pg_catalog.pg_namespace dn on dn.nspname OPERATOR(pg_catalog.=) $7::pg_catalog.text
		left join pg_catalog.pg_class dst on dst.relnamespace OPERATOR(pg_catalog.=) dn.oid and dst.relname OPERATOR(pg_catalog.=) $8::pg_catalog.text
		cross join lateral (
		    select coalesce(pg_catalog.bool_or(d.deptype OPERATOR(pg_catalog.=) 'i'), false) as identity, count(*) OPERATOR(pg_catalog.>) 0 as owned
		    from pg_catalog.pg_depend d where d.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		        and d.objid OPERATOR(pg_catalog.=) c.oid and d.objsubid OPERATOR(pg_catalog.=) 0
		        and d.refclassid OPERATOR(pg_catalog.=) 'pg_catalog.pg_class'::pg_catalog.regclass
		        and d.refobjsubid OPERATOR(pg_catalog.>) 0 and d.deptype OPERATOR(pg_catalog.=) any(array['a','i']::pg_catalog."char"[])
		) dep
	`, req.Schema, req.Name, q["create"] == "true", owner.Table.Schema, owner.Table.Name, owner.Column, destinationSchema, destinationName).Scan(
		&state.SchemaExists, &state.Kind, &state.Width, &start, &increment, &minValue, &maxValue, &cache,
		&owner.Kind, &owner.OwnerMatches, &owner.ColumnExists, &owner.IdentitySequence,
		&state.Owned, &state.DestinationSchema, &state.DestinationOccupied, &state.DestinationForbidden)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL sequence definition: %w", err)
	}
	if start.Valid && increment.Valid && minValue.Valid && maxValue.Valid && cache.Valid {
		state.Definition = &spec.SequenceDefinition{Start: &start.Int64, Increment: &increment.Int64,
			MinValue: &minValue.Int64, MaxValue: &maxValue.Int64, CacheValue: &cache.Int64}
	}
	if owner.Table.Name != "" || q["owner_none"] == "true" {
		state.Ownership = &owner
	}
	return &spec.ObjectSnapshot{PGSequenceChange: &state}, nil
}
