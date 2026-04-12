//go:build postgresql

// Package httpapi verifies PostgreSQL-only metadata-aware HTTP audit behavior.
// input: HTTP audit requests with PostgreSQL dialect plus fake shared metadata preparation results
// output: focused coverage for PG-capable metadata-aware context and public audit request wiring
// pos: tagged HTTP adapter regression coverage for PostgreSQL metadata-aware support
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"context"
	"strings"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	ifaceconn "github.com/Fanduzi/DeltaScope/internal/interfaces/metadata"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

type plannerMetadataAuditTestClient struct {
	metadataAuditTestClient
	planCalls int
}

func (c *plannerMetadataAuditTestClient) LoadPlanEstimate(context.Context, spec.Statement) (*spec.ImpactEstimate, error) {
	c.planCalls++
	rows := int64(7)
	ratio := 0.07
	return &spec.ImpactEstimate{
		EstimatedRows:  &rows,
		EstimatedRatio: &ratio,
		RiskLevel:      spec.ImpactRiskMedium,
		Confidence:     spec.ImpactConfidenceHigh,
		Source:         spec.ImpactSourcePlan,
		ReasonCodes:    []string{"planner_estimate"},
	}, nil
}

func TestExecuteAuditRequestSupportsPostgreSQLMetadataAwareMode(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &plannerMetadataAuditTestClient{metadataAuditTestClient: metadataAuditTestClient{detectDialect: spec.DialectPostgreSQL}}
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		if request.Connection.Dialect != spec.DialectPostgreSQL {
			t.Fatalf("expected postgresql dialect hint to flow into shared prepare, got %#v", request.Connection)
		}
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "inferred",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "delete from public.users where id = 1",
		Dialect: deltascope.DialectPostgreSQL,
		Connection: &ifaceconn.ConnectionInput{
			Host: "127.0.0.1",
			User: "root",
		},
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		result, err := deltascope.Audit(ctx, request)
		if err != nil {
			return deltascope.Result{}, err
		}
		return result, nil
	})
	if err != nil {
		t.Fatalf("expected postgresql metadata-aware request to succeed, got %v", err)
	}
	if response.Context == nil || response.Context.Mode != "metadata-aware" {
		t.Fatalf("expected metadata-aware context, got %#v", response.Context)
	}
	if len(response.Statements) != 1 || response.Statements[0].Impact == nil {
		t.Fatalf("expected statement impact, got %#v", response.Statements)
	}
	impact := response.Statements[0].Impact
	if impact.Source != deltascope.ImpactSourcePlan {
		t.Fatalf("expected planner impact source, got %#v", impact)
	}
	if impact.EstimatedRows == nil || *impact.EstimatedRows != 7 {
		t.Fatalf("expected estimated rows 7, got %#v", impact)
	}
	if client.planCalls != 1 {
		t.Fatalf("expected one planner call, got %d", client.planCalls)
	}
	if !client.closed {
		t.Fatalf("expected metadata client close to be called")
	}
}

func TestExecuteAuditRequestPostgreSQLMetadataAwareUPDATETriggersPlanEstimation(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &plannerMetadataAuditTestClient{metadataAuditTestClient: metadataAuditTestClient{detectDialect: spec.DialectPostgreSQL}}
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "inferred",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "update public.users set name = 'x' where id = 1",
		Dialect: deltascope.DialectPostgreSQL,
		Connection: &ifaceconn.ConnectionInput{
			Host: "127.0.0.1",
			User: "root",
		},
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		result, err := deltascope.Audit(ctx, request)
		if err != nil {
			return deltascope.Result{}, err
		}
		return result, nil
	})
	if err != nil {
		t.Fatalf("expected postgresql metadata-aware request to succeed, got %v", err)
	}
	if response.Context == nil || response.Context.Mode != "metadata-aware" {
		t.Fatalf("expected metadata-aware context, got %#v", response.Context)
	}
	if client.planCalls != 1 {
		t.Fatalf("expected one planner call, got %d", client.planCalls)
	}
	if len(response.Statements) != 1 || response.Statements[0].Impact == nil {
		t.Fatalf("expected statement impact, got %#v", response.Statements)
	}
	if response.Statements[0].Impact.Source != deltascope.ImpactSourcePlan {
		t.Fatalf("expected planner impact source, got %#v", response.Statements[0].Impact)
	}
}

