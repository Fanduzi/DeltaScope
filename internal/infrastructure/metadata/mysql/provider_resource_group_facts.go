package mysqlmeta

// provider_resource_group_facts.go implements the T24/#103
// ordinary-client premise bundle — the facts the shared MySQL
// resource-group outcome consults that are SQL-observable: the global
// read-only variables, the executing subject's effective grants (base
// + active roles), the proven bound-group occupancy set, and the
// resource-group feature probe. Every read runs on one dedicated
// session so all answers describe a single consistent identity — the
// connection's own account, which is the subject that would execute
// the audited lifecycle statements.
//
// The honesty contract mirrors the catalog loaders: query, scan, and
// iteration errors propagate; an inapplicable channel (a missing
// CURRENT_ROLE on a pre-8.0 server, an absent performance_schema
// table/column) degrades to an unproven fact, never to a fabricated
// value. A privilege is proven negative only when the complete
// effective grant set was read — a partial read (unreadable active
// roles, a failed USING expansion) yields positive-only answers so a
// hidden grant never produces a false "denied" verdict.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	gomysql "github.com/go-sql-driver/mysql"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// LoadMySQLResourceGroupFacts implements
// audit.MySQLResourceGroupFactsLoader for the real MySQL provider.
// Non-MySQL dialects answer nil — the bundle never cross-loads.
func (p *Provider) LoadMySQLResourceGroupFacts(ctx context.Context, dialect spec.Dialect) (*spec.InstanceFacts, error) {
	if dialect != spec.DialectMySQL {
		return nil, nil
	}
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire mysql resource-group facts session: %w", err)
	}
	defer conn.Close()

	facts := &spec.InstanceFacts{}
	if err := loadMySQLReadOnlyVars(ctx, conn, facts); err != nil {
		return nil, err
	}
	privileges, grantsComplete, err := loadMySQLEffectivePrivileges(ctx, conn)
	if err != nil {
		return nil, err
	}
	if grantsComplete {
		// The complete effective set was read — both directions of
		// every consulted privilege are proven.
		facts.MySQLResourceGroupAdmin = privileges.Has("RESOURCE_GROUP_ADMIN") || privileges.Has("ALL PRIVILEGES")
		facts.MySQLResourceGroupAdminKnown = true
		facts.MySQLReadOnlyExempt = privileges.Has("SUPER") || privileges.Has("CONNECTION_ADMIN") || privileges.Has("ALL PRIVILEGES")
		facts.MySQLReadOnlyExemptKnown = true
	} else {
		// Partial visibility — only positive grants are proven; an
		// unobserved privilege stays unknown, never a guessed denial.
		if privileges.Has("RESOURCE_GROUP_ADMIN") || privileges.Has("ALL PRIVILEGES") {
			facts.MySQLResourceGroupAdmin = true
			facts.MySQLResourceGroupAdminKnown = true
		}
		if privileges.Has("SUPER") || privileges.Has("CONNECTION_ADMIN") || privileges.Has("ALL PRIVILEGES") {
			facts.MySQLReadOnlyExempt = true
			facts.MySQLReadOnlyExemptKnown = true
		}
	}
	if err := p.loadMySQLResourceGroupOccupancy(ctx, conn, facts, privileges.Has("PROCESS") || privileges.Has("ALL PRIVILEGES")); err != nil {
		return nil, err
	}
	if err := p.loadMySQLResourceGroupFeature(ctx, conn, facts); err != nil {
		return nil, err
	}
	return facts, nil
}

// loadMySQLReadOnlyVars reads @@global.read_only and
// @@global.super_read_only through SHOW GLOBAL VARIABLES — an absent
// variable returns a clean zero rows (unproven), so only observed
// answers ever set a fact.
func loadMySQLReadOnlyVars(ctx context.Context, conn *sql.Conn, facts *spec.InstanceFacts) error {
	rows, err := conn.QueryContext(ctx, `
		show global variables where variable_name in
		('read_only','super_read_only')
	`)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("query mysql read-only variables: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return fmt.Errorf("scan mysql read-only variable: %w", err)
		}
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "read_only":
			facts.MySQLReadOnly = normalizeOnOff(value)
			facts.MySQLReadOnlyKnown = true
		case "super_read_only":
			facts.MySQLSuperReadOnly = normalizeOnOff(value)
			facts.MySQLSuperReadOnlyKnown = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate mysql read-only variables: %w", err)
	}
	return nil
}

