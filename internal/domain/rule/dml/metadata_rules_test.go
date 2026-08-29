// Package dml verifies metadata-backed DML rule behavior.
// input: synthetic DML statements with optional target-table snapshots
// output: stable missing-target findings and dialect/metadata boundary coverage
// pos: domain DML metadata rule test seam
// note: if this file changes, update this header and module README.md.
package dml

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestTableExistenceRuleUsesOnlyDefinitiveMySQLTiDBSnapshots(t *testing.T) {
	t.Parallel()

	ruleUnderTest, err := newTableExistenceRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new table existence rule: %v", err)
	}

	cases := []struct {
		name      string
		dialect   spec.Dialect
		operation spec.DMLOperation
		metadata  *spec.Metadata
		want      bool
	}{
		{name: "missing insert", dialect: spec.DialectMySQL, operation: spec.DMLOperationInsert, metadata: missingTableMetadata(), want: true},
		{name: "missing update", dialect: spec.DialectTiDB, operation: spec.DMLOperationUpdate, metadata: missingTableMetadata(), want: true},
		{name: "missing delete", dialect: spec.DialectMySQL, operation: spec.DMLOperationDelete, metadata: missingTableMetadata(), want: true},
		{name: "existing table", dialect: spec.DialectMySQL, operation: spec.DMLOperationUpdate, metadata: &spec.Metadata{TargetTable: &spec.TableSnapshot{Exists: true}}, want: false},
		{name: "no snapshot", dialect: spec.DialectMySQL, operation: spec.DMLOperationUpdate, want: false},
		{name: "postgresql", dialect: spec.DialectPostgreSQL, operation: spec.DMLOperationUpdate, metadata: missingTableMetadata(), want: false},
		{name: "unknown operation", dialect: spec.DialectMySQL, operation: spec.DMLOperationUnknown, metadata: missingTableMetadata(), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			statement := spec.Statement{
				Kind:     spec.KindDML,
				Dialect:  tc.dialect,
				Metadata: tc.metadata,
				DML: &spec.DML{
					Operation: tc.operation,
					Tables:    []spec.Table{{Schema: "app", Name: "users"}},
				},
			}
			findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if (len(findings) == 1) != tc.want {
				t.Fatalf("expected finding=%t, got %#v", tc.want, findings)
			}
			if !tc.want {
				return
			}
			finding := findings[0]
			if finding.RuleID != "" {
				t.Fatalf("domain registry should attach rule id, got %q", finding.RuleID)
			}
			if finding.Level != rule.LevelBlocker {
				t.Fatalf("level = %q, want blocker", finding.Level)
			}
			if finding.Message != `table "users" does not exist in the target schema` {
				t.Fatalf("message = %q", finding.Message)
			}
			if finding.Metadata["table"] != "users" || finding.Metadata["exists"] != false {
				t.Fatalf("metadata = %#v", finding.Metadata)
			}
		})
	}
}

func missingTableMetadata() *spec.Metadata {
	return &spec.Metadata{TargetTable: &spec.TableSnapshot{Schema: "app", Exists: false}}
}
