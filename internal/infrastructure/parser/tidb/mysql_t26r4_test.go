// Package tidbparser verifies the T26-R4 corrections to the MySQL
// SERVER/SRS recognizers (issue #105): OPTIONS lists require at least
// one complete member with no trailing comma, ORGANIZATION must fully
// consume its IDENTIFIED BY pair, unquoted server names obey the
// upstream ident rule (no reserved words, no pure digits), unclosed
// literals stay unclaimed, and the declaration facts carry accurate
// units — the name bound counts characters, and the public SQL renders
// the fixed redacted skeletons.
package tidbparser

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func parseExtractT26(t *testing.T, sql string, dialect spec.Dialect) spec.Statement {
	t.Helper()
	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 ast statement, got %d", len(result.Statements))
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	statement, err := wrapped[0].Extractor.Extract(dialect, wrapped[0].RawSQL)
	if err != nil {
		t.Fatalf("extract statement: %v", err)
	}
	return statement
}

func parseErrT26(t *testing.T, sql string) error {
	t.Helper()
	parser := New()
	_, err := parser.Parse(context.Background(), sql)
	return err
}

// R4-01: OPTIONS must hold at least one complete member — empty lists,
// trailing commas, consecutive commas, and missing values fall back to
// the base parser's diagnostics. Legal neighbors still land.
func TestT26R4ServerOptionsStrict(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS ()",
		"ALTER SERVER s OPTIONS ()",
		"ALTER SERVER s OPTIONS (HOST 'x',)",
		"CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'x',)",
		"ALTER SERVER s OPTIONS (HOST 'x',, PORT 1)",
		"ALTER SERVER s OPTIONS (HOST, PORT 1)",
		"ALTER SERVER s OPTIONS (HOST 'x', PORT)",
	} {
		if err := parseErrT26(t, sql); err == nil {
			t.Fatalf("%q: malformed OPTIONS claimed or silently accepted", sql)
		}
	}
	for _, sql := range []string{
		"CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'x')",
		"ALTER SERVER s OPTIONS (HOST 'x', PORT 3306)",
	} {
		stmt := parseExtractT26(t, sql, spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil {
			t.Fatalf("%q: legal OPTIONS form not recognized", sql)
		}
	}
}

// R4-02: ORGANIZATION requires the complete IDENTIFIED BY pair —
// missing, partial, non-numeric, and overflowing ids stay unclaimed.
func TestT26R4SRSOrgPairing(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"CREATE SPATIAL REFERENCE SYSTEM 1 NAME 'a' DEFINITION 'd' ORGANIZATION 'EPSG'",
		"CREATE SPATIAL REFERENCE SYSTEM 1 NAME 'a' DEFINITION 'd' ORGANIZATION 'EPSG' DESCRIPTION 'x'",
		"CREATE SPATIAL REFERENCE SYSTEM 1 NAME 'a' DEFINITION 'd' ORGANIZATION 'EPSG' IDENTIFIED",
		"CREATE SPATIAL REFERENCE SYSTEM 1 NAME 'a' DEFINITION 'd' ORGANIZATION 'EPSG' IDENTIFIED BY",
		"CREATE SPATIAL REFERENCE SYSTEM 1 NAME 'a' DEFINITION 'd' ORGANIZATION 'EPSG' IDENTIFIED BY 'abc'",
		"CREATE SPATIAL REFERENCE SYSTEM 1 NAME 'a' DEFINITION 'd' ORGANIZATION 'EPSG' IDENTIFIED BY 4294967296",
	} {
		if err := parseErrT26(t, sql); err == nil {
			t.Fatalf("%q: incomplete ORGANIZATION pair claimed", sql)
		}
	}
	stmt := parseExtractT26(t,
		"CREATE SPATIAL REFERENCE SYSTEM 1 NAME 'a' DEFINITION 'd' ORGANIZATION 'EPSG' IDENTIFIED BY 1234 DESCRIPTION 'x'",
		spec.DialectMySQL)
	decl := stmt.DDL.SpatialReference
	if decl == nil || !decl.Attrs.HasIdentifiedBy || decl.Attrs.OrgID != 1234 {
		t.Fatalf("complete pair: attrs=%+v", decl.Attrs)
	}
}

