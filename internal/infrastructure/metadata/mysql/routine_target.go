package mysqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) loadMySQLRoutineTarget(ctx context.Context, req spec.ObjectLookupRequest, snapshot *spec.ObjectSnapshot) (*spec.ObjectSnapshot, error) {
	kind := spec.RoutineKind(req.Qualifiers["kind"])
	if req.Schema == "" || (kind != spec.RoutineKindProcedure && kind != spec.RoutineKindFunction) {
		return snapshot, nil
	}
	var witness int
	err := p.db.QueryRowContext(ctx, `
		SELECT 1 FROM information_schema.ROUTINES
		WHERE ROUTINE_SCHEMA = ? AND ROUTINE_NAME = ? AND ROUTINE_TYPE = UPPER(?)
		AND BINARY ROUTINE_SCHEMA = BINARY ? AND BINARY LOWER(ROUTINE_NAME) = BINARY LOWER(?)
		LIMIT 1`, req.Schema, req.Name, kind, req.Schema, req.Name).Scan(&witness)
	if errors.Is(err, sql.ErrNoRows) {
		// The routine directory can omit rows the metadata account cannot see.
		return snapshot, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read MySQL stored routine identity: %w", err)
	}
	snapshot.MySQLRoutine = &spec.MySQLRoutineTarget{Name: spec.Table{Schema: req.Schema, Name: req.Name}, Kind: kind}
	return snapshot, nil
}
