// Package mcpapi verifies the MCP DML table-existence contract.
// input: metadata-aware MCP audit_sql requests and fake MySQL/TiDB metadata clients
// output: stable missing-target blocker findings in structured MCP results
// pos: MCP public audit seam for metadata-aware DML existence behavior
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestAuditSQLWithMetadataExposesDMLTableExistenceFinding(t *testing.T) {
	cases := []struct {
		dialect spec.Dialect
		sql     string
	}{
		{dialect: spec.DialectMySQL, sql: "INSERT INTO missing_users (id) VALUES (1)"},
		{dialect: spec.DialectMySQL, sql: "UPDATE missing_users SET name = 'x' WHERE id = 1"},
		{dialect: spec.DialectMySQL, sql: "DELETE FROM missing_users WHERE id = 1"},
		{dialect: spec.DialectTiDB, sql: "INSERT INTO missing_users (id) VALUES (1)"},
		{dialect: spec.DialectTiDB, sql: "UPDATE missing_users SET name = 'x' WHERE id = 1"},
		{dialect: spec.DialectTiDB, sql: "DELETE FROM missing_users WHERE id = 1"},
	}

	for _, tc := range cases {
		t.Run(string(tc.dialect)+"/"+tc.sql[:6], func(t *testing.T) {
			previous := prepareMetadataAudit
			prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
				return &auditmeta.PreparedAudit{
					Client:        metadataOnlyClientWithDialect{dialect: tc.dialect},
					Dialect:       tc.dialect,
					Schema:        "app",
					DialectSource: "detected",
					SchemaSource:  "request",
				}, nil
			}
			t.Cleanup(func() { prepareMetadataAudit = previous })

			_, payloadValue, err := auditSQLWithMetadata(context.Background(), AuditSQLParams{
				SQL:     tc.sql,
				Dialect: string(tc.dialect),
			}, ResolvedConnection{
				Enabled: true,
				Source:  MetadataSourceDirect,
				Schema:  "app",
			}, 0)
			if err != nil {
				t.Fatalf("audit_sql: %v", err)
			}
			payload, ok := payloadValue.(AuditSQLResult)
			if !ok {
				t.Fatalf("expected AuditSQLResult payload, got %T", payloadValue)
			}
			if len(payload.Statements) != 1 {
				t.Fatalf("expected one statement, got %#v", payload.Statements)
			}
			for _, finding := range payload.Statements[0].Findings {
				if finding.RuleID == "dml.table.exists.require" {
					if finding.Level != deltascope.LevelBlocker || finding.Message != `table "missing_users" does not exist in the target schema` {
						t.Fatalf("unexpected finding shape: %#v", finding)
					}
					return
				}
			}
			t.Fatalf("expected dml.table.exists.require finding, got %#v", payload.Statements[0].Findings)
		})
	}
}
