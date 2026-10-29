// Package tidb verifies the T15 table-option / online-DDL extraction
// facts: authored ALGORITHM/LOCK values, ORDER BY member order,
// DEFAULT-vs-CONVERT charset distinction, and the bounded long-tail
// option vocabulary (#94).
package tidbparser

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// The authored ALGORITHM and LOCK clauses land on their own alter specs
// with the authored values lower-cased.
func TestExtractorT15AlgorithmLockValues(t *testing.T) {
	t.Parallel()
	statement := extractOne(t, spec.DialectMySQL,
		"ALTER TABLE t ADD COLUMN c INT, ALGORITHM=COPY, LOCK=EXCLUSIVE")
	if len(statement.DDL.Alter) != 3 {
		t.Fatalf("expected three alter actions, got %#v", statement.DDL.Alter)
	}
	if statement.DDL.Alter[0].Action != "add_columns" {
		t.Fatalf("first action = %q, want add_columns", statement.DDL.Alter[0].Action)
	}
	if got := statement.DDL.Alter[1]; got.Action != "algorithm" || got.Options["algorithm"] != "copy" {
		t.Fatalf("algorithm spec = %#v", got)
	}
	if got := statement.DDL.Alter[2]; got.Action != "lock" || got.Options["lock"] != "exclusive" {
		t.Fatalf("lock spec = %#v", got)
	}
}

// ORDER BY members keep authored order and direction — the structure is a
// declaration fact, not a validated member binding.
func TestExtractorT15OrderByMembers(t *testing.T) {
	t.Parallel()
	statement := extractOne(t, spec.DialectMySQL,
		"ALTER TABLE t ORDER BY a DESC, b, C ASC")
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %#v", statement.DDL.Alter)
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "order_by_columns" {
		t.Fatalf("action = %q, want order_by_columns", alter.Action)
	}
	want := []spec.OrderByItem{{Column: "a", Desc: true}, {Column: "b"}, {Column: "c"}}
	if len(alter.OrderBy) != len(want) {
		t.Fatalf("OrderBy = %#v, want %#v", alter.OrderBy, want)
	}
	for i := range want {
		if alter.OrderBy[i] != want[i] {
			t.Fatalf("OrderBy[%d] = %#v, want %#v", i, alter.OrderBy[i], want[i])
		}
	}
}

// collectTableOptions merges every table_option spec's option map — the
// parser emits one spec per authored `name=value` clause.
func collectTableOptions(t *testing.T, sql string) map[string]string {
	t.Helper()
	statement := extractOne(t, spec.DialectMySQL, sql)
	merged := map[string]string{}
	for _, alter := range statement.DDL.Alter {
		if alter.Action != "table_option" {
			t.Fatalf("unexpected action %q in %#v", alter.Action, statement.DDL.Alter)
		}
		for key, value := range alter.Options {
			merged[key] = value
		}
	}
	return merged
}

// DEFAULT CHARACTER SET and CONVERT TO CHARACTER SET are distinct facts —
// default-setting vs conversion semantics never conflate.
func TestExtractorT15DefaultVsConvertCharset(t *testing.T) {
	t.Parallel()
	def := collectTableOptions(t,
		"ALTER TABLE t DEFAULT CHARACTER SET utf8mb4, COLLATE utf8mb4_bin")
	if def["charset"] != "utf8mb4" || def["collate"] != "utf8mb4_bin" {
		t.Fatalf("default charset options = %#v", def)
	}
	if _, ok := def["convert_to_charset"]; ok {
		t.Fatalf("DEFAULT charset must not produce convert_to_charset: %#v", def)
	}

	conv := collectTableOptions(t,
		"ALTER TABLE t CONVERT TO CHARACTER SET latin1 COLLATE latin1_bin")
	if conv["convert_to_charset"] != "latin1" || conv["convert_to_collate"] != "latin1_bin" {
		t.Fatalf("convert options = %#v", conv)
	}
	if _, ok := conv["charset"]; ok {
		t.Fatalf("CONVERT must not produce the charset key: %#v", conv)
	}
	if _, ok := conv["collate"]; ok {
		t.Fatalf("CONVERT COLLATE must not produce the collate key: %#v", conv)
	}
}

// The bounded long-tail option vocabulary extracts with authored values —
// numeric, string, and keyword forms alike.
func TestExtractorT15LongTailOptions(t *testing.T) {
	t.Parallel()
	opts := collectTableOptions(t,
		"ALTER TABLE t STATS_PERSISTENT=1, AVG_ROW_LENGTH=128, CHECKSUM=1, MAX_ROWS=1000, MIN_ROWS=10, KEY_BLOCK_SIZE=4, COMPRESSION='zlib', ENCRYPTION='Y', INSERT_METHOD=LAST, TABLESPACE ts1")
	want := map[string]string{
		"stats_persistent": "declared", "avg_row_length": "128", "checksum": "1",
		"max_rows": "1000", "min_rows": "10", "key_block_size": "4",
		"compression": "zlib", "encryption": "Y", "insert_method": "last",
		"tablespace": "ts1",
	}
	for key, value := range want {
		if opts[key] != value {
			t.Fatalf("option %q = %q, want %q (all: %#v)", key, opts[key], value, opts)
		}
	}
}

// UNION member names extract in authored order — the declaration is a
// bounded fact, not a member binding.
func TestExtractorT15UnionMembers(t *testing.T) {
	t.Parallel()
	opts := collectTableOptions(t, "ALTER TABLE t UNION=(a, b, c)")
	if opts["union"] != "a,b,c" {
		t.Fatalf("union members = %q, want a,b,c", opts["union"])
	}
}

// PACK_KEYS / DELAY_KEY_WRITE / storage media / data-directory keys are
// extracted, not silently dropped; SECONDARY_ENGINE=NULL records its reset.
func TestExtractorT15MyISAMVocabulary(t *testing.T) {
	t.Parallel()
	opts := collectTableOptions(t,
		"ALTER TABLE t PACK_KEYS=1, DELAY_KEY_WRITE=1, DATA DIRECTORY='/x', INDEX DIRECTORY='/y', CONNECTION='f', PASSWORD='s', STORAGE MEMORY, SECONDARY_ENGINE=InnoDB")
	want := map[string]string{
		"pack_keys": "declared", "delay_key_write": "1",
		"data_directory": "/x", "index_directory": "/y",
		"connection": "f", "password": "s",
		"storage_media": "memory", "secondary_engine": "InnoDB",
	}
	for key, value := range want {
		if opts[key] != value {
			t.Fatalf("option %q = %q, want %q (all: %#v)", key, opts[key], value, opts)
		}
	}
}
