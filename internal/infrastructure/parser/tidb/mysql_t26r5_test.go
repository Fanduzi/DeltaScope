// Package tidbparser verifies the T26-R5 corrections (issue #105):
// quote closure is decided by the same lexical scan the tokenizer
// runs — backslash escapes and doubled quotes inside strings, doubled
// backticks inside quoted identifiers — so `'x\'` (escaped final
// quote) stays unclaimed while `'x\''` (escape then real close) lands.
// The spelling-length fact records the unquoted spelling's rune count
// and never claims the decoded MySQL name length.
package tidbparser

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// R5-01: closure follows the real lexical contract — backslash escapes
// (including consecutive backslashes) and doubled quotes for strings,
// doubled backticks for quoted identifiers. Named subtests keep one
// red failure from masking the rest.
func TestT26R5QuoteClosure(t *testing.T) {
	t.Parallel()
	t.Run("escaped_final_quote_unclosed", func(t *testing.T) {
		// 'x\' — the final quote is escaped, the literal is unclosed.
		if err := parseErrT26(t, `DROP SERVER 'x\'`); err == nil {
			t.Fatalf("escaped final quote claimed as closed")
		}
	})
	t.Run("backslash_pair_then_escaped_quote_unclosed", func(t *testing.T) {
		// 'x\\\' — \\ pair then \' escape; still unclosed.
		if err := parseErrT26(t, `DROP SERVER 'x\\\'`); err == nil {
			t.Fatalf("consecutive-backslash escaped quote claimed as closed")
		}
	})
	t.Run("dquote_escaped_final_unclosed", func(t *testing.T) {
		if err := parseErrT26(t, `DROP SERVER "x\"`); err == nil {
			t.Fatalf("double-quoted escaped final quote claimed as closed")
		}
	})
	t.Run("trailing_doubled_quote_unclosed", func(t *testing.T) {
		// 'x'' — the trailing pair is an escape, not a close.
		if err := parseErrT26(t, `DROP SERVER 'x''`); err == nil {
			t.Fatalf("doubled-quote escape at end claimed as close")
		}
	})
	t.Run("srs_last_string_attr_unclosed", func(t *testing.T) {
		if err := parseErrT26(t, `CREATE SPATIAL REFERENCE SYSTEM 1 NAME 'a' DESCRIPTION 'x\'`); err == nil {
			t.Fatalf("SRS trailing unclosed attribute claimed")
		}
	})
	t.Run("srs_name_attr_unclosed", func(t *testing.T) {
		if err := parseErrT26(t, `CREATE SPATIAL REFERENCE SYSTEM 1 NAME 'a\'`); err == nil {
			t.Fatalf("SRS NAME unclosed attribute claimed")
		}
	})
	t.Run("options_value_unclosed", func(t *testing.T) {
		if err := parseErrT26(t, `ALTER SERVER s OPTIONS (HOST 'x\')`); err == nil {
			t.Fatalf("OPTIONS unclosed value claimed")
		}
	})
	t.Run("escape_then_real_close", func(t *testing.T) {
		// 'x\'' — \' escapes, the final ' is a real close.
		stmt := parseExtractT26(t, `DROP SERVER 'x\''`, spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil {
			t.Fatalf("escaped quote plus real close not recognized")
		}
	})
	t.Run("backslash_pair_then_close", func(t *testing.T) {
		// 'x\\' — \\ pair, then a real close.
		stmt := parseExtractT26(t, `DROP SERVER 'x\\'`, spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil {
			t.Fatalf("backslash pair plus close not recognized")
		}
	})
	t.Run("doubled_quote_inside_close", func(t *testing.T) {
		// 'x''y' — '' escape inside, real close at end.
		stmt := parseExtractT26(t, `DROP SERVER 'x''y'`, spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil {
			t.Fatalf("doubled-quote interior plus close not recognized")
		}
	})
	t.Run("doubled_backtick_inside_ident", func(t *testing.T) {
		// `x``y` — `` escape inside a quoted identifier, real close.
		stmt := parseExtractT26(t, "DROP SERVER `x``y`", spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil || stmt.DDL.Server.Name != "x`y" {
			t.Fatalf("doubled-backtick ident: %+v", stmt.DDL)
		}
	})
	t.Run("backtick_backslash_is_literal", func(t *testing.T) {
		// `x\` — inside backticks backslash is literal, so the
		// backtick still closes (tokenizer's own contract).
		stmt := parseExtractT26(t, "DROP SERVER `x\\`", spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil {
			t.Fatalf("backtick ident with literal backslash not recognized")
		}
	})
	t.Run("options_empty_string_value", func(t *testing.T) {
		stmt := parseExtractT26(t, `CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (PASSWORD '')`, spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil {
			t.Fatalf("legal empty option string not recognized")
		}
	})
	t.Run("options_escaped_quote_value", func(t *testing.T) {
		stmt := parseExtractT26(t, `ALTER SERVER s OPTIONS (HOST 'x\'y')`, spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil {
			t.Fatalf("escaped-quote option value not recognized")
		}
	})
}

// R5-02: the name fact is the unquoted spelling's rune count, never
// the decoded valid-name length — '...63 digits...\4' spells 65 runes
// while its decoded name is 64; the fact must not read as a proven
// over-limit violation.
func TestT26R5NameSpellingFact(t *testing.T) {
	t.Parallel()
	stmt := parseExtractT26(t, "DROP SERVER '"+strings.Repeat("0", 63)+`\4'`, spec.DialectMySQL)
	decl := stmt.DDL.Server
	if decl == nil {
		t.Fatalf("escaped-digit name not recognized")
	}
	if decl.NameSpellingRuneCount != 65 {
		t.Fatalf("spelling runes = %d, want 65 (63 digits + \\4 unescaped pair)", decl.NameSpellingRuneCount)
	}
	if !strings.HasSuffix(decl.Name, `\4`) {
		t.Fatalf("spelling should keep the undecoded \\4 pair, got %q", decl.Name[len(decl.Name)-4:])
	}
}
