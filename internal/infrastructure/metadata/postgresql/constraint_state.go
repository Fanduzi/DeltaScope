package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) loadPGConstraintStates(ctx context.Context, snapshot *spec.TableSnapshot) error {
	rows, err := p.db.QueryContext(ctx, `
		select pc.conname, pc.contype::pg_catalog.text, pc.convalidated,
		       pc.contype OPERATOR(pg_catalog.=) 'f' and tbl.relispartition
		         and tbl.relpersistence OPERATOR(pg_catalog.=) 'p'
		         and pc.conparentid OPERATOR(pg_catalog.<>) 0::pg_catalog.oid,
		       case when pc.contype OPERATOR(pg_catalog.=) 'c'
		                  and tbl.relkind OPERATOR(pg_catalog.=) any(array['r','p']::pg_catalog."char"[])
		                  and tbl.relpersistence OPERATOR(pg_catalog.=) 'p'
		                  and tbl.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		             then pc.coninhcount OPERATOR(pg_catalog.>) 0 end,
		       case when pc.contype OPERATOR(pg_catalog.=) 'c'
		                  and tbl.relkind OPERATOR(pg_catalog.=) 'p'
		                  and tbl.relpersistence OPERATOR(pg_catalog.=) 'p'
		                  and tbl.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		             then not pc.connoinherit and exists (
		                 select 1 from pg_catalog.pg_inherits h
		                 where h.inhparent OPERATOR(pg_catalog.=) tbl.oid and not h.inhdetachpending)
		             end,
		       case when pc.contype OPERATOR(pg_catalog.=) 'c' and not pc.convalidated
		                  and tbl.relkind OPERATOR(pg_catalog.=) any(array['r','p']::pg_catalog."char"[])
		                  and tbl.relpersistence OPERATOR(pg_catalog.=) 'p'
		                  and tbl.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		             then not pc.connoinherit and exists (
		                 select 1 from pg_catalog.pg_inherits h
		                 where h.inhparent OPERATOR(pg_catalog.=) tbl.oid and not h.inhdetachpending)
		             end
		from pg_catalog.pg_constraint pc
		join pg_catalog.pg_class tbl on tbl.oid OPERATOR(pg_catalog.=) pc.conrelid
		join pg_catalog.pg_namespace n on n.oid OPERATOR(pg_catalog.=) tbl.relnamespace
		where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		  and tbl.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		order by pc.oid
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		return fmt.Errorf("query constraint states: %w", err)
	}
	defer rows.Close()

	states := make([]spec.PGConstraintState, 0)
	for rows.Next() {
		var state spec.PGConstraintState
		var inherited, children, validationChildren sql.NullBool
		if err := rows.Scan(&state.Name, &state.Type, &state.Validated, &state.DerivedForeignKey, &inherited, &children, &validationChildren); err != nil {
			return fmt.Errorf("scan constraint state: %w", err)
		}
		state.InheritanceKnown, state.Inherited = inherited.Valid, inherited.Bool
		state.PartitionCheckHasChildrenKnown, state.PartitionCheckHasChildren = children.Valid, children.Bool
		state.CheckValidationChildrenKnown, state.CheckValidationHasChildren = validationChildren.Valid, validationChildren.Bool
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate constraint states: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close constraint states: %w", err)
	}
	snapshot.PGConstraints = states
	return nil
}
