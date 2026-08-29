// Package deltascope verifies the public DML table-existence contract.
// input: public MySQL/TiDB audit requests and metadata providers
// output: stable missing-target blocker findings without offline claims
// pos: public SDK audit seam for metadata-aware DML existence behavior
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"testing"
)

func TestAuditPublicDMLTableExistenceContract(t *testing.T) {
	cases := []struct {
		dialect Dialect
		sql     string
	}{
		{dialect: DialectMySQL, sql: "INSERT INTO missing_users (id) VALUES (1)"},
		{dialect: DialectMySQL, sql: "UPDATE missing_users SET name = 'x' WHERE id = 1"},
		{dialect: DialectMySQL, sql: "DELETE FROM missing_users WHERE id = 1"},
		{dialect: DialectTiDB, sql: "INSERT INTO missing_users (id) VALUES (1)"},
		{dialect: DialectTiDB, sql: "UPDATE missing_users SET name = 'x' WHERE id = 1"},
		{dialect: DialectTiDB, sql: "DELETE FROM missing_users WHERE id = 1"},
	}

	for _, tc := range cases {
		t.Run(string(tc.dialect)+"/"+tc.sql[:6], func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:              tc.sql,
				Dialect:          tc.dialect,
				Schema:           "app",
				MetadataProvider: &fakeMetadataProvider{snapshot: &TableSnapshot{Schema: "app"}},
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			finding, ok := publicDMLTableExistenceFinding(result.Statements[0].Findings)
			if !ok {
				t.Fatalf("expected missing-target finding, got %#v", result.Statements[0].Findings)
			}
			if finding.Level != LevelBlocker || finding.Message != `table "missing_users" does not exist in the target schema` {
				t.Fatalf("unexpected finding shape: %#v", finding)
			}
			if finding.Metadata["table"] != "missing_users" || finding.Metadata["exists"] != false {
				t.Fatalf("unexpected finding metadata: %#v", finding.Metadata)
			}
		})
	}
}

func TestAuditPublicDMLTableExistenceSkipsExistingAndOffline(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider MetadataProvider
	}{
		{name: "existing", provider: &fakeMetadataProvider{snapshot: &TableSnapshot{Exists: true}}},
		{name: "offline", provider: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:              "UPDATE users SET name = 'x' WHERE id = 1",
				Dialect:          DialectMySQL,
				Schema:           "app",
				MetadataProvider: tc.provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if finding, ok := publicDMLTableExistenceFinding(result.Statements[0].Findings); ok {
				t.Fatalf("unexpected missing-target finding: %#v", finding)
			}
		})
	}
}

func publicDMLTableExistenceFinding(findings []Finding) (Finding, bool) {
	for _, finding := range findings {
		if finding.RuleID == "dml.table.exists.require" {
			return finding, true
		}
	}
	return Finding{}, false
}
