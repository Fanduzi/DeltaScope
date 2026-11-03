// Package mysqlmeta reads TiDB placement-policy catalog facts (T20/#99).
package mysqlmeta

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// placementColumnProbe queries the catalog for one bounded column on one
// information_schema table — the same capability pattern the
// STATISTICS/SCHEMATA probes use: the answer reflects the connected server,
// not a version guess.
func (p *Provider) informationSchemaHasColumn(ctx context.Context, table, column string) (bool, error) {
	var n int
	err := p.db.QueryRowContext(ctx, `
		select count(*) from information_schema.`+"`columns`"+`
		where table_schema = 'information_schema'
			and table_name = ?
			and column_name = ?
	`, table, column).Scan(&n)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("probe %s column %s: %w", table, column, err)
	}
	return n > 0, nil
}

// LoadPlacementPolicies implements audit.PlacementPolicyLoader: the proven
// policy catalog — existence plus each policy's bounded bound-object set
// from SHOW PLACEMENT. A nil map return keeps the catalog unvouched (the
// PLACEMENT_POLICIES table is absent on MySQL and on TiDB versions without
// the surface); SHOW PLACEMENT is the authoritative bound-target read and
// its failure propagates rather than pretending an empty set.
func (p *Provider) LoadPlacementPolicies(ctx context.Context, dialect spec.Dialect) (map[string]*spec.PlacementPolicyFact, error) {
	if dialect != spec.DialectTiDB {
		return nil, nil
	}
	hasTable, err := p.informationSchemaHasColumn(ctx, "PLACEMENT_POLICIES", "POLICY_NAME")
	if err != nil {
		return nil, fmt.Errorf("probe placement policies catalog: %w", err)
	}
	if !hasTable {
		return nil, nil
	}
	catalog := map[string]*spec.PlacementPolicyFact{}
	rows, err := p.db.QueryContext(ctx, `
		select policy_name, primary_region, regions
		from information_schema.placement_policies
	`)
	if err != nil {
		return nil, fmt.Errorf("query placement policies: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, primaryRegion, regions sql.NullString
		if err := rows.Scan(&name, &primaryRegion, &regions); err != nil {
			return nil, fmt.Errorf("scan placement policy: %w", err)
		}
		if !name.Valid {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(name.String))
		if key == "" {
			continue
		}
		fact := &spec.PlacementPolicyFact{}
		if primaryRegion.Valid {
			fact.PrimaryRegion = primaryRegion.String
		}
		if regions.Valid {
			fact.Regions = regions.String
		}
		catalog[key] = fact
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate placement policies: %w", err)
	}
	if err := p.loadPlacementBoundTargets(ctx, catalog); err != nil {
		return nil, err
	}
	return catalog, nil
}

// loadPlacementBoundTargets reads SHOW PLACEMENT — the authoritative
// bound-object directory: POLICY rows open a group and each following
// target row belongs to it until the next POLICY row. Targets normalize to
// the same identity vocabulary the ordered model publishes
// ("table:<schema>.<name>", "partition:<schema>.<name>.<part>",
// "schema:<name>", "range:<name>"). An unrecognizable target shape fails
// the read rather than fabricating an incomplete bound set — the proven
// catalog then reports the whole load as failed, never half-empty.
func (p *Provider) loadPlacementBoundTargets(ctx context.Context, catalog map[string]*spec.PlacementPolicyFact) error {
	rows, err := p.db.QueryContext(ctx, `show placement`)
	if err != nil {
		return fmt.Errorf("query show placement: %w", err)
	}
	defer rows.Close()
	current := ""
	for rows.Next() {
		var target, placement, schedulingState sql.NullString
		if err := rows.Scan(&target, &placement, &schedulingState); err != nil {
			return fmt.Errorf("scan show placement row: %w", err)
		}
		if !target.Valid {
			continue
		}
		text := strings.TrimSpace(target.String)
		upper := strings.ToUpper(text)
		if strings.HasPrefix(upper, "POLICY ") {
			current = strings.ToLower(strings.TrimSpace(text[len("POLICY "):]))
			continue
		}
		if current == "" {
			continue
		}
		fact := catalog[current]
		if fact == nil {
			continue
		}
		ref, ok := normalizePlacementTarget(text)
		if !ok {
			return fmt.Errorf("unrecognized show placement target %q", text)
		}
		fact.BoundTargets = append(fact.BoundTargets, ref)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate show placement: %w", err)
	}
	return nil
}

// normalizePlacementTarget converts one SHOW PLACEMENT target cell into
// the ordered model's identity vocabulary. Recognized shapes:
//
//	TABLE <schema>.<name>                → table:<schema>.<name>
//	TABLE <schema>.<name> PARTITION <p>  → partition:<schema>.<name>.<p>
//	DATABASE <schema>                    → schema:<schema>
//	RANGE <TiDB_GLOBAL|TiDB_META|...>    → range:<global|meta|...>
func normalizePlacementTarget(text string) (string, bool) {
	upper := strings.ToUpper(text)
	switch {
	case strings.HasPrefix(upper, "TABLE "):
		body := text[len("TABLE "):]
		if i := strings.Index(strings.ToUpper(body), " PARTITION "); i >= 0 {
			full := body[:i]
			member := body[i+len(" PARTITION "):]
			return "partition:" + strings.ToLower(strings.TrimSpace(full)) + "." + strings.ToLower(strings.TrimSpace(member)), true
		}
		return "table:" + strings.ToLower(strings.TrimSpace(body)), true
	case strings.HasPrefix(upper, "DATABASE "):
		return "schema:" + strings.ToLower(strings.TrimSpace(text[len("DATABASE "):])), true
	case strings.HasPrefix(upper, "RANGE "):
		name := strings.ToLower(strings.TrimSpace(text[len("RANGE "):]))
		name = strings.TrimPrefix(name, "tidb_")
		return "range:" + name, true
	default:
		return "", false
	}
}

// loadTiDBPlacement reads the table-level bound policy name from the
// TiDB-only TABLES.TIDB_PLACEMENT_POLICY_NAME column. A missing column
// keeps the answer unvouched; a present column makes the row's NULL the
// proven-unbound answer.
func (p *Provider) loadTiDBPlacement(ctx context.Context, snapshot *spec.TableSnapshot) error {
	hasColumn, err := p.informationSchemaHasColumn(ctx, "TABLES", "TIDB_PLACEMENT_POLICY_NAME")
	if err != nil {
		return fmt.Errorf("probe tables placement column: %w", err)
	}
	if !hasColumn {
		return nil
	}
	var policy sql.NullString
	err = p.db.QueryRowContext(ctx, `
		select tidb_placement_policy_name
		from information_schema.tables
		where table_schema = ? and table_name = ?
	`, snapshot.Schema, snapshot.Table.Name).Scan(&policy)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("query table placement policy: %w", err)
	}
	snapshot.PlacementPolicyKnown = true
	if policy.Valid {
		snapshot.PlacementPolicy = strings.ToLower(strings.TrimSpace(policy.String))
	}
	return nil
}
