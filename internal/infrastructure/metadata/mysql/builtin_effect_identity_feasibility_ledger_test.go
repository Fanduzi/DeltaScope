// Package mysqlmeta records the MySQL 8.4 and TiDB 8.5 builtin-effect identity
// feasibility evidence (Tasks 3 and 4). These tests lock the Docker probe
// evidence without creating promotion code.
//
// input: live Docker MySQL 8.4 and TiDB 8.5 probe conclusions
// output: per-dialect KILL disposition with test-locked evidence
// pos: research evidence only; MySQL and TiDB are independent proof domains
package mysqlmeta

import "testing"

// mysql84FeasibilityEvidence locks the MySQL 8.4 Docker probe evidence.
// Probed against docker/cli-e2e-compose.yaml mysql:8.4 (MySQL 8.4.10).
type mysql84FeasibilityEvidence struct {
	ServerVersion               string
	StoredFunctionSupported     bool
	StoredFunctionCanDeclareDet bool
	BuiltinNameShadowingBlocked bool
	UDFTableExists              bool
	PerfSchemaUDFsListsPlugins  bool
	BuiltinOIDTableExists       bool
	ExplainRevealsBuiltinName   bool
	ExplainRevealsBuiltinOID    bool
	SchemaQualifiedCallWorks    bool
	MetadataOnOneConnection     bool
}

// tidb85FeasibilityEvidence locks the TiDB 8.5 Docker probe evidence.
// Probed against docker/cli-e2e-compose.yaml pingcap/tidb:v8.5.0
// (8.0.11-TiDB-v8.5.0).
type tidb85FeasibilityEvidence struct {
	ServerVersion               string
	StoredFunctionSupported     bool
	LoadableUDFSupported        bool
	BuiltinNameShadowingBlocked bool
	BuiltinOIDTableExists       bool
	ExplainRevealsBuiltinName   bool
	ExplainRevealsBuiltinOID    bool
	PluginsTableHasEntries      bool
	MetadataOnOneConnection     bool
}

// The probed facts below were captured against live Docker services:
//   MySQL 8.4.10 on port 3406 (deltascope-cli-e2e-mysql)
//   TiDB v8.5.0 on port 4400 (deltascope-cli-e2e-tidb)

var mysql84Evidence = mysql84FeasibilityEvidence{
	ServerVersion:               "8.4.10",
	StoredFunctionSupported:     true,
	StoredFunctionCanDeclareDet: true,
	BuiltinNameShadowingBlocked: true,
	UDFTableExists:              true,
	PerfSchemaUDFsListsPlugins:  true,
	BuiltinOIDTableExists:       false,
	ExplainRevealsBuiltinName:   true,
	ExplainRevealsBuiltinOID:    false,
	SchemaQualifiedCallWorks:    true,
	MetadataOnOneConnection:     true,
}

var tidb85Evidence = tidb85FeasibilityEvidence{
	ServerVersion:               "8.0.11-TiDB-v8.5.0",
	StoredFunctionSupported:     false,
	LoadableUDFSupported:        false,
	BuiltinNameShadowingBlocked: true,
	BuiltinOIDTableExists:       false,
	ExplainRevealsBuiltinName:   true,
	ExplainRevealsBuiltinOID:    false,
	PluginsTableHasEntries:      false,
	MetadataOnOneConnection:     true,
}

// TestMySQL84_KillDisposition locks the MySQL 8.4 KILL result.
// The kill criterion is met: the only available builtin identity is the
// function name, and no server facility returns a unique non-name identity.
// A name-based allowlist is explicitly forbidden by the decision.
func TestMySQL84_KillDisposition(t *testing.T) {
	t.Parallel()
	e := mysql84Evidence

	if e.ServerVersion == "" {
		t.Fatal("server version must be recorded")
	}

	// MySQL 8.4 supports stored functions with DETERMINISTIC.
	if !e.StoredFunctionSupported {
		t.Fatal("MySQL 8.4 must support stored functions (probe evidence)")
	}
	if !e.StoredFunctionCanDeclareDet {
		t.Fatal("MySQL 8.4 stored functions must be able to declare DETERMINISTIC")
	}

	// MySQL blocks CREATE FUNCTION with builtin names (count, COUNT).
	if !e.BuiltinNameShadowingBlocked {
		t.Fatal("MySQL 8.4 must reject CREATE FUNCTION with builtin names")
	}

	// mysql.func exists but only for loadable UDFs.
	if !e.UDFTableExists {
		t.Fatal("mysql.func must exist for loadable UDFs")
	}

	// performance_schema.user_defined_functions lists plugin UDFs only.
	if !e.PerfSchemaUDFsListsPlugins {
		t.Fatal("performance_schema.user_defined_functions must list plugin UDFs")
	}

	// No OID-equivalent builtin identity table exists.
	if e.BuiltinOIDTableExists {
		t.Fatal("MySQL 8.4 must not have a builtin OID-equivalent identity table")
	}

	// EXPLAIN shows function names in plan text but not OIDs.
	if !e.ExplainRevealsBuiltinName {
		t.Fatal("EXPLAIN must reveal builtin function names in plan text")
	}
	if e.ExplainRevealsBuiltinOID {
		t.Fatal("EXPLAIN must not reveal builtin function OIDs")
	}

	// Schema-qualified stored function calls work.
	if !e.SchemaQualifiedCallWorks {
		t.Fatal("schema-qualified stored function calls must work")
	}

	// Metadata is available on one connection.
	if !e.MetadataOnOneConnection {
		t.Fatal("metadata must be available on one connection")
	}

	// Kill criterion: the best available identity is the function name.
	// Determinism is not a trust root. No OID-equivalent binding exists.
	// Therefore MySQL 8.4 is KILL — a name allowlist is forbidden.
}

