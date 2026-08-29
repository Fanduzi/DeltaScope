// Package cli verifies the CLI DML table-existence contract.
// input: metadata-aware CLI audit arguments and fake MySQL/TiDB metadata clients
// output: stable missing-target blocker findings in rendered JSON
// pos: CLI public audit seam for metadata-aware DML existence behavior
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditCommandExposesDMLTableExistenceFinding(t *testing.T) {
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
			previous := newMetadataClient
			newMetadataClient = func(auditConnectionOptions) (metadataClient, error) {
				return &fakeMetadataClient{
					detectDialect: tc.dialect,
					snapshot:      &spec.TableSnapshot{Schema: "app", Exists: false},
				}, nil
			}
			t.Cleanup(func() { newMetadataClient = previous })

			stdout := &strings.Builder{}
			code := Execute(context.Background(), []string{
				"audit", "--sql", tc.sql, "--dialect", string(tc.dialect),
				"--host", "127.0.0.1", "--user", "root", "--schema", "app",
				"--format", "json", "--fail-on", "none",
			}, strings.NewReader(""), stdout, &strings.Builder{})
			if code != exitOK {
				t.Fatalf("expected success exit with fail-on none, got %d: %s", code, stdout.String())
			}

			var body struct {
				Statements []struct {
					Findings []struct {
						RuleID  string `json:"rule_id"`
						Level   string `json:"level"`
						Message string `json:"message"`
					} `json:"findings"`
				} `json:"statements"`
			}
			if err := json.Unmarshal([]byte(stdout.String()), &body); err != nil {
				t.Fatalf("decode audit JSON: %v\n%s", err, stdout.String())
			}
			if len(body.Statements) != 1 {
				t.Fatalf("expected one statement, got %#v", body.Statements)
			}
			for _, finding := range body.Statements[0].Findings {
				if finding.RuleID == "dml.table.exists.require" {
					if finding.Level != "blocker" || finding.Message != `table "missing_users" does not exist in the target schema` {
						t.Fatalf("unexpected finding shape: %#v", finding)
					}
					return
				}
			}
			t.Fatalf("expected dml.table.exists.require finding, got %#v", body.Statements[0].Findings)
		})
	}
}
