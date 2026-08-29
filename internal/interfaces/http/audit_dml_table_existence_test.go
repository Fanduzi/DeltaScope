// Package httpapi verifies the HTTP DML table-existence contract.
// input: registry-backed HTTP audit requests and fake MySQL/TiDB metadata clients
// output: stable missing-target blocker findings in the HTTP audit response
// pos: HTTP public audit seam for metadata-aware DML existence behavior
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"context"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestExecuteAuditRequestExposesDMLTableExistenceFinding(t *testing.T) {
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
			previous := prepareHTTPMetadataAudit
			client := &metadataAuditTestClient{snapshot: &spec.TableSnapshot{Schema: "app", Exists: false}}
			prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
				return &auditmeta.PreparedAudit{
					Client:        client,
					Dialect:       tc.dialect,
					Schema:        "app",
					DialectSource: "detected",
					SchemaSource:  "registry",
				}, nil
			}
			t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

			response, err := executeAuditRequest(context.Background(), auditRequest{
				SQL:          tc.sql,
				ConnectionID: "test-conn",
			}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
				return deltascope.Audit(ctx, request)
			}, MetadataConfig{}, newTestRegistry(t, "test-conn", func(c *runtimeconfig.ConnectionConfig) {
				c.Dialect = string(tc.dialect)
			}), "default-key")
			if err != nil {
				t.Fatalf("execute audit request: %v", err)
			}
			if len(response.Statements) != 1 {
				t.Fatalf("expected one statement, got %#v", response.Statements)
			}
			for _, finding := range response.Statements[0].Findings {
				if finding.RuleID == "dml.table.exists.require" {
					if finding.Level != deltascope.LevelBlocker || finding.Message != `table "missing_users" does not exist in the target schema` {
						t.Fatalf("unexpected finding shape: %#v", finding)
					}
					return
				}
			}
			t.Fatalf("expected dml.table.exists.require finding, got %#v", response.Statements[0].Findings)
		})
	}
}
