// Package tidbparser verifies the T26-SERVER-DROP-01 coverage-boundary
// narrowing (issue #105): the MySQL identifier-form DROP SERVER leaves
// the named unaudited boundary and enters the audited slice, while the
// string-literal drop, CREATE/ALTER forms, and every TiDB SERVER
// statement keep their registered boundaries.
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

func TestT26ServerNonDropStaysUnaudited(t *testing.T) {
	for _, sql := range []string{
		"CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')",
		"ALTER SERVER s1 OPTIONS (HOST 'h2')",
	} {
		statement := parseExtractT26(t, sql, spec.DialectMySQL)
		if statement.Unsupported == nil || statement.Unsupported.Reason != spec.UnsupportedUnauditedReason {
			t.Fatalf("%s: non-drop form must keep the unaudited boundary: %+v", sql, statement.Unsupported)
		}
	}
}

func TestT26DropServerTiDBVendorBoundary(t *testing.T) {
	statement := parseExtractT26(t, "DROP SERVER s1", spec.DialectTiDB)
	if statement.Unsupported == nil || statement.Unsupported.Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("tidb drop must keep the vendor boundary: %+v", statement.Unsupported)
	}
}
