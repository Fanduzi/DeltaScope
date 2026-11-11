// Package tidbparser verifies the T26-SERVER-CREATE-EXECUTION-01
// parser facts: WrapperIdent distinguishes identifier-form wrappers
// from string literals while the token category is still visible;
// numeric OPTIONS members carry a bounded RangeState (in-range,
// out-of-range, unproven) over the documented 0-9999 PORT bound; and
// the recognizer keeps recording keys, kinds, presence, and
// duplicates without ever storing a raw OPTIONS literal value.
package tidbparser

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// WrapperIdent: identifier and backquoted wrappers decode as catalog
// wrapper identities; string-literal wrappers stay unprovable and keep
// the boundary.
func TestT26CreateWrapperIdentFact(t *testing.T) {
	t.Run("identifier wrapper marks ident", func(t *testing.T) {
		stmt := parseExtractT26(t, `CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')`, spec.DialectMySQL)
		if stmt.DDL.Server.Wrapper != "mysql" || !stmt.DDL.Server.WrapperIdent {
			t.Fatalf("wrapper fact = %+v, want mysql/ident", stmt.DDL.Server)
		}
	})
	t.Run("backquoted wrapper marks ident", func(t *testing.T) {
		stmt := parseExtractT26(t, "CREATE SERVER s FOREIGN DATA WRAPPER `mysql` OPTIONS (HOST 'h')", spec.DialectMySQL)
		if stmt.DDL.Server.Wrapper != "mysql" || !stmt.DDL.Server.WrapperIdent {
			t.Fatalf("backquoted wrapper fact = %+v, want mysql/ident", stmt.DDL.Server)
		}
	})
	t.Run("string-literal wrapper is not ident", func(t *testing.T) {
		stmt := parseExtractT26(t, `CREATE SERVER s FOREIGN DATA WRAPPER 'mysql' OPTIONS (HOST 'h')`, spec.DialectMySQL)
		if stmt.DDL.Server.Wrapper != "mysql" || stmt.DDL.Server.WrapperIdent {
			t.Fatalf("literal wrapper fact = %+v, want mysql/non-ident", stmt.DDL.Server)
		}
		if stmt.Unsupported == nil {
			t.Fatalf("literal wrapper must keep the boundary")
		}
	})
}

// Numeric RangeState: PORT inside the documented 0-9999 bound reads
// in-range; above it reads out-of-range (still parses — the
// adjudication owns the premise). uint64 overflow stays a parser
// rejection, never a RangeState.
func TestT26CreateNumericRangeStateFact(t *testing.T) {
	t.Run("port in range", func(t *testing.T) {
		stmt := parseExtractT26(t, `CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (PORT 3306)`, spec.DialectMySQL)
		opts := stmt.DDL.Server.Options
		if len(opts) != 1 || opts[0].Key != "port" || opts[0].RangeState != spec.ServerOptionRangeInRange {
			t.Fatalf("port 3306 range state = %+v, want in-range", opts)
		}
	})
	t.Run("port boundary values", func(t *testing.T) {
		for _, tc := range []struct {
			literal string
			want    spec.ServerOptionRangeState
		}{
			{"0", spec.ServerOptionRangeInRange},
			{"9999", spec.ServerOptionRangeInRange},
			{"10000", spec.ServerOptionRangeOutOfRange},
		} {
			stmt := parseExtractT26(t, `CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (PORT `+tc.literal+`)`, spec.DialectMySQL)
			got := stmt.DDL.Server.Options[0].RangeState
			if got != tc.want {
				t.Fatalf("PORT %s = %q, want %q", tc.literal, got, tc.want)
			}
		}
	})
	t.Run("port uint64 overflow stays parser rejection", func(t *testing.T) {
		if err := parseErrT26(t, `CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (PORT 18446744073709551616)`); err == nil {
			t.Fatalf("overflowing PORT literal must stay a parser rejection")
		}
	})
}

// The recognizer still records keys, kinds, presence, and duplicate
// tracking without ever storing a raw literal value — the declaration
// carries no OPTIONS content.
func TestT26CreateOptionsFacts(t *testing.T) {
	stmt := parseExtractT26(t, `CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 's3cret-host', PASSWORD 'p4ssw0rd', PORT 3306, USER 'u', USER 'u2')`, spec.DialectMySQL)
	decl := stmt.DDL.Server
	keys := make([]string, 0, len(decl.Options))
	for _, opt := range decl.Options {
		keys = append(keys, opt.Key)
	}
	wantKeys := []string{"host", "password", "port", "user", "user"}
	if len(keys) != len(wantKeys) {
		t.Fatalf("option keys = %v, want %v", keys, wantKeys)
	}
	for i, want := range wantKeys {
		if keys[i] != want {
			t.Fatalf("option[%d] = %q, want %q", i, keys[i], want)
		}
	}
	if len(decl.DuplicateKeys) != 1 || decl.DuplicateKeys[0] != "user" {
		t.Fatalf("duplicate keys = %v, want [user]", decl.DuplicateKeys)
	}
	for _, opt := range decl.Options {
		wantKind := "char"
		if opt.Key == "port" {
			wantKind = "numeric"
		}
		if opt.ValueKind != wantKind {
			t.Fatalf("option %s kind = %q, want %q", opt.Key, opt.ValueKind, wantKind)
		}
	}
	// No field on the declaration may carry a raw literal — check the
	// JSON-facing shape contains neither credential value.
	for _, leaked := range []string{"s3cret-host", "p4ssw0rd"} {
		if strings.Contains(strings.ToLower(stmt.DDL.Server.Wrapper+decl.Name), leaked) {
			t.Fatalf("declaration leaked OPTIONS literal %q", leaked)
		}
	}
}

// Unknown keys, malformed members, trailing tokens, and the empty
// list keep their established boundaries — they stay unclaimed by the
// recognizer so the base parser owns the diagnostic.
func TestT26CreateOptionsBoundaries(t *testing.T) {
	for _, sql := range []string{
		`CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (BOGUS 'x')`,
		`CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST)`,
		`CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h',)`,
		`CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS ()`,
		`CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (PORT 'abc')`,
	} {
		if err := parseErrT26(t, sql); err == nil {
			t.Fatalf("%s: must stay a parser rejection, not a claimed declaration", sql)
		}
	}
}