// mysqlRolePattern matches one quoted 'name'@'host' role identity in a
// CURRENT_ROLE() answer or a GRANT role line.
var mysqlRolePattern = regexp.MustCompile(`'[^']*'@'[^']*'`)

// mysqlGlobalGrantPattern matches a privilege-scoped SHOW GRANTS line —
// "GRANT <privileges> ON <scope> TO <grantee> ...". Role-assignment
// lines (GRANT 'r'@'%' TO ...) carry no ON clause and never match.
var mysqlGlobalGrantPattern = regexp.MustCompile(`(?i)^GRANT\s+(.+?)\s+ON\s+(\S+)\s+TO\s+`)

// mysqlPrivilegeSet is the union of global-scope privileges observed
// across a subject's effective grant lines.
type mysqlPrivilegeSet map[string]bool

// Has reports whether the global grant set carries a named privilege.
func (s mysqlPrivilegeSet) Has(privilege string) bool { return s[privilege] }

// loadMySQLEffectivePrivileges reads the subject's effective global
// privileges: the base SHOW GRANTS FOR CURRENT_USER() plus every
// active role's augmentation. CURRENT_ROLE() enumerates the session's
// active roles; when the function is unavailable the role set is
// unreadable, so completeness degrades rather than the read failing —
// a possibly-hidden role grant must never flip a positive answer into
// a proven denial.
func loadMySQLEffectivePrivileges(ctx context.Context, conn *sql.Conn) (mysqlPrivilegeSet, bool, error) {
	roles, rolesKnown, err := loadMySQLActiveRoles(ctx, conn)
	if err != nil {
		return nil, false, err
	}
	base, err := loadMySQLGrantLines(ctx, conn, "show grants for current_user()")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// The grant channel itself is unanswerable — the empty set
			// stays positive-only, completeness degrades.
			return mysqlPrivilegeSet{}, false, nil
		}
		return nil, false, err
	}
	set := mysqlPrivilegeSet{}
	collectMySQLGlobalPrivileges(set, base)
	complete := rolesKnown
	if rolesKnown && len(roles) > 0 {
		using := "show grants for current_user() using " + strings.Join(roles, ", ")
		augmented, augErr := loadMySQLGrantLines(ctx, conn, using)
		if augErr != nil {
			// The role set is known but its privileges are unreadable —
			// base answers stay positive-only, completeness degrades.
			return set, false, nil
		}
		collectMySQLGlobalPrivileges(set, augmented)
	}
	return set, complete, nil
}

