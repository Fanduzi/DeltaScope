//go:build postgresql

package deltascope

import (
	"context"
	"testing"
)

func TestAuditPostgreSQLCreateViewEmitsDefaultForbidFinding(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "create view public.active_users as select id from public.users;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.view.create.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected create-view forbid finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLDropViewEmitsDefaultForbidFinding(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "drop view if exists public.active_users;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.view.drop.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop-view forbid finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLMetadataDoesNotLoadTableSnapshotForCreateView(t *testing.T) {
	provider := &fakeMetadataProvider{}

	result, err := Audit(context.Background(), Request{
		SQL:              "create view public.active_users as select id from public.users;",
		Dialect:          DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.tableCalls) != 0 {
		t.Fatalf("expected no table-snapshot calls for create view, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.view.create.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected create-view forbid finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditPostgreSQLMetadataDoesNotLoadTableSnapshotForDropView(t *testing.T) {
	provider := &fakeMetadataProvider{}

	result, err := Audit(context.Background(), Request{
		SQL:              "drop view if exists public.active_users;",
		Dialect:          DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(provider.tableCalls) != 0 {
		t.Fatalf("expected no table-snapshot calls for drop view, got %#v", provider.tableCalls)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	found := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.view.drop.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop-view forbid finding, got %#v", result.Statements[0].Findings)
	}
}