func TestExecuteAuditRequestPostgreSQLMetadataAwareINSERTDoesNotTriggerPlanEstimation(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &plannerMetadataAuditTestClient{metadataAuditTestClient: metadataAuditTestClient{detectDialect: spec.DialectPostgreSQL}}
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "inferred",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "insert into public.users (id, name) values (1, 'alice')",
		Dialect: deltascope.DialectPostgreSQL,
		Connection: &ifaceconn.ConnectionInput{
			Host: "127.0.0.1",
			User: "root",
		},
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		result, err := deltascope.Audit(ctx, request)
		if err != nil {
			return deltascope.Result{}, err
		}
		return result, nil
	})
	if err != nil {
		t.Fatalf("expected postgresql metadata-aware request to succeed, got %v", err)
	}
	if response.Context == nil || response.Context.Mode != "metadata-aware" {
		t.Fatalf("expected metadata-aware context, got %#v", response.Context)
	}
	if client.planCalls != 0 {
		t.Fatalf("expected no planner calls for INSERT, got %d", client.planCalls)
	}
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement, got %#v", response.Statements)
	}
}

func TestExecuteAuditRequestPostgreSQLMetadataMapsDropConstraintToPrimaryKeyRule(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &plannerMetadataAuditTestClient{metadataAuditTestClient: metadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{
			Exists:      true,
			Table:       &spec.Table{Name: "users"},
			PrimaryKey:  &spec.Index{Name: "users_primary_idx", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
			Constraints: []spec.Constraint{{Type: "primary_key", Name: "users_pkey", Columns: []string{"id"}}},
		},
	}}
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "alter table users drop constraint users_pkey;",
		Dialect: deltascope.DialectPostgreSQL,
		Connection: &ifaceconn.ConnectionInput{
			Host:   "127.0.0.1",
			User:   "root",
			Schema: "public",
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
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	found := false
	for _, finding := range response.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.drop_primary_key.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop primary key finding, got %#v", response.Statements[0].Findings)
	}
}

func TestExecuteAuditRequestPostgreSQLMetadataRequiresExistingColumnForRenameColumn(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &plannerMetadataAuditTestClient{metadataAuditTestClient: metadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
			Columns: []spec.Column{
				{Name: "email"},
			},
		},
	}}
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "alter table users rename column missing_email to email;",
		Dialect: deltascope.DialectPostgreSQL,
		Connection: &ifaceconn.ConnectionInput{
			Host:   "127.0.0.1",
			User:   "root",
			Schema: "public",
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
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	found := false
	for _, finding := range response.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.rename_column.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rename-column existence finding, got %#v", response.Statements[0].Findings)
	}
}

func TestExecuteAuditRequestPostgreSQLMetadataRequiresExistingColumnForDropColumn(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &plannerMetadataAuditTestClient{metadataAuditTestClient: metadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Name: "users"},
			Columns: []spec.Column{
				{Name: "email"},
			},
		},
	}}
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "alter table users drop column missing_email;",
		Dialect: deltascope.DialectPostgreSQL,
		Connection: &ifaceconn.ConnectionInput{
			Host:   "127.0.0.1",
			User:   "root",
			Schema: "public",
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
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	found := false
	for _, finding := range response.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.drop_column.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop-column existence finding, got %#v", response.Statements[0].Findings)
	}
}

func TestExecuteAuditRequestPostgreSQLMetadataRequiresExistingTableForRenameTable(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	client := &plannerMetadataAuditTestClient{metadataAuditTestClient: metadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{
			Exists: false,
			Table:  &spec.Table{Name: "users"},
		},
	}}
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "alter table users rename to users_archive;",
		Dialect: deltascope.DialectPostgreSQL,
		Connection: &ifaceconn.ConnectionInput{
			Host:   "127.0.0.1",
			User:   "root",
			Schema: "public",
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
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	found := false
	for _, finding := range response.Statements[0].Findings {
		if finding.RuleID == "ddl.table.exists.alter.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected alter-table existence finding, got %#v", response.Statements[0].Findings)
	}
}

