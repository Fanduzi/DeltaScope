//go:build postgresql

// Package cli verifies PostgreSQL-specific CLI metadata-aware audit behavior.
// input: PostgreSQL audit arguments and fake metadata clients
// output: PostgreSQL metadata rule findings and valid database/schema connection coverage
// pos: interface-layer PostgreSQL-tagged CLI audit tests
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditCommandSupportsPostgreSQLMetadataAwareMode(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{detectDialect: spec.DialectPostgreSQL}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from public.users where id = 1", "--host", "127.0.0.1", "--user", "root", "--dialect", "postgresql"},
		strings.NewReader(""),
		&strings.Builder{},
		&strings.Builder{},
	)

	if code != exitOK {
		t.Fatalf("expected metadata-aware postgresql path to succeed, got %d", code)
	}
	if client.options.Dialect != string(spec.DialectPostgreSQL) {
		t.Fatalf("expected postgresql dialect to flow into opener, got %#v", client.options)
	}
	if len(client.findSchemaCalls) != 0 {
		t.Fatalf("expected qualified schema to skip inference, got %#v", client.findSchemaCalls)
	}
	if len(client.instanceCalls) != 1 || client.instanceCalls[0] != "public" {
		t.Fatalf("expected public schema instance lookup, got %#v", client.instanceCalls)
	}
	if len(client.tableSnapshotCalls) != 1 || client.tableSnapshotCalls[0].Schema != "public" || client.tableSnapshotCalls[0].Table != "users" {
		t.Fatalf("expected public.users snapshot lookup, got %#v", client.tableSnapshotCalls)
	}
	if !client.closed {
		t.Fatalf("expected metadata client close to be called")
	}
}

func TestAuditCommandPostgreSQLMetadataAwareDELETETriggersPlanEstimation(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{detectDialect: spec.DialectPostgreSQL}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "delete from public.users where id = 1", "--host", "127.0.0.1", "--user", "root", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitOK {
		t.Fatalf("expected metadata-aware postgresql path to succeed, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
	}
	if client.planCalls != 1 {
		t.Fatalf("expected one plan estimation call, got %d", client.planCalls)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	impact, ok := statement["impact"].(map[string]any)
	if !ok {
		t.Fatalf("expected statement impact, got %#v", statement["impact"])
	}
	if impact["source"] != "plan" {
		t.Fatalf("expected planner impact source, got %#v", impact)
	}
	if rows, ok := impact["estimated_rows"].(float64); !ok || rows != 7 {
		t.Fatalf("expected estimated_rows 7, got %#v", impact["estimated_rows"])
	}
}

func TestAuditCommandPostgreSQLMetadataAwareUPDATETriggersPlanEstimation(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{detectDialect: spec.DialectPostgreSQL}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "update public.users set name = 'x' where id = 1", "--host", "127.0.0.1", "--user", "root", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitOK {
		t.Fatalf("expected metadata-aware postgresql path to succeed, got %d", code)
	}
	if client.planCalls != 1 {
		t.Fatalf("expected one plan estimation call, got %d", client.planCalls)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	impact, ok := statement["impact"].(map[string]any)
	if !ok {
		t.Fatalf("expected statement impact, got %#v", statement["impact"])
	}
	if impact["source"] != "plan" {
		t.Fatalf("expected planner impact source, got %#v", impact)
	}
}

func TestAuditCommandPostgreSQLMetadataAwareINSERTDoesNotTriggerPlanEstimation(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{detectDialect: spec.DialectPostgreSQL}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "insert into public.users (id, name) values (1, 'alice')", "--host", "127.0.0.1", "--user", "root", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitOK {
		t.Fatalf("expected metadata-aware postgresql path to succeed, got %d", code)
	}
	if client.planCalls != 0 {
		t.Fatalf("expected no planner calls for INSERT, got %d", client.planCalls)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", decoded["statements"])
	}
}

func TestAuditCommandPostgreSQLMetadataMapsDropConstraintToPrimaryKeyRule(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{
			Exists:      true,
			Schema:      "public",
			Table:       &spec.Table{Name: "users"},
			PrimaryKey:  &spec.Index{Name: "users_primary_idx", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
			Constraints: []spec.Constraint{{Type: "primary_key", Name: "users_pkey", Columns: []string{"id"}}},
		},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users drop constraint users_pkey;", "--host", "127.0.0.1", "--user", "root", "--database", "app", "--schema", "public", "--dialect", "postgresql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "drop primary key") {
		t.Fatalf("expected primary-key drop finding in stdout, got %q", stdout.String())
	}
	if len(client.tableSnapshotCalls) != 1 || client.tableSnapshotCalls[0].Schema != "public" || client.tableSnapshotCalls[0].Table != "users" {
		t.Fatalf("expected public.users snapshot lookup, got %#v", client.tableSnapshotCalls)
	}
}

func TestAuditCommandPostgreSQLMetadataRequiresExistingColumnForRenameColumn(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Schema: "public",
			Table:  &spec.Table{Name: "users"},
			Columns: []spec.Column{
				{Name: "email"},
			},
		},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users rename column missing_email to email;", "--host", "127.0.0.1", "--user", "root", "--database", "app", "--schema", "public", "--dialect", "postgresql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "does not exist on table \"users\"") {
		t.Fatalf("expected rename-column existence finding in stdout, got %q", stdout.String())
	}
	if len(client.tableSnapshotCalls) != 1 || client.tableSnapshotCalls[0].Schema != "public" || client.tableSnapshotCalls[0].Table != "users" {
		t.Fatalf("expected public.users snapshot lookup, got %#v", client.tableSnapshotCalls)
	}
}

func TestAuditCommandPostgreSQLMetadataRequiresExistingColumnForDropColumn(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{
			Exists: true,
			Schema: "public",
			Table:  &spec.Table{Name: "users"},
			Columns: []spec.Column{
				{Name: "email"},
			},
		},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users drop column missing_email;", "--host", "127.0.0.1", "--user", "root", "--database", "app", "--schema", "public", "--dialect", "postgresql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "does not exist on table \"users\"") {
		t.Fatalf("expected drop-column existence finding in stdout, got %q", stdout.String())
	}
	if len(client.tableSnapshotCalls) != 1 || client.tableSnapshotCalls[0].Schema != "public" || client.tableSnapshotCalls[0].Table != "users" {
		t.Fatalf("expected public.users snapshot lookup, got %#v", client.tableSnapshotCalls)
	}
}

func TestAuditCommandPostgreSQLMetadataRequiresExistingTableForRenameTable(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{
			Exists: false,
			Schema: "public",
			Table:  &spec.Table{Name: "users"},
		},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users rename to users_archive;", "--host", "127.0.0.1", "--user", "root", "--database", "app", "--schema", "public", "--dialect", "postgresql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "table \"users\" does not exist in the target schema") {
		t.Fatalf("expected alter-table existence finding in stdout, got %q", stdout.String())
	}
	if len(client.tableSnapshotCalls) != 1 || client.tableSnapshotCalls[0].Schema != "public" || client.tableSnapshotCalls[0].Table != "users" {
		t.Fatalf("expected public.users snapshot lookup, got %#v", client.tableSnapshotCalls)
	}
}
