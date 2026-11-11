// Package tidbparser verifies the T26 SERVER coverage-boundary
// narrowing (issue #105): the MySQL identifier-form DROP SERVER, the
// audited CREATE SERVER first slice
// (T26-SERVER-CREATE-EXECUTION-01), and the audited ALTER SERVER
// first slice (T26-SERVER-ALTER-EXECUTION-01) leave the named
// unaudited boundary and enter the audited slice, while
// string-literal forms, non-mysql or literal wrappers, overlong
// spellings, and every TiDB SERVER statement keep their registered
// boundaries.
package tidbparser

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestT26DropServerIdentLeavesBoundary(t *testing.T) {
	for _, sql := range []string{
		"DROP SERVER s1",
		"DROP SERVER `s1`",
		"DROP SERVER IF EXISTS s1",
		"DROP SERVER ` s1`",
	} {
		statement := parseExtractT26(t, sql, spec.DialectMySQL)
		if statement.Unsupported != nil {
			t.Fatalf("%s: identifier drop must enter the audited slice, got %+v", sql, statement.Unsupported)
		}
		if statement.DDL == nil || statement.DDL.Server == nil || statement.DDL.Operation != spec.DDLOperationDropServer {
			t.Fatalf("%s: declaration lost: %+v", sql, statement.DDL)
		}
	}
}

func TestT26DropServerLiteralStaysUnaudited(t *testing.T) {
	statement := parseExtractT26(t, "DROP SERVER 's1'", spec.DialectMySQL)
	if statement.Unsupported == nil || statement.Unsupported.Reason != spec.UnsupportedUnauditedReason {
		t.Fatalf("literal drop must keep the unaudited boundary: %+v", statement.Unsupported)
	}
	if statement.Unsupported.Metadata["object"] != "server" {
		t.Fatalf("boundary metadata lost the server object marker: %+v", statement.Unsupported.Metadata)
	}
}

// The audited CREATE slice: identifier-form name within the rune bound
// plus an identifier-form mysql wrapper leaves the boundary and reaches
// adjudication. OPTIONS cleanliness is an adjudication premise, not a
// shape gate — unclean options still enter the slice.
func TestT26CreateServerIdentLeavesBoundary(t *testing.T) {
	for _, sql := range []string{
		"CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')",
		"CREATE SERVER `s1` FOREIGN DATA WRAPPER `mysql` OPTIONS (HOST 'h')",
		"CREATE SERVER s1 FOREIGN DATA WRAPPER MYSQL OPTIONS (PORT 100000)",
		"CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (USER 'u', USER 'u2')",
		// The 64-rune bound is inclusive — a name spelled at exactly
		// the documented mysql.servers bound still qualifies (R2-E).
		"CREATE SERVER " + strings.Repeat("n", 64) + " FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')",
	} {
		statement := parseExtractT26(t, sql, spec.DialectMySQL)
		if statement.Unsupported != nil {
			t.Fatalf("%s: qualified create must enter the audited slice, got %+v", sql, statement.Unsupported)
		}
		if statement.DDL == nil || statement.DDL.Server == nil || statement.DDL.Operation != spec.DDLOperationCreateServer {
			t.Fatalf("%s: declaration lost: %+v", sql, statement.DDL)
		}
	}
}

// Every CREATE outside the audited shape — literal names, literal or
// non-mysql wrappers, over-long names — keeps the unaudited boundary.
func TestT26ServerNonDropStaysUnaudited(t *testing.T) {
	for _, sql := range []string{
		"CREATE SERVER 's1' FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')",
		"CREATE SERVER s1 FOREIGN DATA WRAPPER 'mysql' OPTIONS (HOST 'h')",
		"CREATE SERVER s1 FOREIGN DATA WRAPPER pgsql OPTIONS (HOST 'h')",
		// One rune over the documented bound keeps the unaudited
		// boundary — qualification is lexical, never decoded (R2-E).
		"CREATE SERVER " + strings.Repeat("n", 65) + " FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')",
	} {
		statement := parseExtractT26(t, sql, spec.DialectMySQL)
		if statement.Unsupported == nil || statement.Unsupported.Reason != spec.UnsupportedUnauditedReason {
			t.Fatalf("%s: non-audited form must keep the unaudited boundary: %+v", sql, statement.Unsupported)
		}
	}
}

// The audited ALTER slice: an identifier-form name inside the same
// documented 64-rune bound leaves the boundary — ALTER declares no
// wrapper, so the only shape gates are the name's identifier form and
// its lexical length. OPTIONS cleanliness is an adjudication premise,
// not a shape gate — unclean options still enter the slice
// (T26-SERVER-ALTER-EXECUTION-01).
func TestT26AlterServerIdentLeavesBoundary(t *testing.T) {
	for _, sql := range []string{
		"ALTER SERVER s1 OPTIONS (HOST 'h2')",
		"ALTER SERVER `s1` OPTIONS (USER 'u')",
		"ALTER SERVER s1 OPTIONS (PORT 100000)",
		"ALTER SERVER s1 OPTIONS (USER 'u', USER 'u2')",
		// AM-3.x option-shape coverage: the in-range 9999 boundary and
		// one rung over it both stay inside the audited shape —
		// cleanliness is the adjudication premise, never the gate.
		"ALTER SERVER s1 OPTIONS (PORT 9999)",
		"ALTER SERVER s1 OPTIONS (PORT 10000)",
		// The 64-rune bound is inclusive — a name spelled at exactly
		// the documented mysql.servers bound still qualifies (R2-E).
		"ALTER SERVER " + strings.Repeat("n", 64) + " OPTIONS (HOST 'h')",
	} {
		statement := parseExtractT26(t, sql, spec.DialectMySQL)
		if statement.Unsupported != nil {
			t.Fatalf("%s: qualified alter must enter the audited slice, got %+v", sql, statement.Unsupported)
		}
		if statement.DDL == nil || statement.DDL.Server == nil || statement.DDL.Operation != spec.DDLOperationAlterServer {
			t.Fatalf("%s: declaration lost: %+v", sql, statement.DDL)
		}
	}
}

// Every ALTER outside the audited shape — literal names and
// over-long spellings — keeps the unaudited boundary; every TiDB
// SERVER form keeps the vendor boundary.
func TestT26AlterServerNonAuditedStaysUnaudited(t *testing.T) {
	for _, sql := range []string{
		"ALTER SERVER 's1' OPTIONS (HOST 'h')",
		// One rune over the documented bound keeps the unaudited
		// boundary — qualification is lexical, never decoded.
		"ALTER SERVER " + strings.Repeat("n", 65) + " OPTIONS (HOST 'h')",
	} {
		statement := parseExtractT26(t, sql, spec.DialectMySQL)
		if statement.Unsupported == nil || statement.Unsupported.Reason != spec.UnsupportedUnauditedReason {
			t.Fatalf("%s: non-audited alter must keep the unaudited boundary: %+v", sql, statement.Unsupported)
		}
	}
	statement := parseExtractT26(t, "ALTER SERVER s1 OPTIONS (HOST 'h')", spec.DialectTiDB)
	if statement.Unsupported == nil || statement.Unsupported.Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("tidb alter must keep the vendor boundary: %+v", statement.Unsupported)
	}
}

func TestT26DropServerTiDBVendorBoundary(t *testing.T) {
	statement := parseExtractT26(t, "DROP SERVER s1", spec.DialectTiDB)
	if statement.Unsupported == nil || statement.Unsupported.Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("tidb drop must keep the vendor boundary: %+v", statement.Unsupported)
	}
}