func TestExecuteAuditRequestPostgreSQLAlterColumnActionsMapToSemanticRules(t *testing.T) {
	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "alter table users alter column created_at set default now(), alter column updated_at drop default, alter column email set not null, alter column phone drop not null;",
		Dialect: deltascope.DialectPostgreSQL,
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		return deltascope.Audit(ctx, request)
	})
	if err != nil {
		t.Fatalf("expected postgresql request to succeed, got %v", err)
	}
	if response.Context == nil || response.Context.Mode != "offline" {
		t.Fatalf("expected offline context, got %#v", response.Context)
	}
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	counts := map[string]int{}
	for _, finding := range response.Statements[0].Findings {
		counts[finding.RuleID]++
	}
	if len(response.Statements[0].Findings) != 8 {
		t.Fatalf("expected exactly 8 alter-column findings, got %#v", response.Statements[0].Findings)
	}
	if counts["ddl.alter.set_default.explicit_default_change.forbid"] != 1 {
		t.Fatalf("expected set_default semantic finding, got %#v", response.Statements[0].Findings)
	}
	if counts["ddl.alter.drop_default.explicit_default_change.forbid"] != 1 {
		t.Fatalf("expected drop_default semantic finding, got %#v", response.Statements[0].Findings)
	}
	if counts["ddl.alter.set_not_null.explicit_nullability_change.forbid"] != 1 {
		t.Fatalf("expected set_not_null semantic finding, got %#v", response.Statements[0].Findings)
	}
	if counts["ddl.alter.drop_not_null.explicit_nullability_change.forbid"] != 1 {
		t.Fatalf("expected drop_not_null semantic finding, got %#v", response.Statements[0].Findings)
	}
}

func TestExecuteAuditRequestPostgreSQLSetDataTypeMapsToForbidRule(t *testing.T) {
	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "alter table users alter column status type bigint;",
		Dialect: deltascope.DialectPostgreSQL,
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		return deltascope.Audit(ctx, request)
	})
	if err != nil {
		t.Fatalf("expected postgresql request to succeed, got %v", err)
	}
	if response.Context == nil || response.Context.Mode != "offline" {
		t.Fatalf("expected offline context, got %#v", response.Context)
	}
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	counts := make(map[string]int)
	for _, finding := range response.Statements[0].Findings {
		counts[finding.RuleID]++
	}
	if counts["ddl.alter.set_data_type.forbid"] != 1 {
		t.Fatalf("expected set_data_type forbid finding, got %#v", response.Statements[0].Findings)
	}
	if counts["ddl.pg.alter.set_data_type.rewrite.warn"] != 1 {
		t.Fatalf("expected pg set_data_type rewrite warning, got %#v", response.Statements[0].Findings)
	}
}

func TestExecuteAuditRequestPostgreSQLRenameIndexMapsToForbidRule(t *testing.T) {
	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "alter index idx_old rename to idx_new;",
		Dialect: deltascope.DialectPostgreSQL,
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		return deltascope.Audit(ctx, request)
	})
	if err != nil {
		t.Fatalf("expected postgresql request to succeed, got %v", err)
	}
	if response.Context == nil || response.Context.Mode != "offline" {
		t.Fatalf("expected offline context, got %#v", response.Context)
	}
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	if len(response.Statements[0].Findings) != 1 {
		t.Fatalf("expected exactly 1 rename_index finding, got %#v", response.Statements[0].Findings)
	}
	if response.Statements[0].Findings[0].RuleID != "ddl.alter.rename_index.forbid" {
		t.Fatalf("expected rename_index forbid finding, got %#v", response.Statements[0].Findings)
	}
}

func TestExecuteAuditRequestPostgreSQLCreateViewMapsToForbidRule(t *testing.T) {
	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "create view public.active_users as select id from public.users;",
		Dialect: deltascope.DialectPostgreSQL,
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		return deltascope.Audit(ctx, request)
	})
	if err != nil {
		t.Fatalf("expected postgresql request to succeed, got %v", err)
	}
	if response.Context == nil || response.Context.Mode != "offline" {
		t.Fatalf("expected offline context, got %#v", response.Context)
	}
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	if len(response.Statements[0].Findings) != 1 {
		t.Fatalf("expected exactly 1 create_view finding, got %#v", response.Statements[0].Findings)
	}
	if response.Statements[0].Findings[0].RuleID != "ddl.view.create.forbid" {
		t.Fatalf("expected create_view forbid finding, got %#v", response.Statements[0].Findings)
	}
}

