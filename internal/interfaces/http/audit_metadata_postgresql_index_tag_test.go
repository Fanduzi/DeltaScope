//go:build postgresql

// Package httpapi verifies PostgreSQL standalone index metadata-aware HTTP behavior.
// input: HTTP audit requests for standalone PostgreSQL index statements with fake metadata clients
// output: focused transport parity coverage for standalone index owner resolution paths
// pos: tagged HTTP adapter regression coverage for PostgreSQL standalone index metadata support
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"context"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	ifaceconn "github.com/Fanduzi/DeltaScope/internal/interfaces/metadata"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestExecuteAuditRequestPostgreSQLMetadataResolvesQualifiedRenameIndexWithoutRequestSchema(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &plannerMetadataAuditTestClient{metadataAuditTestClient: metadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		indexTable:    "users",
		snapshot: &spec.TableSnapshot{
			Exists:  true,
			Table:   &spec.Table{Name: "users"},
			Indexes: []spec.Index{{Name: "idx_users_email", Kind: spec.IndexKindSecondary}},
		},
	}}
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			DialectSource: "request",
			SchemaSource:  "none",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "alter index accounting.missing_idx rename to idx_new;",
		Dialect: deltascope.DialectPostgreSQL,
		Connection: &ifaceconn.ConnectionInput{
			Host: "127.0.0.1",
			User: "root",
		},
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		return deltascope.Audit(ctx, request)
	})
	if err != nil {
		t.Fatalf("expected postgresql metadata-aware request to succeed, got %v", err)
	}
	if response.Context == nil || response.Context.Mode != "metadata-aware" {
		t.Fatalf("expected metadata-aware context, got %#v", response.Context)
	}
	if len(client.indexCalls) != 1 || client.indexCalls[0] != "missing_idx" {
		t.Fatalf("expected one index-owner resolution, got %#v", client.indexCalls)
	}
	if len(client.indexSchemas) != 1 || client.indexSchemas[0] != "accounting" {
		t.Fatalf("expected accounting schema for index-owner resolution, got %#v", client.indexSchemas)
	}
	if len(client.indexDialects) != 1 || client.indexDialects[0] != spec.DialectPostgreSQL {
		t.Fatalf("expected postgresql dialect for index-owner resolution, got %#v", client.indexDialects)
	}
	if len(client.tableCalls) != 1 || client.tableCalls[0] != "users" {
		t.Fatalf("expected one table-snapshot call for users, got %#v", client.tableCalls)
	}
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	found := false
	for _, finding := range response.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.rename_index.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rename-index existence finding, got %#v", response.Statements[0].Findings)
	}
}
