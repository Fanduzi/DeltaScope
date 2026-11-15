package mysqlmeta

// provider_server_drop_privilege.go implements the
// T26-SERVER-DROP-EXECUTION-01 privilege observation — the narrow
// evidence the audited identifier-form DROP SERVER slice's execution
// premise consumes. One dedicated provider connection reads the
// session's own CURRENT_USER() account and the verbatim SHOW GRANTS
// FOR CURRENT_USER() lines; both answers describe the same observed
// account on the same target, which the conditional audit's scope then
// binds as the expected execution principal.
//
// The honesty contract mirrors every other catalog read: connection,
// query, scan, and iteration errors propagate — a failed observation
// is never laundered into an unproven grant, an evidence gap, or an
// absence claim. A successful empty or unrecognised answer simply
// carries no positive witness. The raw account and grant text are
// internal evidence only — they never reach serialized output,
// findings, or public errors.

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// LoadMySQLServerDropPrivilege implements
// audit.MySQLServerDropPrivilegeLoader for the real MySQL provider.
// Non-MySQL dialects answer nil — the observation never cross-loads.
func (p *Provider) LoadMySQLServerDropPrivilege(ctx context.Context, dialect spec.Dialect) (*spec.MySQLServerDropPrivilegeObservation, error) {
	if dialect != spec.DialectMySQL {
		return nil, nil
	}
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire mysql server drop privilege session: %w", err)
	}
	defer conn.Close()

	observation := &spec.MySQLServerDropPrivilegeObservation{}
	// A rowset read, not QueryRow: the driver surfaces a deferred query
	// failure at QueryContext/Next/Err while a successful-but-empty
	// answer simply never calls Scan — the two outcomes stay distinct
	// instead of collapsing onto the sql.ErrNoRows sentinel
	// (T26-SERVER-DROP-EXECUTION-01 R1-C).
	rows, err := conn.QueryContext(ctx, `select current_user()`)
	if err != nil {
		return nil, fmt.Errorf("query current_user: %w", err)
	}
	for rows.Next() {
		var account string
		if err := rows.Scan(&account); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan current_user: %w", err)
		}
		observation.Account = account
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("read current_user rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close current_user rows: %w", err)
	}
	lines, err := loadMySQLGrantLines(ctx, conn, "show grants for current_user()")
	if err != nil {
		return nil, err
	}
	observation.GrantLines = lines
	return observation, nil
}
