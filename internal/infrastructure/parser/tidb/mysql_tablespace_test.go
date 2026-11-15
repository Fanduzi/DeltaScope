// Package tidbparser verifies the T25/#104 MySQL tablespace recognizer through
// the public parse-extract seam: the supported CREATE/ALTER/DROP family
// members land as typed DDL declarations, the strict grammar rejects the
// authored shapes MySQL itself rejects (IF NOT EXISTS on CREATE
// TABLESPACE, UNDO TABLESPACE without ADD DATAFILE), and malformed or
// ambiguous input falls back to the base parser rather than being
// claimed. File paths stay opaque — only clause counts are facts.
package tidbparser

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func parseExtractT25(t *testing.T, sql string, dialect spec.Dialect) spec.Statement {
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

func parseErrT25(t *testing.T, sql string) error {
	t.Helper()
	parser := New()
	_, err := parser.Parse(context.Background(), sql)
	return err
}

// The supported family members land as typed DDL with the authored
// declaration facts — kind, name, engine, file counts — and nothing more.
func TestT25TablespaceRecognizedForms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		sql       string
		operation spec.DDLOperation
		kind      string
		name      string
		alter     string
		engine    string
		fileCount int
		newName   string
		ifExists  bool
	}{
		{"CREATE TABLESPACE ts1 ADD DATAFILE 'a.ibd' ENGINE=INNODB",
			spec.DDLOperationCreateTablespace, spec.TablespaceKindGeneral, "ts1", "", "innodb", 1, "", false},
		// Memberless general CREATE is grammar-valid and executes on
		// InnoDB from 8.0.14 — the server generates the datafile itself.
		{"CREATE TABLESPACE ts1",
			spec.DDLOperationCreateTablespace, spec.TablespaceKindGeneral, "ts1", "", "", 0, "", false},
		{"CREATE TABLESPACE ts1 ADD DATAFILE 'a.ibd' ENGINE INNODB",
			spec.DDLOperationCreateTablespace, spec.TablespaceKindGeneral, "ts1", "", "innodb", 1, "", false},
		{"CREATE UNDO TABLESPACE u1 ADD DATAFILE 'u.ibu'",
			spec.DDLOperationCreateTablespace, spec.TablespaceKindUndo, "u1", "", "", 1, "", false},
		{"CREATE LOGFILE GROUP lg1 ADD UNDOFILE 'u.und' INITIAL_SIZE 8M ENGINE=NDB",
			spec.DDLOperationCreateLogfileGroup, spec.TablespaceKindLogfileGroup, "lg1", "", "ndb", 1, "", false},
		{"ALTER TABLESPACE ts1 ADD DATAFILE 'b.ibd' ENGINE=INNODB",
			spec.DDLOperationAlterTablespace, spec.TablespaceKindGeneral, "ts1", "add_datafile", "innodb", 1, "", false},
		{"ALTER TABLESPACE ts1 DROP DATAFILE 'b.ibd'",
			spec.DDLOperationAlterTablespace, spec.TablespaceKindGeneral, "ts1", "drop_datafile", "", 1, "", false},
		{"ALTER TABLESPACE ts1 RENAME TO ts2",
			spec.DDLOperationAlterTablespace, spec.TablespaceKindGeneral, "ts1", "rename", "", 0, "ts2", false},
		{"ALTER TABLESPACE ts1 ENCRYPTION='Y'",
			spec.DDLOperationAlterTablespace, spec.TablespaceKindGeneral, "ts1", "encryption", "", 0, "", false},
		// ENGINE may lead the member list — `ALTER TABLESPACE t ENGINE
		// [=] engine` is a documented single-member form.
		{"ALTER TABLESPACE ts1 ENGINE=INNODB",
			spec.DDLOperationAlterTablespace, spec.TablespaceKindGeneral, "ts1", "engine", "innodb", 0, "", false},
		{"ALTER TABLESPACE ts1 ENGINE NDB",
			spec.DDLOperationAlterTablespace, spec.TablespaceKindGeneral, "ts1", "engine", "ndb", 0, "", false},
		// Scalar members may lead the member list (R7-N03): they carry
		// no AlterAction — their value lands in Options.
		{"ALTER TABLESPACE ts1 AUTOEXTEND_SIZE=8M",
			spec.DDLOperationAlterTablespace, spec.TablespaceKindGeneral, "ts1", "", "", 0, "", false},
		{"ALTER TABLESPACE ts1 ENGINE_ATTRIBUTE='{\"k\":1}'",
			spec.DDLOperationAlterTablespace, spec.TablespaceKindGeneral, "ts1", "", "", 0, "", false},
		{"DROP TABLESPACE ts1 ENGINE=INNODB",
			spec.DDLOperationDropTablespace, spec.TablespaceKindGeneral, "ts1", "", "innodb", 0, "", false},
		{"DROP UNDO TABLESPACE u1",
			spec.DDLOperationDropTablespace, spec.TablespaceKindUndo, "u1", "", "", 0, "", false},
		{"DROP LOGFILE GROUP lg1 ENGINE=NDB",
			spec.DDLOperationDropLogfileGroup, spec.TablespaceKindLogfileGroup, "lg1", "", "ndb", 0, "", false},
	}
	for _, tc := range cases {
		statement := parseExtractT25(t, tc.sql, spec.DialectMySQL)
		if statement.Kind != spec.KindDDL || statement.DDL == nil {
			t.Fatalf("%q: kind=%q ddl=%v, want DDL", tc.sql, statement.Kind, statement.DDL)
		}
		if statement.DDL.Operation != tc.operation {
			t.Fatalf("%q: operation=%q, want %q", tc.sql, statement.DDL.Operation, tc.operation)
		}
		decl := statement.DDL.Tablespace
		if decl == nil {
			t.Fatalf("%q: Tablespace declaration missing", tc.sql)
		}
		if decl.Kind != tc.kind || decl.Name != tc.name || decl.AlterAction != tc.alter ||
			decl.Engine != tc.engine || decl.FileCount != tc.fileCount ||
			decl.NewName != tc.newName || decl.IfExists != tc.ifExists {
			t.Fatalf("%q: decl=%+v, want kind=%q name=%q alter=%q engine=%q files=%d new=%q ifexists=%t",
				tc.sql, decl, tc.kind, tc.name, tc.alter, tc.engine, tc.fileCount, tc.newName, tc.ifExists)
		}
	}
}

