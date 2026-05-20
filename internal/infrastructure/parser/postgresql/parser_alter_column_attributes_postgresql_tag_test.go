//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractAlterSetStatisticsValue(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users ALTER COLUMN email SET STATISTICS 100")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported: %v", stmt.Unsupported)
	}
	if stmt.DDL == nil || stmt.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table, got %#v", stmt.DDL)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "set_statistics" {
		t.Fatalf("expected action set_statistics, got %q", alter.Action)
	}
	if alter.Name != "email" {
		t.Fatalf("expected name email, got %q", alter.Name)
	}
	if alter.Options["column"] != "email" {
		t.Fatalf("expected option column=email, got %q", alter.Options["column"])
	}
	if alter.Options["statistics_target_kind"] != "value" {
		t.Fatalf("expected statistics_target_kind=value, got %q", alter.Options["statistics_target_kind"])
	}
	if alter.Options["has_statistics_target"] != "true" {
		t.Fatalf("expected has_statistics_target=true, got %q", alter.Options["has_statistics_target"])
	}
	// No-leak: numeric target must not appear
	for _, forbidden := range []string{"100", "statistics_target"} {
		if _, ok := alter.Options[forbidden]; ok {
			t.Fatalf("forbidden key %q present in options", forbidden)
		}
	}
}

func TestExtractAlterSetStatisticsDefault(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users ALTER COLUMN email SET STATISTICS DEFAULT")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported: %v", stmt.Unsupported)
	}
	if len(stmt.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(stmt.DDL.Alter))
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "set_statistics" {
		t.Fatalf("expected action set_statistics, got %q", alter.Action)
	}
	if alter.Name != "email" {
		t.Fatalf("expected name email, got %q", alter.Name)
	}
	if alter.Options["statistics_target_kind"] != "default" {
		t.Fatalf("expected statistics_target_kind=default, got %q", alter.Options["statistics_target_kind"])
	}
	if alter.Options["has_statistics_target"] != "true" {
		t.Fatalf("expected has_statistics_target=true, got %q", alter.Options["has_statistics_target"])
	}
}

func TestExtractAlterSetColumnOptions(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users ALTER COLUMN email SET (n_distinct = -1)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported: %v", stmt.Unsupported)
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "set_column_options" {
		t.Fatalf("expected action set_column_options, got %q", alter.Action)
	}
	if alter.Name != "email" {
		t.Fatalf("expected name email, got %q", alter.Name)
	}
	if alter.Options["column"] != "email" {
		t.Fatalf("expected option column=email, got %q", alter.Options["column"])
	}
	if alter.Options["option_count"] != "1" {
		t.Fatalf("expected option_count=1, got %q", alter.Options["option_count"])
	}
	if alter.Options["has_column_options"] != "true" {
		t.Fatalf("expected has_column_options=true, got %q", alter.Options["has_column_options"])
	}
	// No-leak: option names and values must not appear
	for _, forbidden := range []string{"n_distinct", "-1", "option_names", "option_values"} {
		if _, ok := alter.Options[forbidden]; ok {
			t.Fatalf("forbidden key %q present in options", forbidden)
		}
	}
}

func TestExtractAlterResetColumnOptions(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users ALTER COLUMN email RESET (n_distinct)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported: %v", stmt.Unsupported)
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "reset_column_options" {
		t.Fatalf("expected action reset_column_options, got %q", alter.Action)
	}
	if alter.Name != "email" {
		t.Fatalf("expected name email, got %q", alter.Name)
	}
	if alter.Options["reset_count"] != "1" {
		t.Fatalf("expected reset_count=1, got %q", alter.Options["reset_count"])
	}
	if alter.Options["has_column_options"] != "true" {
		t.Fatalf("expected has_column_options=true, got %q", alter.Options["has_column_options"])
	}
	// No-leak: option names must not appear
	for _, forbidden := range []string{"n_distinct", "option_names"} {
		if _, ok := alter.Options[forbidden]; ok {
			t.Fatalf("forbidden key %q present in options", forbidden)
		}
	}
}

func TestExtractAlterSetStorageExternal(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users ALTER COLUMN bio SET STORAGE EXTERNAL")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported: %v", stmt.Unsupported)
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "set_storage" {
		t.Fatalf("expected action set_storage, got %q", alter.Action)
	}
	if alter.Name != "bio" {
		t.Fatalf("expected name bio, got %q", alter.Name)
	}
	if alter.Options["column"] != "bio" {
		t.Fatalf("expected option column=bio, got %q", alter.Options["column"])
	}
	if alter.Options["storage_kind"] != "external" {
		t.Fatalf("expected storage_kind=external, got %q", alter.Options["storage_kind"])
	}
	if alter.Options["has_storage_setting"] != "true" {
		t.Fatalf("expected has_storage_setting=true, got %q", alter.Options["has_storage_setting"])
	}
}

func TestExtractAlterSetStorageDefault(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users ALTER COLUMN bio SET STORAGE DEFAULT")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported: %v", stmt.Unsupported)
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "set_storage" {
		t.Fatalf("expected action set_storage, got %q", alter.Action)
	}
	if alter.Options["storage_kind"] != "default" {
		t.Fatalf("expected storage_kind=default, got %q", alter.Options["storage_kind"])
	}
	if alter.Options["has_storage_setting"] != "true" {
		t.Fatalf("expected has_storage_setting=true, got %q", alter.Options["has_storage_setting"])
	}
}

func TestExtractAlterSetCompression(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users ALTER COLUMN bio SET COMPRESSION lz4")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported: %v", stmt.Unsupported)
	}
	alter := stmt.DDL.Alter[0]
	if alter.Action != "set_compression" {
		t.Fatalf("expected action set_compression, got %q", alter.Action)
	}
	if alter.Name != "bio" {
		t.Fatalf("expected name bio, got %q", alter.Name)
	}
	if alter.Options["column"] != "bio" {
		t.Fatalf("expected option column=bio, got %q", alter.Options["column"])
	}
	if alter.Options["has_compression"] != "true" {
		t.Fatalf("expected has_compression=true, got %q", alter.Options["has_compression"])
	}
	// No-leak: compression method must not appear
	for _, forbidden := range []string{"lz4", "pglz", "compression_method"} {
		if _, ok := alter.Options[forbidden]; ok {
			t.Fatalf("forbidden key %q present in options", forbidden)
		}
	}
}

func TestExtractAlterMultiOptionSetColumnOptions(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users ALTER COLUMN email SET (n_distinct = -0.5, n_distinct_inherited = 0.1)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported: %v", stmt.Unsupported)
	}
	alter := stmt.DDL.Alter[0]
	if alter.Options["option_count"] != "2" {
		t.Fatalf("expected option_count=2, got %q", alter.Options["option_count"])
	}
}

func TestExtractAlterMultiOptionResetColumnOptions(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users ALTER COLUMN email RESET (n_distinct, n_distinct_inherited)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmt, err := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmt.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported: %v", stmt.Unsupported)
	}
	alter := stmt.DDL.Alter[0]
	if alter.Options["reset_count"] != "2" {
		t.Fatalf("expected reset_count=2, got %q", alter.Options["reset_count"])
	}
}