func TestExecuteAuditRequestPostgreSQLDropViewMapsToForbidRule(t *testing.T) {
	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "drop view if exists public.active_users;",
		Dialect: deltascope.DialectPostgreSQL,
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		return deltascope.Audit(ctx, request)
	})
	if err != nil {
		t.Fatalf("expected postgresql request to succeed, got %v", err)
	}
	if response.Context == nil || response.Context.Mode != "offline" {
		t.Fatalf("expected offline context, got %#v", response.Context)
	}
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	if len(response.Statements[0].Findings) != 1 {
		t.Fatalf("expected exactly 1 drop_view finding, got %#v", response.Statements[0].Findings)
	}
	if response.Statements[0].Findings[0].RuleID != "ddl.view.drop.forbid" {
		t.Fatalf("expected drop_view forbid finding, got %#v", response.Statements[0].Findings)
	}
}

func TestExecuteAuditRequestPostgreSQLValidateConstraintReturnsNormalResult(t *testing.T) {
	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "alter table users validate constraint chk_amount;",
		Dialect: deltascope.DialectPostgreSQL,
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		return deltascope.Audit(ctx, request)
	})
	if err != nil {
		t.Fatalf("expected postgresql request to succeed, got %v", err)
	}
	if response.Context == nil || response.Context.Mode != "offline" {
		t.Fatalf("expected offline context, got %#v", response.Context)
	}
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	if response.Statements[0].Kind != "ddl" {
		t.Fatalf("expected ddl kind, got %q", response.Statements[0].Kind)
	}
	for _, finding := range response.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.drop_primary_key.forbid" {
			t.Fatalf("validate_constraint should not trigger drop_primary_key finding, got %#v", finding)
		}
	}
}

func TestExecuteAuditRequestPostgreSQLAlterColumnSetNotNullReturnsNormalResult(t *testing.T) {
	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "alter table users alter column status set not null;",
		Dialect: deltascope.DialectPostgreSQL,
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		return deltascope.Audit(ctx, request)
	})
	if err != nil {
		t.Fatalf("expected postgresql request to succeed, got %v", err)
	}
	if response.Context == nil || response.Context.Mode != "offline" {
		t.Fatalf("expected offline context, got %#v", response.Context)
	}
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	if response.Statements[0].Kind != "ddl" {
		t.Fatalf("expected ddl kind, got %q", response.Statements[0].Kind)
	}
	found := false
	for _, finding := range response.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.set_not_null.explicit_nullability_change.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected set_not_null semantic finding, got %#v", response.Statements[0].Findings)
	}
}

func TestExecuteAuditRequestPostgreSQLDropNonPrimaryKeyConstraintDoesNotTriggerPrimaryKeyFinding(t *testing.T) {
	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "alter table users drop constraint chk_amount;",
		Dialect: deltascope.DialectPostgreSQL,
	}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		return deltascope.Audit(ctx, request)
	})
	if err != nil {
		t.Fatalf("expected postgresql request to succeed, got %v", err)
	}
	if response.Context == nil || response.Context.Mode != "offline" {
		t.Fatalf("expected offline context, got %#v", response.Context)
	}
	if len(response.Statements) != 1 {
		t.Fatalf("expected one statement result, got %#v", response.Statements)
	}
	for _, finding := range response.Statements[0].Findings {
		if finding.RuleID == "ddl.alter.drop_primary_key.forbid" {
			t.Fatalf("expected no drop_primary_key finding for non-PK constraint, got %#v", finding)
		}
	}
}

