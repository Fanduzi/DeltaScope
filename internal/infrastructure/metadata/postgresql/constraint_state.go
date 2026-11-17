package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) loadPGConstraintStates(ctx context.Context, snapshot *spec.TableSnapshot) error {
	rows, err := p.db.QueryContext(ctx, `
		select pc.conname, pc.contype::text, pc.convalidated
		from pg_catalog.pg_constraint pc
		join pg_catalog.pg_class tbl on tbl.oid = pc.conrelid
		join pg_catalog.pg_namespace n on n.oid = tbl.relnamespace
		where n.nspname = $1 and tbl.relname = $2
		order by pc.oid
	`, snapshot.Schema, snapshot.Table.Name)
	if err != nil {
		return fmt.Errorf("query constraint states: %w", err)
	}
	defer rows.Close()

	states := make([]spec.PGConstraintState, 0)
	for rows.Next() {
		var state spec.PGConstraintState
		if err := rows.Scan(&state.Name, &state.Type, &state.Validated); err != nil {
			return fmt.Errorf("scan constraint state: %w", err)
		}
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