// R7-N03: a scalar member leading the member list still lands in
// Options — the capability floors key off those entries, so the
// extraction must place the value, not merely stop refusing.
func TestT25StandaloneScalarMemberLandsOptions(t *testing.T) {
	t.Parallel()
	cases := []struct{ sql, key, want string }{
		{"ALTER TABLESPACE ts1 AUTOEXTEND_SIZE=8M", "autoextend_size", "8m"},
		{"ALTER TABLESPACE ts1 AUTOEXTEND_SIZE 8M", "autoextend_size", "8m"},
		{"ALTER TABLESPACE ts1 ENGINE_ATTRIBUTE='{\"k\":1}'", "engine_attribute", "{\"k\":1}"},
	}
	for _, tc := range cases {
		statement := parseExtractT25(t, tc.sql, spec.DialectMySQL)
		decl := statement.DDL.Tablespace
		if decl == nil || decl.Options[tc.key] != tc.want {
			t.Fatalf("%q: options=%v, want %s=%q", tc.sql, decl.Options, tc.key, tc.want)
		}
	}
}

// ALTER UNDO TABLESPACE ... SET ACTIVE|INACTIVE is recognized as a typed
// declaration so coverage routes it to the unaudited boundary rather
// than a parser diagnostic.
func TestT25AlterUndoSetStateRecognized(t *testing.T) {
	t.Parallel()
	cases := []struct {
		sql      string
		setState string
		engine   string
	}{
		{"ALTER UNDO TABLESPACE u1 SET ACTIVE", "active", ""},
		{"ALTER UNDO TABLESPACE u1 SET INACTIVE", "inactive", ""},
		{"ALTER UNDO TABLESPACE u1 SET ACTIVE ENGINE=INNODB", "active", "innodb"},
		{"ALTER UNDO TABLESPACE u1 SET INACTIVE ENGINE INNODB", "inactive", "innodb"},
	}
	for _, tc := range cases {
		statement := parseExtractT25(t, tc.sql, spec.DialectMySQL)
		decl := statement.DDL.Tablespace
		if decl == nil {
			t.Fatalf("%q: expected tablespace declaration", tc.sql)
		}
		if decl.Kind != spec.TablespaceKindUndo || decl.AlterAction != "set_state" ||
			decl.Engine != tc.engine || decl.Options["set_state"] != tc.setState {
			t.Fatalf("%q: decl=%+v, want undo set_state=%q engine=%q", tc.sql, decl, tc.setState, tc.engine)
		}
	}
}

