// Package tidbparser verifies the T26-R6 correction (issue #105): a
// block comment that never finds its closing `*/` must not be silently
// dropped by the tokenizer and then claimed by a T26 recognizer as a
// complete input. The lexical scan now keeps the closure result, and
// every T26 claim path — SERVER, SRS, ROTATE — consumes it. A `/*`
// inside a string literal is never a comment start, so legal values
// containing the pair still claim. Named subtests keep one red
// failure from masking the rest.
package tidbparser

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// R6-01: unclosed block comments make the whole-input claim fail at
// the recognizer so the statement reaches the base parser's fixed
// diagnostic path; closed comments are inert trivia and string
// literals can legally contain the two-byte pair.
func TestT26R6BlockCommentClosure(t *testing.T) {
	t.Parallel()

	// (a) Statement trailing an unclosed /* must stay unclaimed — the
	// recognizer declines and the base parser produces its diagnostic.
	t.Run("server_drop_unclosed", func(t *testing.T) {
		if err := parseErrT26(t, "DROP SERVER s /* t26-comment-marker"); err == nil {
			t.Fatalf("unclosed block comment claimed as complete DROP SERVER")
		}
	})
	t.Run("server_alter_unclosed", func(t *testing.T) {
		if err := parseErrT26(t, "ALTER SERVER s OPTIONS (USER 'u') /* t26-comment-marker"); err == nil {
			t.Fatalf("unclosed block comment claimed as complete ALTER SERVER")
		}
	})
	t.Run("server_create_unclosed", func(t *testing.T) {
		if err := parseErrT26(t, "CREATE SERVER s FOREIGN DATA WRAPPER w OPTIONS (USER 'u') /* t26-comment-marker"); err == nil {
			t.Fatalf("unclosed block comment claimed as complete CREATE SERVER")
		}
	})
	t.Run("server_mid_unclosed", func(t *testing.T) {
		if err := parseErrT26(t, "ALTER SERVER s /* t26-comment-marker"); err == nil {
			t.Fatalf("mid-statement unclosed block comment claimed")
		}
	})
	t.Run("srs_create_unclosed", func(t *testing.T) {
		if err := parseErrT26(t, "CREATE SPATIAL REFERENCE SYSTEM 4120 NAME 'a' DEFINITION 'd' /* t26-comment-marker"); err == nil {
			t.Fatalf("unclosed block comment claimed as complete CREATE SRS")
		}
	})
	t.Run("srs_drop_unclosed", func(t *testing.T) {
		if err := parseErrT26(t, "DROP SPATIAL REFERENCE SYSTEM 4120 /* t26-comment-marker"); err == nil {
			t.Fatalf("unclosed block comment claimed as complete DROP SRS")
		}
	})
	t.Run("rotate_unclosed", func(t *testing.T) {
		if err := parseErrT26(t, "ALTER INSTANCE ROTATE INNODB MASTER KEY /* t26-comment-marker"); err == nil {
			t.Fatalf("unclosed block comment claimed as complete ROTATE")
		}
	})

	// (b) The matching closed /* ... */ keeps each claim legal.
	t.Run("server_drop_closed", func(t *testing.T) {
		stmt := parseExtractT26(t, "DROP SERVER s /* ok */", spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil {
			t.Fatalf("closed comment broke a legal DROP SERVER claim")
		}
	})
	t.Run("server_alter_closed", func(t *testing.T) {
		stmt := parseExtractT26(t, "ALTER SERVER s OPTIONS (USER 'u') /* ok */", spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil {
			t.Fatalf("closed comment broke a legal ALTER SERVER claim")
		}
	})
	t.Run("server_create_closed", func(t *testing.T) {
		stmt := parseExtractT26(t, "CREATE SERVER s FOREIGN DATA WRAPPER w OPTIONS (USER 'u') /* ok */", spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil {
			t.Fatalf("closed comment broke a legal CREATE SERVER claim")
		}
	})
	t.Run("srs_create_closed", func(t *testing.T) {
		stmt := parseExtractT26(t, "CREATE SPATIAL REFERENCE SYSTEM 4120 NAME 'a' DEFINITION 'd' /* ok */", spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.SpatialReference == nil {
			t.Fatalf("closed comment broke a legal CREATE SRS claim")
		}
	})
	t.Run("srs_drop_closed", func(t *testing.T) {
		stmt := parseExtractT26(t, "DROP SPATIAL REFERENCE SYSTEM 4120 /* ok */", spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.SpatialReference == nil {
			t.Fatalf("closed comment broke a legal DROP SRS claim")
		}
	})
	t.Run("rotate_closed", func(t *testing.T) {
		if err := parseErrT26(t, "ALTER INSTANCE ROTATE INNODB MASTER KEY /* ok */"); err != nil {
			t.Fatalf("closed comment broke a legal ROTATE claim: %v", err)
		}
	})

	// (c) `/*` inside a string literal is value text, never a comment
	// start — the claim must still land.
	t.Run("server_options_string_holds_marker_pair", func(t *testing.T) {
		stmt := parseExtractT26(t, "CREATE SERVER s FOREIGN DATA WRAPPER w OPTIONS (HOST 'h/*t*/p', USER 'u')", spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.Server == nil {
			t.Fatalf("/* inside string literal broke a legal CREATE SERVER claim")
		}
	})
	t.Run("srs_attr_string_holds_marker_pair", func(t *testing.T) {
		stmt := parseExtractT26(t, "CREATE SPATIAL REFERENCE SYSTEM 4120 NAME 'a' DEFINITION 'd' DESCRIPTION 'x /*m*/ y'", spec.DialectMySQL)
		if stmt.DDL == nil || stmt.DDL.SpatialReference == nil {
			t.Fatalf("/* inside SRS string attribute broke a legal claim")
		}
	})
}
