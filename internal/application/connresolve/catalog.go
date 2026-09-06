// Package connresolve turns caller connection input into a ready-to-open configuration.
// input: dialect plus MySQL/TiDB database, connection schema, and request default catalog hints
// output: one catalog and one qualifier, or ErrCatalogConflict
// pos: shared MySQL/TiDB catalog-alias rule for metadata-aware Audit and Online Query Access
// note: if this file changes, update this header and module README.md.
package connresolve

import (
	"errors"
	"strings"
)

// ErrCatalogConflict indicates MySQL/TiDB catalog hints disagree.
var ErrCatalogConflict = errors.New("MySQL/TiDB database, schema, and default schema must match when set; use one catalog value")

// BindMySQLTiDBCatalog canonicalizes MySQL/TiDB database, connection schema, and
// request default hints into one catalog and one qualifier. Non-MySQL/TiDB
// dialects return the database and requested schema unchanged.
func BindMySQLTiDBCatalog(dialect, database, connectionSchema, requestedSchema string) (string, string, error) {
	dialect = strings.ToLower(strings.TrimSpace(dialect))
	database = strings.TrimSpace(database)
	connectionSchema = strings.TrimSpace(connectionSchema)
	requestedSchema = strings.TrimSpace(requestedSchema)
	if dialect != "mysql" && dialect != "tidb" {
		return database, requestedSchema, nil
	}

	if database != "" && connectionSchema != "" && database != connectionSchema {
		return "", "", ErrCatalogConflict
	}
	catalog := database
	if catalog == "" {
		catalog = connectionSchema
	}
	if catalog != "" && requestedSchema != "" && catalog != requestedSchema {
		return "", "", ErrCatalogConflict
	}
	if requestedSchema != "" {
		return catalog, requestedSchema, nil
	}
	return catalog, catalog, nil
}