// Strict-grammar rejections: shapes MySQL itself refuses must not be
// claimed by the recognizer. Some fall back to the base parser (which may
// reject them too) — what matters is they never produce a declaration.
func TestT25TablespaceRejectedShapes(t *testing.T) {
	t.Parallel()
	rejected := []string{
		// CREATE TABLESPACE has no IF NOT EXISTS in MySQL.
		"CREATE TABLESPACE IF NOT EXISTS ts1 ADD DATAFILE 'a.ibd'",
		// UNDO TABLESPACE requires ADD DATAFILE.
		"CREATE UNDO TABLESPACE u1",
		"CREATE UNDO TABLESPACE u1 ENGINE=INNODB",
		// LOGFILE GROUP requires ADD UNDOFILE.
		"CREATE LOGFILE GROUP lg1",
		// No members at all.
		"ALTER TABLESPACE ts1",
		"ALTER TABLESPACE ts1 ENGINE",
		"DROP TABLESPACE",
		// DROP TABLESPACE has no IF EXISTS member in the MySQL grammar.
		"DROP TABLESPACE IF EXISTS ts1",
		// Trailing junk after a complete statement.
		"CREATE TABLESPACE ts1 ADD DATAFILE 'a.ibd' WAT",
		"DROP TABLESPACE ts1 EXTRA JUNK",
		// The grammar carries at most one file clause.
		"CREATE TABLESPACE ts1 ADD DATAFILE 'a.ibd' ADD DATAFILE 'b.ibd'",
		"CREATE UNDO TABLESPACE u1 ADD DATAFILE 'a.ibu' ADD DATAFILE 'b.ibu'",
		// Each kind takes its documented file keyword: LOGFILE GROUP
		// takes UNDOFILE, general and undo take DATAFILE.
		"CREATE TABLESPACE ts1 ADD UNDOFILE 'a.ibd'",
		"CREATE UNDO TABLESPACE u1 ADD UNDOFILE 'a.ibu'",
		"CREATE LOGFILE GROUP lg1 ADD DATAFILE 'a.dat'",
		"ALTER TABLESPACE ts1 ADD UNDOFILE 'a.ibd'",
		// ALTER UNDO TABLESPACE ... SET carries no trailing member beyond
		// the optional ENGINE clause — everything else is real garbage.
		"ALTER UNDO TABLESPACE u1 SET ACTIVE EXTRA",
		"ALTER UNDO TABLESPACE u1 SET ACTIVE ENGINE",
		"ALTER UNDO TABLESPACE u1 SET ACTIVE RENAME TO u2",
		"ALTER UNDO TABLESPACE u1 SET",
		"ALTER UNDO TABLESPACE u1 ADD DATAFILE 'a.ibu'",
	}
	for _, sql := range rejected {
		parser := New()
		result, err := parser.Parse(context.Background(), sql)
		if err != nil {
			continue // base parser refusing is fine — it was never claimed
		}
		// If the base parser accepted it, extraction must not produce a
		// tablespace declaration — the recognizer never claimed it.
		wrapped := WrapStatements(result.Statements, result.Warnings)
		for _, w := range wrapped {
			statement, eerr := w.Extractor.Extract(spec.DialectMySQL, w.RawSQL)
			if eerr != nil {
				continue
			}
			if statement.DDL != nil && statement.DDL.Tablespace != nil {
				t.Fatalf("%q must not produce a tablespace declaration, got %+v", sql, statement.DDL.Tablespace)
			}
		}
	}
}
