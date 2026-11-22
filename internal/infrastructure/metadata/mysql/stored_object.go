package mysqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) loadMySQLStoredObject(ctx context.Context, req spec.ObjectLookupRequest, snapshot *spec.ObjectSnapshot) (*spec.ObjectSnapshot, error) {
	if req.Schema == "" {
		return snapshot, nil
	}
	target := spec.MySQLStoredObjectTarget{Kind: req.Qualifiers["kind"], Name: spec.Table{Schema: req.Schema, Name: req.Name},
		Related: spec.Table{Schema: req.Qualifiers["related_schema"], Name: req.Qualifiers["related_name"]}}
	state := spec.MySQLStoredObjectState{Target: target}
	var err error
	switch target.Kind {
	case "trigger":
		err = p.db.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM information_schema.TRIGGERS
				WHERE TRIGGER_SCHEMA = ? AND TRIGGER_NAME = ? AND BINARY TRIGGER_SCHEMA = BINARY ? AND BINARY TRIGGER_NAME = BINARY ?),
			       COALESCE(s.EVENT_OBJECT_SCHEMA,''), COALESCE(s.EVENT_OBJECT_TABLE,''),
			       COALESCE(LOWER(s.ACTION_TIMING),''), COALESCE(LOWER(s.EVENT_MANIPULATION),'')
			FROM (SELECT 1) AS witness LEFT JOIN information_schema.TRIGGERS s
			ON s.TRIGGER_SCHEMA = ? AND s.TRIGGER_NAME = ? AND BINARY s.TRIGGER_SCHEMA = BINARY ? AND BINARY s.TRIGGER_NAME = BINARY ?
		`, req.Schema, req.Name, req.Schema, req.Name, target.Related.Schema, target.Related.Name, target.Related.Schema, target.Related.Name).Scan(
			&state.SourceSeen, &state.SiblingTable.Schema, &state.SiblingTable.Name, &state.Timing, &state.Event)
		state.RelatedSeen = state.SiblingTable.Name != ""
	case "event":
		err = p.db.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM information_schema.EVENTS
				WHERE EVENT_SCHEMA = ? AND EVENT_NAME = ? AND BINARY EVENT_SCHEMA = BINARY ? AND BINARY EVENT_NAME = BINARY ?),
			       EXISTS(SELECT 1 FROM information_schema.EVENTS
				WHERE EVENT_SCHEMA = ? AND EVENT_NAME = ? AND BINARY EVENT_SCHEMA = BINARY ? AND BINARY EVENT_NAME = BINARY ?)
		`, req.Schema, req.Name, req.Schema, req.Name, target.Related.Schema, target.Related.Name, target.Related.Schema, target.Related.Name).Scan(
			&state.SourceSeen, &state.RelatedSeen)
	default:
		return snapshot, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return snapshot, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read MySQL trigger or event identities: %w", err)
	}
	snapshot.MySQLStoredObject = &state
	return snapshot, nil
}
