// Package tidbparser verifies the T26 SERVER coverage-boundary
// narrowing (issue #105): the MySQL identifier-form DROP SERVER and the
// audited CREATE SERVER first slice (T26-SERVER-CREATE-EXECUTION-01)
// leave the named unaudited boundary and enter the audited slice, while
// string-literal forms, non-mysql or literal wrappers, ALTER SERVER,
// and every TiDB SERVER statement keep their registered boundaries.
package tidbparser

import (
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
// non-mysql wrappers, over-long names — keeps the unaudited boundary,
// and ALTER SERVER stays outside this slice entirely.
func TestT26ServerNonDropStaysUnaudited(t *testing.T) {
	for _, sql := range []string{
		"CREATE SERVER 's1' FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')",
		"CREATE SERVER s1 FOREIGN DATA WRAPPER 'mysql' OPTIONS (HOST 'h')",
		"CREATE SERVER s1 FOREIGN DATA WRAPPER pgsql OPTIONS (HOST 'h')",
		"ALTER SERVER s1 OPTIONS (HOST 'h2')",
	} {
		statement := parseExtractT26(t, sql, spec.DialectMySQL)
		if statement.Unsupported == nil || statement.Unsupported.Reason != spec.UnsupportedUnauditedReason {
			t.Fatalf("%s: non-audited form must keep the unaudited boundary: %+v", sql, statement.Unsupported)
		}
	}
}

func TestT26DropServerTiDBVendorBoundary(t *testing.T) {
	statement := parseExtractT26(t, "DROP SERVER s1", spec.DialectTiDB)
	if statement.Unsupported == nil || statement.Unsupported.Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("tidb drop must keep the vendor boundary: %+v", statement.Unsupported)
	}
}