// TestTiDB85_KillDisposition locks the TiDB 8.5 KILL result.
// The kill criterion is met independently of MySQL: TiDB has no stored
// functions, no loadable UDFs, no builtin identity catalog, and no server
// facility that returns a unique non-name identity. The function name is
// the only available root, which is explicitly forbidden.
func TestTiDB85_KillDisposition(t *testing.T) {
	t.Parallel()
	e := tidb85Evidence

	if e.ServerVersion == "" {
		t.Fatal("server version must be recorded")
	}

	// TiDB does NOT support CREATE FUNCTION.
	if e.StoredFunctionSupported {
		t.Fatal("TiDB 8.5 must not support CREATE FUNCTION")
	}

	// TiDB does NOT support loadable UDFs (mysql.func doesn't exist).
	if e.LoadableUDFSupported {
		t.Fatal("TiDB 8.5 must not support loadable UDFs")
	}

	// Builtin name shadowing is blocked (no stored functions at all).
	if !e.BuiltinNameShadowingBlocked {
		t.Fatal("TiDB 8.5 must block builtin name shadowing")
	}

	// No OID-equivalent builtin identity table exists.
	if e.BuiltinOIDTableExists {
		t.Fatal("TiDB 8.5 must not have a builtin OID-equivalent identity table")
	}

	// EXPLAIN shows function names in plan text but not OIDs.
	if !e.ExplainRevealsBuiltinName {
		t.Fatal("EXPLAIN must reveal builtin function names in plan text")
	}
	if e.ExplainRevealsBuiltinOID {
		t.Fatal("EXPLAIN must not reveal builtin function OIDs")
	}

	// information_schema.PLUGINS returns 0 rows.
	if e.PluginsTableHasEntries {
		t.Fatal("TiDB 8.5 PLUGINS table must be empty")
	}

	// Metadata is available on one connection.
	if !e.MetadataOnOneConnection {
		t.Fatal("metadata must be available on one connection")
	}

	// Kill criterion: the only available identity is the function name.
	// No OID-equivalent binding exists. Therefore TiDB 8.5 is KILL —
	// a name allowlist is forbidden.
}

// TestMySQLTiDB_IndependentProofDomains verifies that MySQL and TiDB evidence
// are evaluated independently. MySQL evidence does not promote TiDB, and vice
// versa. Both dialects reach KILL independently.
func TestMySQLTiDB_IndependentProofDomains(t *testing.T) {
	t.Parallel()

	// MySQL KILL is based on: stored functions with DETERMINISTIC exist, no
	// OID-equivalent identity, name is the only root.
	mysqlKill := !mysql84Evidence.BuiltinOIDTableExists && mysql84Evidence.StoredFunctionCanDeclareDet

	// TiDB KILL is based on: no stored functions, no UDFs, no OID-equivalent
	// identity, name is the only root.
	tidbKill := !tidb85Evidence.BuiltinOIDTableExists && !tidb85Evidence.StoredFunctionSupported

	if !mysqlKill {
		t.Fatal("MySQL 8.4 must independently reach KILL")
	}
	if !tidbKill {
		t.Fatal("TiDB 8.5 must independently reach KILL")
	}

	// The evidence paths are different: MySQL has stored functions, TiDB does not.
	// Neither dialect's evidence is inferred from the other.
	if mysql84Evidence.StoredFunctionSupported == tidb85Evidence.StoredFunctionSupported {
		// This is fine as long as the KILL is independently justified.
		// But it highlights that the evidence is different: MySQL supports
		// stored functions (shadowing risk), TiDB does not (different gap).
	}
}
