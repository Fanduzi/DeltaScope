//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractAlterClusterOn(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users CLUSTER ON users_email_idx")
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
	if alter.Action != "cluster_on" {
		t.Fatalf("expected action cluster_on, got %q", alter.Action)
	}
	if alter.Name != "users_email_idx" {
		t.Fatalf("expected name users_email_idx, got %q", alter.Name)
	}
	if alter.Options["index"] != "users_email_idx" {
		t.Fatalf("expected option index=users_email_idx, got %q", alter.Options["index"])
	}
	if alter.Options["has_cluster_index"] != "true" {
		t.Fatalf("expected has_cluster_index=true, got %q", alter.Options["has_cluster_index"])
	}
	// No-leak: cluster SQL must not appear
	for _, forbidden := range []string{"cluster_sql", "raw_sql"} {
		if _, ok := alter.Options[forbidden]; ok {
			t.Fatalf("forbidden key %q present in options", forbidden)
		}
	}
}

func TestExtractAlterSetWithoutCluster(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE users SET WITHOUT CLUSTER")
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
	if alter.Action != "set_without_cluster" {
		t.Fatalf("expected action set_without_cluster, got %q", alter.Action)
	}
	if alter.Options["has_cluster_index"] != "false" {
		t.Fatalf("expected has_cluster_index=false, got %q", alter.Options["has_cluster_index"])
	}
	// No-leak
	for _, forbidden := range []string{"cluster_sql", "raw_sql"} {
		if _, ok := alter.Options[forbidden]; ok {
			t.Fatalf("forbidden key %q present in options", forbidden)
		}
	}
}

func TestExtractAlterDetachPartitionFinalize(t *testing.T) {
	t.Parallel()
	parser := New()

	result, err := parser.Parse(context.Background(), "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04 FINALIZE")
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
	if alter.Action != "detach_partition_finalize" {
		t.Fatalf("expected action detach_partition_finalize, got %q", alter.Action)
	}
	if alter.Options["partition"] != "measurement_y2026m04" {
		t.Fatalf("expected option partition=measurement_y2026m04, got %q", alter.Options["partition"])
	}
	if alter.Options["finalize"] != "true" {
		t.Fatalf("expected finalize=true, got %q", alter.Options["finalize"])
	}
	// No-leak: partition bounds must not appear
	for _, forbidden := range []string{"partition_bound", "raw_sql", "expression"} {
		if _, ok := alter.Options[forbidden]; ok {
			t.Fatalf("forbidden key %q present in options", forbidden)
		}
	}
}

func TestExtractAlterDetachPartitionFinalizeDistinctFromDetach(t *testing.T) {
	t.Parallel()
	parser := New()

	// Plain DETACH (non-finalize) must still produce detach_partition, not detach_partition_finalize
	result, err := parser.Parse(context.Background(), "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04")
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
	if alter.Action != "detach_partition" {
		t.Fatalf("expected action detach_partition for plain DETACH, got %q", alter.Action)
	}
	if alter.Options["finalize"] != "" {
		t.Fatalf("expected no finalize key for plain DETACH, got %q", alter.Options["finalize"])
	}
}