func TestExecuteAuditRequestPostgreSQLMetadataResolvesOwningTableForRenameIndex(t *testing.T) {
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
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "alter index missing_idx rename to idx_new;",
		Dialect: deltascope.DialectPostgreSQL,
		Connection: &ifaceconn.ConnectionInput{
			Host:   "127.0.0.1",
			User:   "root",
			Schema: "public",
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
	if len(client.indexSchemas) != 1 || client.indexSchemas[0] != "public" {
		t.Fatalf("expected public schema for index-owner resolution, got %#v", client.indexSchemas)
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

func TestExecuteAuditRequestPostgreSQLMetadataResolvesOwningTableForDropIndex(t *testing.T) {
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
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	response, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:     "drop index missing_idx;",
		Dialect: deltascope.DialectPostgreSQL,
		Connection: &ifaceconn.ConnectionInput{
			Host:   "127.0.0.1",
			User:   "root",
			Schema: "public",
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
	if len(client.indexSchemas) != 1 || client.indexSchemas[0] != "public" {
		t.Fatalf("expected public schema for index-owner resolution, got %#v", client.indexSchemas)
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
		if finding.RuleID == "ddl.alter.drop_index.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop-index existence finding, got %#v", response.Statements[0].Findings)
	}
}

func TestExecuteAuditRequestPostgreSQLCreateTableConstraintsReturnNormalResult(t *testing.T) {
	cases := map[string]string{
		"named table-level CHECK":       "create table orders (id bigint primary key, amount numeric, constraint chk_orders_amount check (amount > 0));",
		"column-level inline CHECK":     "create table orders (id bigint primary key, amount numeric check (amount > 0));",
		"named table-level UNIQUE":      "create table users (id bigint primary key, email text, constraint uq_users_email unique (email));",
		"column-level inline UNIQUE":    "create table users (id bigint primary key, email text unique);",
		"named table-level FOREIGN KEY": "create table orders (id bigint primary key, user_id bigint, constraint fk_orders_user foreign key (user_id) references users(id));",
		"column-level inline REFERENCES": "create table orders (id bigint primary key, user_id bigint references users(id));",
	}

	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			response, err := executeAuditRequest(context.Background(), auditRequest{
				SQL:     sql,
				Dialect: deltascope.DialectPostgreSQL,
			}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
				return deltascope.Audit(ctx, request)
			})
			if err != nil {
				t.Fatalf("expected postgresql request to succeed, got %v", err)
			}
			if response.Context == nil || response.Context.Mode != "offline" {
				t.Fatalf("expected offline context, got %#v", response.Context)
			}
			if len(response.Statements) != 1 {
				t.Fatalf("expected one statement result, got %#v", response.Statements)
			}
			if response.Statements[0].Kind != "ddl" {
				t.Fatalf("expected ddl kind, got %q", response.Statements[0].Kind)
			}
			if len(response.Result.Unsupported) != 0 {
				t.Fatalf("expected no unsupported entries, got %#v", response.Result.Unsupported)
			}
		})
	}
}

func TestExecuteAuditRequestPostgreSQLCreateTableForeignKeyRendersForbidFinding(t *testing.T) {
	cases := map[string]string{
		"named FOREIGN KEY":  "create table orders (id bigint primary key, user_id bigint, constraint bad_fk foreign key (user_id) references users(id));",
		"inline REFERENCES": "create table orders (id bigint primary key, user_id bigint references users(id));",
	}

	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			response, err := executeAuditRequest(context.Background(), auditRequest{
				SQL:     sql,
				Dialect: deltascope.DialectPostgreSQL,
			}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
				return deltascope.Audit(ctx, request)
			})
			if err != nil {
				t.Fatalf("expected postgresql request to succeed, got %v", err)
			}
			if len(response.Statements) != 1 {
				t.Fatalf("expected one statement result, got %#v", response.Statements)
			}
			if response.Statements[0].Kind != "ddl" {
				t.Fatalf("expected ddl kind, got %q", response.Statements[0].Kind)
			}
			found := false
			for _, finding := range response.Statements[0].Findings {
				if finding.RuleID == "ddl.table.foreign_key.forbid" {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected foreign_key forbid finding, got %#v", response.Statements[0].Findings)
			}
		})
	}
}

func TestExecuteAuditRequestPostgreSQLCreateTableBoundaryReturnsUnsupportedError(t *testing.T) {
	cases := map[string]struct {
		sql     string
		feature string
	}{
		"identity": {
			sql:     "CREATE TABLE users (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, email text);",
			feature: "generated_as_identity",
		},
		"generated_stored": {
			sql:     "CREATE TABLE users (first_name text, last_name text, full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED);",
			feature: "generated_column",
		},
		"exclusion": {
			sql:     "CREATE TABLE bookings (room_id int, during tsrange, EXCLUDE USING gist (room_id WITH =, during WITH &&));",
			feature: "exclusion_constraint",
		},
		"partitioned": {
			sql:     "CREATE TABLE events (id bigint, created_at timestamptz NOT NULL) PARTITION BY RANGE (created_at);",
			feature: "partitioning",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := executeAuditRequest(context.Background(), auditRequest{
				SQL:     tc.sql,
				Dialect: deltascope.DialectPostgreSQL,
			}, "", func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
				return deltascope.Audit(ctx, request)
			})
			if err == nil {
				t.Fatalf("expected unsupported error for %s, got nil", name)
			}
			if !strings.Contains(err.Error(), "unsupported") {
				t.Fatalf("expected unsupported error message, got %v", err)
			}
		})
	}
}