// loadMySQLActiveRoles returns the session's active roles as quoted
// 'name'@'host' fragments. An unavailable CURRENT_ROLE() (pre-8.0
// servers, proxies) answers known=false rather than erroring — the
// role dimension stays unproven.
func loadMySQLActiveRoles(ctx context.Context, conn *sql.Conn) ([]string, bool, error) {
	var answer string
	if err := conn.QueryRowContext(ctx, `select current_role()`).Scan(&answer); err != nil {
		if err == sql.ErrNoRows || isMySQLMissingRoutineError(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("query current_role: %w", err)
	}
	answer = strings.TrimSpace(answer)
	if answer == "" || strings.EqualFold(answer, "none") {
		return nil, true, nil
	}
	return mysqlRolePattern.FindAllString(answer, -1), true, nil
}

// loadMySQLGrantLines returns every row of one SHOW GRANTS variant.
func loadMySQLGrantLines(ctx context.Context, conn *sql.Conn, statement string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, statement)
	if err != nil {
		return nil, fmt.Errorf("query mysql grants: %w", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, fmt.Errorf("scan mysql grant line: %w", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql grant lines: %w", err)
	}
	return lines, nil
}

// collectMySQLGlobalPrivileges folds every global-scope (*.*) grant
// line into the set. Privileges parse as bounded, exact tokens — never
// substring matches, so RESOURCE_GROUP_USER never counts as
// RESOURCE_GROUP_ADMIN.
func collectMySQLGlobalPrivileges(set mysqlPrivilegeSet, lines []string) {
	for _, line := range lines {
		match := mysqlGlobalGrantPattern.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil || match[2] != "*.*" {
			continue
		}
		for _, privilege := range strings.Split(match[1], ",") {
			privilege = strings.ToUpper(strings.TrimSpace(privilege))
			if privilege != "" {
				set[privilege] = true
			}
		}
	}
}

// loadMySQLResourceGroupOccupancy reads the bound-group set from
// performance_schema.threads.RESOURCE_GROUP. The observed names prove
// occupancy. A missing performance_schema table or RESOURCE_GROUP
// column degrades to unproven; every other failure propagates.
// BoundComplete is claimed only when the subject provably sees every
// thread's group (the PROCESS privilege or ALL at global scope) —
// partial visibility means a missing key is never a proven
// "unoccupied".
func (p *Provider) loadMySQLResourceGroupOccupancy(ctx context.Context, conn *sql.Conn, facts *spec.InstanceFacts, processGranted bool) error {
	var pfs string
	if err := conn.QueryRowContext(ctx, `select @@global.performance_schema`).Scan(&pfs); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("query performance_schema switch: %w", err)
	}
	if !normalizeOnOff(pfs) {
		return nil
	}
	rows, err := conn.QueryContext(ctx, `
		select distinct resource_group
		from performance_schema.threads
		where resource_group is not null
	`)
	if err != nil {
		if err == sql.ErrNoRows || isMySQLMissingObjectError(err) {
			return nil
		}
		return fmt.Errorf("query bound resource groups: %w", err)
	}
	defer rows.Close()
	bound := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan bound resource group: %w", err)
		}
		if trimmed := strings.ToLower(strings.TrimSpace(name)); trimmed != "" {
			bound[trimmed] = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate bound resource groups: %w", err)
	}
	facts.MySQLResourceGroupBound = bound
	facts.MySQLResourceGroupBoundComplete = processGranted
	return nil
}

// loadMySQLResourceGroupFeature reads the defensible feature-positive
// probe: the builtin usr_default row the resource-group component
// registers in information_schema.RESOURCE_GROUPS at post_init. A
// observed row proves the feature supported; a zero-row answer stays
// unproven — catalog visibility is not documented as unfiltered, so an
// empty read never claims the feature absent. A missing table
// (pre-8.0.3) likewise stays unproven — the version premise adjudicates
// that boundary.
func (p *Provider) loadMySQLResourceGroupFeature(ctx context.Context, conn *sql.Conn, facts *spec.InstanceFacts) error {
	var name string
	err := conn.QueryRowContext(ctx, `
		select resource_group_name
		from information_schema.resource_groups
		where resource_group_name = 'usr_default'
		limit 1
	`).Scan(&name)
	if err != nil {
		if err == sql.ErrNoRows || isMySQLMissingObjectError(err) {
			return nil
		}
		return fmt.Errorf("query mysql resource-group feature probe: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(name), "usr_default") {
		facts.MySQLResourceGroupFeatureSupported = true
		facts.MySQLResourceGroupFeatureKnown = true
	}
	return nil
}

// isMySQLMissingRoutineError reports whether an error is the server's
// "no such routine" answer for CURRENT_ROLE() on a pre-8.0 surface.
func isMySQLMissingRoutineError(err error) bool {
	var mysqlErr *gomysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1305
}

// isMySQLMissingObjectError reports whether an error is the server's
// "no such table/column" answer — an inapplicable catalog channel that
// leaves the fact unproven rather than a propagated failure.
func isMySQLMissingObjectError(err error) bool {
	var mysqlErr *gomysql.MySQLError
	if !errors.As(err, &mysqlErr) {
		return false
	}
	switch mysqlErr.Number {
	case 1054, 1109, 1146: // ER_BAD_FIELD_ERROR, ER_UNKNOWN_TABLE, ER_NO_SUCH_TABLE
		return true
	}
	return false
}