// R4-03a: unquoted server names obey the upstream ident rule — pure
// digits and reserved words stay unclaimed; backtick-quoted spellings
// are still legal names.
func TestT26R4ServerNameIdentRule(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"DROP SERVER 123",
		"DROP SERVER SELECT",
		"ALTER SERVER 123 OPTIONS (HOST 'x')",
	} {
		if err := parseErrT26(t, sql); err == nil {
			t.Fatalf("%q: non-identifier name claimed", sql)
		}
	}
	stmt := parseExtractT26(t, "DROP SERVER `select`", spec.DialectMySQL)
	if stmt.DDL == nil || stmt.DDL.Server == nil || stmt.DDL.Server.Name != "select" {
		t.Fatalf("backticked reserved name: ddl=%+v", stmt.DDL)
	}
}

// R4-03b: unclosed literals never produce a claim or a silently
// truncated name — the failure keeps the base diagnostic, no panic.
func TestT26R4UnclosedTokens(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{
		"CREATE SERVER 'abc FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'x')",
		"ALTER SERVER s OPTIONS (HOST 'x)",
		"DROP SERVER `abc",
		"CREATE SPATIAL REFERENCE SYSTEM 1 NAME 'a DEFINITION 'd'",
		"CREATE SPATIAL REFERENCE SYSTEM 1 DEFINITION 'd",
	} {
		if err := parseErrT26(t, sql); err == nil {
			t.Fatalf("%q: unclosed literal claimed", sql)
		}
	}
}

// R4-03c: the documented 64-character name bound counts characters,
// not bytes — a 65-rune multibyte name trips it, a 64-rune one does not.
func TestT26R4ServerNameCharLimit(t *testing.T) {
	t.Parallel()
	ascii64 := strings.Repeat("a", 64)
	ascii65 := strings.Repeat("a", 65)
	multi64 := strings.Repeat("é", 64) // 64 chars, 128 bytes
	for _, c := range []struct {
		name string
		want bool
	}{
		{ascii64, false},
		{ascii65, true},
		{multi64, false},
		{strings.Repeat("é", 65), true},
	} {
		stmt := parseExtractT26(t, "DROP SERVER `"+c.name+"`", spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil {
			t.Fatalf("name len %d: not recognized", len(c.name))
		}
		if got := stmt.DDL.Server.NameOverLimit; got != c.want {
			t.Fatalf("name chars %d (bytes %d): NameOverLimit=%v want %v — byte/char confusion",
				len([]rune(c.name)), len(c.name), got, c.want)
		}
	}
}

// R4-04: the public SQL renders the fixed redacted skeletons — server
// identity and wrapper never leave the internal declaration.
func TestT26R4PublicSQLSkeletons(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		sql  string
		want string
	}{
		{"CREATE SERVER golden_srv FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', PASSWORD 'p')", "CREATE SERVER <redacted>"},
		{"ALTER SERVER golden_srv OPTIONS (HOST 'h')", "ALTER SERVER <redacted>"},
		{"DROP SERVER golden_srv", "DROP SERVER <redacted>"},
		{"DROP SERVER IF EXISTS golden_srv", "DROP SERVER IF EXISTS <redacted>"},
		{"CREATE SPATIAL REFERENCE SYSTEM IF NOT EXISTS 4120 NAME 'G' DEFINITION 'x'", "CREATE SPATIAL REFERENCE SYSTEM IF NOT EXISTS 4120 <attributes redacted>"},
		{"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 40001", "CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 40001"},
		{"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 40001", "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 40001"},
	} {
		stmt := parseExtractT26(t, c.sql, spec.DialectMySQL)
		if got := stmt.PublicSQL(); got != c.want {
			t.Fatalf("%q: PublicSQL=%q want %q", c.sql, got, c.want)
		}
		if got := stmt.PublicNormalizedSQL(); got != c.want {
			t.Fatalf("%q: PublicNormalizedSQL=%q want %q", c.sql, got, c.want)
		}
	}
	// Internal declaration keeps identity — public form drops it.
	stmt := parseExtractT26(t, "CREATE SERVER golden_srv FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')", spec.DialectMySQL)
	if stmt.DDL.Server.Name != "golden_srv" || stmt.DDL.Server.Wrapper != "mysql" {
		t.Fatalf("internal identity lost: %+v", stmt.DDL.Server)
	}
}
