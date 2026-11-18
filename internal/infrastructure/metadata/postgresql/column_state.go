package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) loadPGColumnCatalog(ctx context.Context, snapshot *spec.TableSnapshot) error {
	rows, err := p.db.QueryContext(ctx, `
		select (c.relkind = 'r' and c.relpersistence = 'p' and not c.relispartition
		        and not exists (select 1 from pg_catalog.pg_inherits inh where inh.inhrelid = c.oid or inh.inhparent = c.oid)) as generation_ordinary,
		       a.attname, a.attidentity::text, a.attgenerated::text,
		       a.atttypid, a.attnotnull, a.atthasdef,
		       exists (select 1 from pg_catalog.pg_constraint pk where pk.conrelid = c.oid and pk.contype = 'p' and a.attnum = any(pk.conkey)),
		       seq.seqtypid, seq.seqstart, seq.seqincrement, seq.seqmin, seq.seqmax, seq.seqcache, seq.seqcycle,
		       a.attnum, typ.typstorage::text
		from pg_catalog.pg_class c
		join pg_catalog.pg_namespace n on n.oid = c.relnamespace
		left join pg_catalog.pg_attribute a on a.attrelid = c.oid and a.attnum OPERATOR(pg_catalog.<>) 0 and not a.attisdropped
		left join pg_catalog.pg_type typ on typ.oid OPERATOR(pg_catalog.=) a.atttypid
		left join pg_catalog.pg_depend dep on dep.refclassid = 'pg_catalog.pg_class'::pg_catalog.regclass
		  and dep.classid = 'pg_catalog.pg_class'::pg_catalog.regclass and dep.refobjid = c.oid
		  and dep.refobjsubid = a.attnum and dep.objsubid = 0 and dep.deptype = 'i'
		left join pg_catalog.pg_sequence seq on seq.seqrelid = dep.objid
		where n.nspname = $1 and c.relname = $2
		order by a.attnum, dep.objid
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		return fmt.Errorf("query column generation catalog: %w", err)
	}
	defer rows.Close()
	var catalog *spec.PGColumnCatalog
	for rows.Next() {
		var ordinary, pk bool
		var name, identity, generated, typeStorage sql.NullString
		var typeOID, seqType, start, increment, min, max, cache, attributeNumber sql.NullInt64
		var notNull, hasDefault, cycle sql.NullBool
		if err := rows.Scan(&ordinary, &name, &identity, &generated, &typeOID, &notNull, &hasDefault, &pk,
			&seqType, &start, &increment, &min, &max, &cache, &cycle, &attributeNumber, &typeStorage); err != nil {
			return fmt.Errorf("scan column generation catalog: %w", err)
		}
		if catalog == nil {
			catalog = &spec.PGColumnCatalog{Ordinary: ordinary, Columns: make([]spec.PGColumnState, 0), SystemColumns: make([]spec.PGColumnState, 0)}
		} else if catalog.Ordinary != ordinary {
			return fmt.Errorf("inconsistent column generation table scope")
		}
		if !name.Valid {
			continue // LEFT JOIN preserves a proven zero-column table.
		}
		if !identity.Valid || !generated.Valid || !typeOID.Valid || !notNull.Valid || !hasDefault.Valid || !attributeNumber.Valid || attributeNumber.Int64 == 0 || !typeStorage.Valid {
			return fmt.Errorf("incomplete column generation member")
		}
		state := spec.PGColumnState{Name: name.String, Identity: identity.String, Generated: generated.String,
			TypeWidth: pgCatalogIntegerWidth(typeOID.Int64), TypeStorage: typeStorage.String, System: attributeNumber.Int64 < 0,
			NotNull: notNull.Bool, HasDefault: hasDefault.Bool, PrimaryKey: pk}
		if seqType.Valid && start.Valid && increment.Valid && min.Valid && max.Valid && cache.Valid && cycle.Valid {
			state.IdentitySequence = &spec.PGIdentitySequenceState{TypeWidth: pgCatalogIntegerWidth(seqType.Int64), Definition: spec.SequenceDefinition{
				Start: &start.Int64, Increment: &increment.Int64, MinValue: &min.Int64, MaxValue: &max.Int64, CacheValue: &cache.Int64, Cycle: &cycle.Bool,
			}}
		}
		if state.System {
			catalog.SystemColumns = append(catalog.SystemColumns, state)
		} else {
			catalog.Columns = append(catalog.Columns, state)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate column generation catalog: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close column generation catalog: %w", err)
	}
	snapshot.PGColumns = catalog // No rows means the previously observed table is no longer proven.
	return nil
}

func pgCatalogIntegerWidth(oid int64) int {
	switch oid {
	case 21:
		return 16
	case 23:
		return 32
	case 20:
		return 64
	default:
		return -1
	}
}
