//go:build postgresql

package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditCommandRendersPartialJSONForMixedUnsupportedPostgreSQL(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users add column email text; select 1", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d", exitAudit, code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output when rendering partial result, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered supported statement, got %#v", decoded["statements"])
	}
	unsupported, ok := decoded["unsupported"].([]any)
	if !ok || len(unsupported) != 1 {
		t.Fatalf("expected one unsupported detail, got %#v", decoded["unsupported"])
	}
	item, ok := unsupported[0].(map[string]any)
	if !ok {
		t.Fatalf("expected unsupported object, got %#v", unsupported[0])
	}
	if item["feature"] != "select" || item["reason"] == "" {
		t.Fatalf("expected unsupported feature and reason, got %#v", item)
	}
}

func TestAuditCommandRendersPartialMarkdownForMixedUnsupportedPostgreSQL(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users add column email text; select 1", "--dialect", "postgresql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d", exitAudit, code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output when rendering partial markdown result, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "## Unsupported Statements") {
		t.Fatalf("expected markdown unsupported section, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Statement 2") || !strings.Contains(stdout.String(), "select") {
		t.Fatalf("expected markdown unsupported statement details, got %q", stdout.String())
	}
}

func TestAuditCommandPostgreSQLOnConflictDoesNotRenderMySQLSpecificMessage(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "insert into users(id, name) values (1, 'a') on conflict (id) do update set name = excluded.name;", "--dialect", "postgresql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d\nstdout=%q\nstderr=%q", exitOK, code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "ON DUPLICATE KEY") {
		t.Fatalf("expected stdout not to contain MySQL-specific duplicate-key text, got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "INSERT ... SELECT statements are forbidden") {
		t.Fatalf("expected stdout not to contain insert-select finding, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "ON DUPLICATE KEY") {
		t.Fatalf("expected stderr not to contain MySQL-specific duplicate-key text, got %q", stderr.String())
	}
}

func TestAuditCommandPostgreSQLInsertSelectOnConflictKeepsInsertSelectRuleOnly(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "insert into users(id, name) select id, name from staging_users on conflict (id) do update set name = excluded.name;", "--dialect", "postgresql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "INSERT ... SELECT statements are forbidden") {
		t.Fatalf("expected stdout to contain insert-select finding, got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "ON DUPLICATE KEY") {
		t.Fatalf("expected stdout not to contain MySQL-specific duplicate-key text, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "ON DUPLICATE KEY") {
		t.Fatalf("expected stderr not to contain MySQL-specific duplicate-key text, got %q", stderr.String())
	}
}

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
		[]string{"audit", "--sql", "alter table users drop constraint users_pkey;", "--host", "127.0.0.1", "--user", "root", "--schema", "public", "--dialect", "postgresql"},
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
		[]string{"audit", "--sql", "alter table users rename column missing_email to email;", "--host", "127.0.0.1", "--user", "root", "--schema", "public", "--dialect", "postgresql"},
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
		[]string{"audit", "--sql", "alter table users drop column missing_email;", "--host", "127.0.0.1", "--user", "root", "--schema", "public", "--dialect", "postgresql"},
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
		[]string{"audit", "--sql", "alter table users rename to users_archive;", "--host", "127.0.0.1", "--user", "root", "--schema", "public", "--dialect", "postgresql"},
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

func TestAuditCommandPostgreSQLAlterColumnActionsRenderSemanticFindings(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users alter column created_at set default now(), alter column updated_at drop default, alter column email set not null, alter column phone drop not null;", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	counts := map[string]int{}
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ruleID, _ := finding["rule_id"].(string)
		counts[ruleID]++
	}
	if len(findings) != 8 {
		t.Fatalf("expected exactly 8 alter-column findings, got %#v", findings)
	}
	if counts["ddl.alter.set_default.explicit_default_change.forbid"] != 1 {
		t.Fatalf("expected set_default semantic finding, got %#v", findings)
	}
	if counts["ddl.alter.drop_default.explicit_default_change.forbid"] != 1 {
		t.Fatalf("expected drop_default semantic finding, got %#v", findings)
	}
	if counts["ddl.alter.set_not_null.explicit_nullability_change.forbid"] != 1 {
		t.Fatalf("expected set_not_null semantic finding, got %#v", findings)
	}
	if counts["ddl.alter.drop_not_null.explicit_nullability_change.forbid"] != 1 {
		t.Fatalf("expected drop_not_null semantic finding, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLSetDataTypeRendersForbidFinding(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users alter column status type bigint;", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitOK {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitOK, code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	counts := make(map[string]int)
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected finding object, got %#v", item)
		}
		ruleID, _ := finding["rule_id"].(string)
		counts[ruleID]++
	}
	if counts["ddl.alter.set_data_type.forbid"] != 1 {
		t.Fatalf("expected set_data_type forbid finding, got %#v", findings)
	}
	if counts["ddl.pg.alter.set_data_type.rewrite.warn"] != 1 {
		t.Fatalf("expected pg set_data_type rewrite warning, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLRenameIndexRendersForbidFinding(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter index idx_old rename to idx_new;", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 rename_index finding, got %#v", findings)
	}
	finding, ok := findings[0].(map[string]any)
	if !ok || finding["rule_id"] != "ddl.alter.rename_index.forbid" {
		t.Fatalf("expected rename_index forbid finding, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLCreateViewRendersForbidFinding(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "create view public.active_users as select id from public.users;", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 create_view finding, got %#v", findings)
	}
	finding, ok := findings[0].(map[string]any)
	if !ok || finding["rule_id"] != "ddl.view.create.forbid" {
		t.Fatalf("expected create_view forbid finding, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLDropViewRendersForbidFinding(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "drop view if exists public.active_users;", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 drop_view finding, got %#v", findings)
	}
	finding, ok := findings[0].(map[string]any)
	if !ok || finding["rule_id"] != "ddl.view.drop.forbid" {
		t.Fatalf("expected drop_view forbid finding, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLMetadataRenameIndexRendersExistenceFinding(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect: spec.DialectPostgreSQL,
		indexTable:    "users",
		snapshot: &spec.TableSnapshot{
			Exists:  true,
			Schema:  "public",
			Table:   &spec.Table{Name: "users"},
			Indexes: []spec.Index{{Name: "idx_users_email", Kind: spec.IndexKindSecondary}},
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
		[]string{"audit", "--sql", "alter index missing_idx rename to idx_new;", "--host", "127.0.0.1", "--user", "root", "--schema", "public", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	contextValue, ok := decoded["context"].(map[string]any)
	if !ok || contextValue["mode"] != "metadata-aware" {
		t.Fatalf("expected metadata-aware context, got %#v", decoded["context"])
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	found := false
	for _, raw := range findings {
		finding, ok := raw.(map[string]any)
		if ok && finding["rule_id"] == "ddl.alter.rename_index.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rename_index existence finding, got %#v", findings)
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
}

func TestAuditCommandSupportsPostgreSQLValidateConstraint(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users validate constraint chk_amount;", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitOK {
		t.Fatalf("expected exit code %d for validate constraint, got %d\nstdout=%q\nstderr=%q", exitOK, code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	if statement["kind"] != "ddl" {
		t.Fatalf("expected ddl kind, got %#v", statement["kind"])
	}
	unsupported, ok := decoded["unsupported"].([]any)
	if ok && len(unsupported) != 0 {
		t.Fatalf("expected no unsupported entries, got %#v", unsupported)
	}
}

func TestAuditCommandPostgreSQLDropNonPrimaryKeyConstraintDoesNotRenderPrimaryKeyFinding(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users drop constraint chk_amount;", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitOK {
		t.Fatalf("expected exit code %d for drop constraint non-PK, got %d\nstdout=%q\nstderr=%q", exitOK, code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, _ := statement["findings"].([]any)
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if ok && finding["rule_id"] == "ddl.alter.drop_primary_key.forbid" {
			t.Fatalf("expected no drop_primary_key finding for non-PK constraint, got %#v", finding)
		}
	}
}

func TestAuditCommandPostgreSQLSchemaQualifiedForeignKeyExposesReferencedObjectMetadata(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE TABLE orders (id bigint PRIMARY KEY, approver_id bigint, CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES public.users(id));", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) < 1 {
		t.Fatalf("expected at least one finding, got %#v", statement["findings"])
	}

	// 1. FK forbid finding must trigger.
	found := false
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.table.foreign_key.forbid" {
			found = true
			metadata, _ := finding["metadata"].(map[string]any)
			if metadata == nil {
				t.Fatalf("expected metadata map in finding, got nil")
			}

			// 2. Current metadata contains: table, constraint, columns.
			if metadata["table"] == nil {
				t.Errorf("expected metadata key 'table', got nil")
			}
			if metadata["constraint"] == nil {
				t.Errorf("expected metadata key 'constraint', got nil")
			}
			if metadata["columns"] == nil {
				t.Errorf("expected metadata key 'columns', got nil")
			}

			// v0.28.0: referenced-object metadata is now exposed.
			if metadata["referenced_schema"] == nil {
				t.Errorf("expected metadata key 'referenced_schema', got nil")
			}
			if metadata["referenced_table"] == nil {
				t.Errorf("expected metadata key 'referenced_table', got nil")
			}
			if metadata["referenced_columns"] == nil {
				t.Errorf("expected metadata key 'referenced_columns', got nil")
			}
			// referenced_table must NOT be schema-qualified.
			if refTable, _ := metadata["referenced_table"].(string); refTable == "public.users" {
				t.Fatalf("referenced_table must not be schema-qualified 'public.users'")
			}
		}
	}
	if !found {
		t.Fatalf("expected foreign_key forbid finding, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLAlterColumnSetDefaultRendersForbidFinding(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users alter column status set default 'active';", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) < 1 {
		t.Fatalf("expected at least one finding, got %#v", statement["findings"])
	}
	found := false
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if ok && finding["rule_id"] == "ddl.alter.set_default.explicit_default_change.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected set_default semantic finding, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLMetadataDropIndexRendersExistenceFinding(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect: spec.DialectPostgreSQL,
		indexTable:    "users",
		snapshot: &spec.TableSnapshot{
			Exists:  true,
			Schema:  "public",
			Table:   &spec.Table{Name: "users"},
			Indexes: []spec.Index{{Name: "idx_users_email", Kind: spec.IndexKindSecondary}},
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
		[]string{"audit", "--sql", "drop index missing_idx;", "--host", "127.0.0.1", "--user", "root", "--schema", "public", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	contextValue, ok := decoded["context"].(map[string]any)
	if !ok || contextValue["mode"] != "metadata-aware" {
		t.Fatalf("expected metadata-aware context, got %#v", decoded["context"])
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}
	found := false
	for _, raw := range findings {
		finding, ok := raw.(map[string]any)
		if ok && finding["rule_id"] == "ddl.alter.drop_index.exists.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drop_index existence finding, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLCreateTableConstraintsReturnNormalResult(t *testing.T) {
	cases := map[string]string{
		"named table-level CHECK":        "create table orders (id bigint primary key, amount numeric, constraint chk_orders_amount check (amount > 0));",
		"column-level inline CHECK":      "create table orders (id bigint primary key, amount numeric check (amount > 0));",
		"named table-level UNIQUE":       "create table users (id bigint primary key, email text, constraint uq_users_email unique (email));",
		"column-level inline UNIQUE":     "create table users (id bigint primary key, email text unique);",
		"named table-level FOREIGN KEY":  "create table orders (id bigint primary key, user_id bigint, constraint fk_orders_user foreign key (user_id) references users(id));",
		"column-level inline REFERENCES": "create table orders (id bigint primary key, user_id bigint references users(id));",
	}

	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)

			if code != exitOK && code != exitAudit {
				t.Fatalf("expected normal audit exit code, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			if statement["kind"] != "ddl" {
				t.Fatalf("expected ddl kind, got %#v", statement["kind"])
			}
			unsupported, ok := decoded["unsupported"].([]any)
			if ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported entries, got %#v", unsupported)
			}
		})
	}
}

func TestAuditCommandPostgreSQLCreateTableForeignKeyRendersForbidFinding(t *testing.T) {
	cases := map[string]string{
		"named FOREIGN KEY": "create table orders (id bigint primary key, user_id bigint, constraint bad_fk foreign key (user_id) references users(id));",
		"inline REFERENCES": "create table orders (id bigint primary key, user_id bigint references users(id));",
	}

	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)

			if code != exitAudit {
				t.Fatalf("expected audit exit code, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) < 1 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}
			found := false
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if ok && finding["rule_id"] == "ddl.table.foreign_key.forbid" {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected foreign_key forbid finding, got %#v", findings)
			}
		})
	}
}

func TestAuditCommandPostgreSQLSchemaQualifiedReferencesRenderFKFindings(t *testing.T) {
	cases := map[string]struct {
		sql            string
		wantConstraint string
		wantColumns    []string
	}{
		"inline REFERENCES public.users": {
			sql:            "create table orders (id bigint primary key, user_id bigint references public.users(id));",
			wantConstraint: "",
		},
		"named FK REFERENCES public.users": {
			sql:            "create table orders (id bigint primary key, approver_id bigint, constraint fk_orders_approver foreign key (approver_id) references public.users(id));",
			wantConstraint: "fk_orders_approver",
			wantColumns:    []string{"approver_id"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tc.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)

			if code != exitAudit {
				t.Fatalf("expected audit exit code, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) < 1 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			found := false
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if finding["rule_id"] == "ddl.table.foreign_key.forbid" {
					found = true
					meta, _ := finding["metadata"].(map[string]any)
					if meta == nil {
						t.Fatalf("expected finding metadata, got nil")
					}
					if tc.wantConstraint != "" {
						if meta["constraint"] != tc.wantConstraint {
							t.Errorf("expected constraint %q, got %q", tc.wantConstraint, meta["constraint"])
						}
					}
					if len(tc.wantColumns) > 0 {
						cols, _ := meta["columns"].([]any)
						if len(cols) != len(tc.wantColumns) {
							t.Errorf("expected %d columns, got %d", len(tc.wantColumns), len(cols))
						}
					}
					// Verify referenced_table is NOT "public.users" (schema-qualified).
					// The finding metadata does not include referenced_table, but the
					// FK forbid finding must not concatenate schema+table.
					if refTable, _ := meta["referenced_table"].(string); refTable == "public.users" {
						t.Fatalf("referenced_table must not be schema-qualified concat 'public.users', got %q", refTable)
					}
				}
			}
			if !found {
				t.Fatalf("expected foreign_key forbid finding, got %#v", findings)
			}
		})
	}
}

func TestAuditCommandPostgreSQLCrossSchemaFKRendersAdvisoryNotice(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE TABLE public.orders (id bigint PRIMARY KEY, approver_id bigint, CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id));", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) < 2 {
		t.Fatalf("expected at least two findings, got %#v", statement["findings"])
	}

	advisoryFound := false
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.pg.table.foreign_key.cross_schema.advisory" {
			advisoryFound = true
			if finding["level"] != "notice" {
				t.Errorf("expected advisory level notice, got %v", finding["level"])
			}
			meta, _ := finding["metadata"].(map[string]any)
			if meta == nil {
				t.Fatalf("expected metadata in advisory finding, got nil")
			}
			if meta["table_schema"] != "public" {
				t.Errorf("expected table_schema public, got %v", meta["table_schema"])
			}
			if meta["referenced_schema"] != "auth" {
				t.Errorf("expected referenced_schema auth, got %v", meta["referenced_schema"])
			}
			if meta["referenced_table"] != "users" {
				t.Errorf("expected referenced_table users, got %v", meta["referenced_table"])
			}
			refCols, _ := meta["referenced_columns"].([]any)
			if len(refCols) < 1 || refCols[0] != "id" {
				t.Errorf("expected referenced_columns [id], got %v", refCols)
			}
			// referenced_table must NOT be schema-qualified.
			if refTable, _ := meta["referenced_table"].(string); refTable == "auth.users" {
				t.Fatalf("referenced_table must not be schema-qualified 'auth.users'")
			}
		}
	}
	if !advisoryFound {
		t.Fatalf("expected cross-schema advisory finding, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLSameSchemaFKDoesNotRenderAdvisory(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE TABLE public.orders (id bigint PRIMARY KEY, user_id bigint, CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES public.users(id));", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}

	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if ok && finding["rule_id"] == "ddl.pg.table.foreign_key.cross_schema.advisory" {
			t.Fatalf("expected no cross-schema advisory for same-schema FK, got %#v", finding)
		}
	}
}

func TestAuditCommandPostgreSQLBareFKDoesNotRenderAdvisory(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE TABLE public.orders (id bigint PRIMARY KEY, approver_id bigint REFERENCES users(id));", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings array, got %#v", statement["findings"])
	}

	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if ok && finding["rule_id"] == "ddl.pg.table.foreign_key.cross_schema.advisory" {
			t.Fatalf("expected no cross-schema advisory for bare FK reference, got %#v", finding)
		}
	}
}

func TestAuditCommandPostgreSQLCreateTableBoundaryReturnsUnsupported(t *testing.T) {
	cases := map[string]struct {
		sql     string
		feature string
	}{
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
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tc.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)

			if code != exitAudit {
				t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			unsupported, ok := decoded["unsupported"].([]any)
			if !ok || len(unsupported) != 1 {
				t.Fatalf("expected one unsupported detail, got %#v", decoded["unsupported"])
			}
			item, ok := unsupported[0].(map[string]any)
			if !ok {
				t.Fatalf("expected unsupported object, got %#v", unsupported[0])
			}
			if item["feature"] != tc.feature {
				t.Fatalf("expected unsupported feature %q, got %q", tc.feature, item["feature"])
			}
			if item["reason"] == "" {
				t.Fatalf("expected unsupported reason, got %#v", item)
			}
		})
	}
}

func TestAuditCommandPostgreSQLAlterTableGeneratedIdentityStateTransitionsNowSupported(t *testing.T) {
	cases := map[string]struct {
		sql string
	}{
		"drop generated expression": {
			sql: `ALTER TABLE users
  ALTER COLUMN full_name DROP EXPRESSION;`,
		},
		"set identity generated always": {
			sql: `ALTER TABLE users
  ALTER COLUMN id SET GENERATED ALWAYS;`,
		},
		"set identity generated by default": {
			sql: `ALTER TABLE users
  ALTER COLUMN id SET GENERATED BY DEFAULT;`,
		},
		"drop identity": {
			sql: `ALTER TABLE users
  ALTER COLUMN id DROP IDENTITY;`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tc.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)

			if code != exitOK {
				t.Fatalf("expected exit code %d, got %d\nstdout=%q\nstderr=%q", exitOK, code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			contextValue, ok := decoded["context"].(map[string]any)
			if !ok || contextValue["mode"] != "offline" || contextValue["dialect"] != "postgresql" {
				t.Fatalf("expected offline postgresql context, got %#v", decoded["context"])
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			if statement["kind"] != "ddl" {
				t.Fatalf("expected ddl kind, got %#v", statement["kind"])
			}
			unsupported, ok := decoded["unsupported"].([]any)
			if ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported entries, got %#v", unsupported)
			}
		})
	}
}

func TestAuditCommandPostgreSQLGeneratedIdentityRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "drop_expression forbid",
			sql:        "ALTER TABLE users ALTER COLUMN full_name DROP EXPRESSION",
			wantRuleID: "ddl.alter.drop_expression.forbid",
		},
		{
			name:       "set_generated_by_default forbid",
			sql:        "ALTER TABLE users ALTER COLUMN id SET GENERATED BY DEFAULT",
			wantRuleID: "ddl.alter.set_generated.forbid",
		},
		{
			name:       "set_generated_always forbid",
			sql:        "ALTER TABLE users ALTER COLUMN id SET GENERATED ALWAYS",
			wantRuleID: "ddl.alter.set_generated.forbid",
		},
		{
			name:       "drop_identity forbid",
			sql:        "ALTER TABLE users ALTER COLUMN id DROP IDENTITY",
			wantRuleID: "ddl.alter.drop_identity.forbid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)

			if code != exitOK {
				t.Fatalf("expected exit code %d, got %d\nstdout=%q\nstderr=%q", exitOK, code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			contextValue, ok := decoded["context"].(map[string]any)
			if !ok || contextValue["mode"] != "offline" || contextValue["dialect"] != "postgresql" {
				t.Fatalf("expected offline postgresql context, got %#v", decoded["context"])
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			if statement["kind"] != "ddl" {
				t.Fatalf("expected ddl kind, got %#v", statement["kind"])
			}
			unsupported, ok := decoded["unsupported"].([]any)
			if ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported entries, got %#v", unsupported)
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) == 0 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}
			found := false
			for _, f := range findings {
				finding, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if finding["rule_id"] == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule_id %s, got %#v", tt.wantRuleID, findings)
			}
		})
	}
}

func TestAuditCommandPostgreSQLAlterTableAddGeneratedIdentityNarrowNowSupported(t *testing.T) {
	cases := map[string]struct {
		sql string
	}{
		"generated stored add-column": {
			sql: "ALTER TABLE users ADD COLUMN full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED;",
		},
		"identity add-column": {
			sql: "ALTER TABLE users ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tc.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)

			if code != 0 {
				t.Fatalf("expected exit code 0 (supported pass), got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			unsupported, _ := decoded["unsupported"].([]any)
			if len(unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected 1 statement, got %#v", decoded["statements"])
			}
		})
	}
}

// TestAuditCommandPostgreSQLGeneratedIdentityNarrowNowSupported proves that narrow
// generated/identity forms are now processed through the normal supported CLI path.
// Each case asserts: exitAudit or 0 (supported path), no stderr, empty unsupported array, one statement.
func TestAuditCommandPostgreSQLGeneratedIdentityNarrowNowSupported(t *testing.T) {
	cases := map[string]struct {
		sql string
	}{
		"generated_stored_column": {
			sql: `CREATE TABLE t (first_name text, full_name text GENERATED ALWAYS AS (first_name) STORED);`,
		},
		"generated_always_as_identity": {
			sql: `CREATE TABLE t (id bigint GENERATED ALWAYS AS IDENTITY);`,
		},
		"generated_by_default_identity_with_options": {
			sql: `CREATE TABLE t (id bigint GENERATED BY DEFAULT AS IDENTITY (START WITH 10 INCREMENT BY 5 CACHE 20 CYCLE));`,
		},
		"alter_table_add_generated_column": {
			sql: `ALTER TABLE t ADD COLUMN full_name text GENERATED ALWAYS AS (first_name) STORED;`,
		},
		"alter_table_add_identity_column": {
			sql: `ALTER TABLE t ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tc.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)

			if code != exitAudit && code != 0 {
				t.Fatalf("expected exitAudit or 0 (supported path), got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			unsupported, _ := decoded["unsupported"].([]any)
			if len(unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected 1 statement, got %#v", decoded["statements"])
			}
		})
	}
}

// cliMetadataValueEqual compares values with numeric type coercion for JSON-decoded output.
func TestAuditCommandPostgreSQLPrimaryKeyRuleCoverage(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE TABLE bad_pk_type (id integer PRIMARY KEY, name text);", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("expected at least one finding, got %#v", statement["findings"])
	}
	found := false
	for _, f := range findings {
		finding, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.table.primary_key.bigint.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding with rule_id ddl.table.primary_key.bigint.require, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLUniqueIndexRuleCoverage(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE UNIQUE INDEX bad_email_unique ON users (email);", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("expected at least one finding, got %#v", statement["findings"])
	}
	found := false
	for _, f := range findings {
		finding, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.index.unique.prefix.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding with rule_id ddl.index.unique.prefix.require, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLAlterTableAddConstraintRuleCoverage(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "ALTER TABLE users ADD CONSTRAINT bad_email_key UNIQUE (email);", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("expected at least one finding, got %#v", statement["findings"])
	}
	found := false
	for _, f := range findings {
		finding, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.alter.add_index.unique.prefix.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding with rule_id ddl.alter.add_index.unique.prefix.require, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLAlterTableForeignKeyRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "forbid only",
			sql:        "ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id);",
			wantRuleID: "ddl.table.foreign_key.forbid",
		},
		{
			name:       "cross_schema advisory",
			sql:        "ALTER TABLE public.orders ADD CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id);",
			wantRuleID: "ddl.pg.table.foreign_key.cross_schema.advisory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)

			if code != exitAudit {
				t.Fatalf("expected audit exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) == 0 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}
			found := false
			for _, f := range findings {
				finding, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if finding["rule_id"] == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule_id %s, got %#v", tt.wantRuleID, findings)
			}
			unsupported, ok := decoded["unsupported"].([]any)
			if ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported entries, got %#v", unsupported)
			}
		})
	}
}

func TestAuditCommandPostgreSQLAlterTableAddConstraintCheckRuleCoverage(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(configPath, []byte("rules:\n  ddl.constraint.check.name.prefix.require:\n    enabled: true\n    params:\n      prefix: ck_\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0);", "--dialect", "postgresql", "--format", "json", "--config", configPath},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("expected at least one finding, got %#v", statement["findings"])
	}

	wantRuleIDs := map[string]bool{
		"ddl.constraint.check.name.prefix.require": false,
		"ddl.pg.alter.add_check.not_valid.require": false,
	}
	for _, f := range findings {
		finding, ok := f.(map[string]any)
		if !ok {
			continue
		}
		ruleID, _ := finding["rule_id"].(string)
		if _, expected := wantRuleIDs[ruleID]; expected {
			wantRuleIDs[ruleID] = true
		}
	}
	for ruleID, found := range wantRuleIDs {
		if !found {
			t.Fatalf("expected finding with rule_id %s, got %#v", ruleID, findings)
		}
	}
}

func cliMetadataValueEqual(a, b any) bool {
	aFloat, aIsNum := cliToFloat64(a)
	bFloat, bIsNum := cliToFloat64(b)
	if aIsNum && bIsNum {
		return aFloat == bFloat
	}
	return a == b
}

func TestAuditCommandPostgreSQLNotValidConstraintValidationRuleCoverage(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d\nstdout=%q\nstderr=%q", exitOK, code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	globalFindings, ok := decoded["global_findings"].([]any)
	if !ok || len(globalFindings) == 0 {
		t.Fatalf("expected at least one global finding, got %#v", decoded["global_findings"])
	}
	found := false
	for _, f := range globalFindings {
		finding, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.pg.alter.not_valid_constraint.validate.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected global finding with rule_id ddl.pg.alter.not_valid_constraint.validate.require, got %#v", globalFindings)
	}
	unsupported, ok := decoded["unsupported"].([]any)
	if ok && len(unsupported) != 0 {
		t.Fatalf("expected no unsupported entries, got %#v", unsupported)
	}
}

func TestAuditCommandDefaultPolicyDialectHygienePostgreSQLExcludesMySQLFamilyRules(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE TABLE pg_smoke (id bigint primary key);", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)
	_ = code
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	mysqlOnly := []string{
		"ddl.table.engine.allowlist",
		"ddl.table.charset.allowlist",
		"ddl.table.row_format.allowlist",
		"ddl.table.auto_increment_init.value",
		"ddl.primary_key.unsigned.require",
		"ddl.primary_key.auto_increment.require",
		"ddl.primary_key.not_null.require",
	}
	statements, ok := decoded["statements"].([]any)
	if !ok {
		t.Fatalf("expected statements array, got %#v", decoded["statements"])
	}
	for _, rawStmt := range statements {
		stmt, ok := rawStmt.(map[string]any)
		if !ok {
			continue
		}
		findings, ok := stmt["findings"].([]any)
		if !ok {
			continue
		}
		for _, rawFinding := range findings {
			finding, ok := rawFinding.(map[string]any)
			if !ok {
				continue
			}
			ruleID, _ := finding["rule_id"].(string)
			for _, forbidden := range mysqlOnly {
				if ruleID == forbidden {
					t.Errorf("PG default CLI audit should not emit MySQL-only rule %q", forbidden)
				}
			}
			message, _ := finding["message"].(string)
			suggestion, _ := finding["suggestion"].(string)
			combined := strings.ToUpper(message + " " + suggestion)
			for _, pattern := range []string{"UNSIGNED", "AUTO_INCREMENT", "ON UPDATE CURRENT_TIMESTAMP"} {
				if strings.Contains(combined, pattern) {
					t.Errorf("PG default CLI audit should not contain MySQL-specific text %q", pattern)
				}
			}
		}
	}
	globalFindings, ok := decoded["global_findings"].([]any)
	if !ok {
		return
	}
	for _, rawFinding := range globalFindings {
		finding, ok := rawFinding.(map[string]any)
		if !ok {
			continue
		}
		ruleID, _ := finding["rule_id"].(string)
		for _, forbidden := range mysqlOnly {
			if ruleID == forbidden {
				t.Errorf("PG default CLI audit should not emit MySQL-only rule %q in global findings", forbidden)
			}
		}
	}
}

func cliToFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

func TestAuditCommandPostgreSQLGitLabCodeQualityRendersGlobalFinding(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;", "--dialect", "postgresql", "--format", "gitlab-codequality", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d, stderr=%s", code, stderr.String())
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &issues); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, stdout.String())
	}

	hasNotValid := false
	for _, issue := range issues {
		cn, _ := issue["check_name"].(string)
		if cn == "ddl.pg.alter.not_valid_constraint.validate.require" {
			hasNotValid = true
			loc, _ := issue["location"].(map[string]any)
			if loc == nil {
				t.Fatal("global finding missing location")
			}
			if loc["path"] != "deltascope.sql" {
				t.Errorf("location.path = %v, want deltascope.sql", loc["path"])
			}
			sev, _ := issue["severity"].(string)
			if sev == "" {
				t.Error("severity is empty")
			}
			fp, _ := issue["fingerprint"].(string)
			if len(fp) != 64 {
				t.Errorf("fingerprint length = %d, want 64", len(fp))
			}
			break
		}
	}
	if !hasNotValid {
		t.Errorf("expected ddl.pg.alter.not_valid_constraint.validate.require in issues, got check_names: %v", func() []string {
			var names []string
			for _, issue := range issues {
				names = append(names, issue["check_name"].(string))
			}
			return names
		}())
	}
}

const locationFidelityPGMultiStmtSQL = `create table ok_users (
  id bigint primary key
);

delete from users;`

// TestLocationFidelityPostgreSQLGitHubActionsFileAndLine verifies that --format
// github-actions with --file and --dialect postgresql outputs the file path
// and real statement-start line number.
func TestLocationFidelityPostgreSQLGitHubActionsFileAndLine(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "migrations.sql")
	if err := os.WriteFile(sqlPath, []byte(locationFidelityPGMultiStmtSQL), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--dialect", "postgresql", "--file", sqlPath, "--format", "github-actions", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "dml.where.require") {
		t.Fatalf("expected dml.where.require in output, got: %s", output)
	}

	if !strings.Contains(output, "file="+filepath.ToSlash(sqlPath)) {
		t.Errorf("expected file=%s in annotation, got: %s", filepath.ToSlash(sqlPath), output)
	}
	// "delete from users;" starts on line 5 in locationFidelityPGMultiStmtSQL.
	if !strings.Contains(output, "line=5") {
		t.Errorf("expected line=5 (delete statement start) in annotation, got: %s", output)
	}
}

// TestLocationFidelityPostgreSQLSARIFArtifactURIAndLine verifies that --format sarif
// with --file and --dialect postgresql outputs artifactLocation.uri and real line numbers.
func TestLocationFidelityPostgreSQLSARIFArtifactURIAndLine(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "migrations.sql")
	if err := os.WriteFile(sqlPath, []byte(locationFidelityPGMultiStmtSQL), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--dialect", "postgresql", "--file", sqlPath, "--format", "sarif", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d", code)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal sarif: %v\noutput=%s", err, stdout.String())
	}

	runs, ok := decoded["runs"].([]any)
	if !ok || len(runs) == 0 {
		t.Fatal("expected runs array in SARIF output")
	}
	run, _ := runs[0].(map[string]any)
	results, _ := run["results"].([]any)

	var whereResult map[string]any
	for _, r := range results {
		result, _ := r.(map[string]any)
		if result["ruleId"] == "dml.where.require" {
			whereResult = result
			break
		}
	}
	if whereResult == nil {
		t.Fatal("expected dml.where.require result in SARIF")
	}

	locations, _ := whereResult["locations"].([]any)
	if len(locations) == 0 {
		t.Fatal("expected locations array in dml.where.require result")
	}

	loc, _ := locations[0].(map[string]any)
	phys, _ := loc["physicalLocation"].(map[string]any)
	if phys == nil {
		t.Fatal("expected physicalLocation in SARIF location")
	}

	artifact, _ := phys["artifactLocation"].(map[string]any)
	if artifact == nil {
		t.Fatal("expected artifactLocation in SARIF physicalLocation")
	}
	uri, _ := artifact["uri"].(string)
	if uri == "" {
		t.Error("expected artifactLocation.uri to be populated")
	}

	region, _ := phys["region"].(map[string]any)
	startLine, _ := region["startLine"].(float64)
	// "delete from users;" starts on line 5 in locationFidelityPGMultiStmtSQL.
	if startLine != 5 {
		t.Errorf("expected startLine=5 (delete statement start), got %v", startLine)
	}
}

// TestLocationFidelityPostgreSQLGitLabCodeQualityLineReal verifies that --format
// gitlab-codequality with --file and --dialect postgresql preserves location.path
// and uses real statement-start line numbers.
func TestLocationFidelityPostgreSQLGitLabCodeQualityLineReal(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "migrations.sql")
	if err := os.WriteFile(sqlPath, []byte(locationFidelityPGMultiStmtSQL), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--dialect", "postgresql", "--file", sqlPath, "--format", "gitlab-codequality", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d", code)
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &issues); err != nil {
		t.Fatalf("unmarshal gitlab-codequality: %v\noutput=%s", err, stdout.String())
	}

	var whereIssue map[string]any
	for _, issue := range issues {
		if issue["check_name"] == "dml.where.require" {
			whereIssue = issue
			break
		}
	}
	if whereIssue == nil {
		t.Fatal("expected dml.where.require issue")
	}

	loc, _ := whereIssue["location"].(map[string]any)
	path, _ := loc["path"].(string)
	lines, _ := loc["lines"].(map[string]any)
	begin := lines["begin"]

	if path == "" {
		t.Fatal("expected location.path to be populated from --file")
	}

	beginFloat, _ := begin.(float64)
	// "delete from users;" starts on line 5 in locationFidelityPGMultiStmtSQL.
	if beginFloat != 5 {
		t.Errorf("expected lines.begin=5 (delete statement start line), got %v", begin)
	}
}

func TestAuditCommandPostgreSQLAdvancedIndexFormsSupportedAndCovered(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE INDEX idx_users_active_email ON users (email) WHERE active = true", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}

	if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
		t.Fatalf("expected no unsupported details, got %#v", unsupported)
	}

	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("expected at least one finding, got %#v", statement["findings"])
	}

	found := false
	for _, f := range findings {
		finding, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.pg.create_index.concurrently.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding with rule_id ddl.pg.create_index.concurrently.require, got %#v", findings)
	}
}

func TestAuditCommandPostgreSQLObjectLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "drop_schema_advisory",
			sql:        "DROP SCHEMA IF EXISTS staging;",
			wantRuleID: "ddl.pg.drop_schema.advisory",
		},
		{
			name:       "drop_schema_cascade_warn",
			sql:        "DROP SCHEMA IF EXISTS staging CASCADE;",
			wantRuleID: "ddl.pg.drop_schema.cascade.warn",
		},
		{
			name:       "alter_sequence_restart_warn",
			sql:        "ALTER SEQUENCE seq_order_id RESTART WITH 100;",
			wantRuleID: "ddl.pg.alter_sequence.restart.warn",
		},
		{
			name:       "drop_sequence_advisory",
			sql:        "DROP SEQUENCE IF EXISTS seq_order_id;",
			wantRuleID: "ddl.pg.drop_sequence.advisory",
		},
		{
			name:       "drop_materialized_view_advisory",
			sql:        "DROP MATERIALIZED VIEW IF EXISTS mv_stats;",
			wantRuleID: "ddl.pg.drop_materialized_view.advisory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) == 0 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			found := false
			for _, f := range findings {
				finding, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if finding["rule_id"] == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule_id %s, got %#v", tt.wantRuleID, findings)
			}
		})
	}
}

func TestAuditCommandPostgreSQLAlterTableGapRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "drop_column_advisory",
			sql:        "ALTER TABLE users DROP COLUMN email;",
			wantRuleID: "ddl.pg.alter.drop_column.advisory",
		},
		{
			name:       "validate_constraint_advisory",
			sql:        "ALTER TABLE users VALIDATE CONSTRAINT chk_price;",
			wantRuleID: "ddl.pg.alter.validate_constraint.advisory",
		},
		{
			name:       "add_column_nullable_notice",
			sql:        "ALTER TABLE users ADD COLUMN bio text;",
			wantRuleID: "ddl.pg.alter.add_column.nullable.notice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) == 0 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			found := false
			for _, f := range findings {
				finding, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if finding["rule_id"] == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule_id %s, got %#v", tt.wantRuleID, findings)
			}
		})
	}
}

func TestAuditCommandPostgreSQLRefreshMaterializedViewRuleCoverage(t *testing.T) {
	t.Run("basic_refresh_concurrently_warn", func(t *testing.T) {
		stdout := &strings.Builder{}
		stderr := &strings.Builder{}

		code := Execute(
			context.Background(),
			[]string{"audit", "--sql", "REFRESH MATERIALIZED VIEW mv_stats;", "--dialect", "postgresql", "--format", "json"},
			strings.NewReader(""),
			stdout,
			stderr,
		)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
		}

		var decoded map[string]any
		if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
			t.Fatalf("unmarshal json output: %v", err)
		}
		if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
			t.Fatalf("expected no unsupported, got %#v", unsupported)
		}
		statements, ok := decoded["statements"].([]any)
		if !ok || len(statements) != 1 {
			t.Fatalf("expected one statement, got %#v", decoded["statements"])
		}
		statement, ok := statements[0].(map[string]any)
		if !ok {
			t.Fatalf("expected statement object, got %#v", statements[0])
		}
		findings, ok := statement["findings"].([]any)
		if !ok || len(findings) == 0 {
			t.Fatalf("expected findings, got %#v", statement["findings"])
		}
		found := false
		for _, f := range findings {
			finding, ok := f.(map[string]any)
			if !ok {
				continue
			}
			if finding["rule_id"] == "ddl.pg.refresh_materialized_view.concurrently.warn" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected concurrently.warn, got %#v", findings)
		}
	})

	t.Run("with_no_data_both_rules", func(t *testing.T) {
		stdout := &strings.Builder{}
		stderr := &strings.Builder{}

		code := Execute(
			context.Background(),
			[]string{"audit", "--sql", "REFRESH MATERIALIZED VIEW mv_stats WITH NO DATA;", "--dialect", "postgresql", "--format", "json"},
			strings.NewReader(""),
			stdout,
			stderr,
		)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
		}

		var decoded map[string]any
		if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
			t.Fatalf("unmarshal json output: %v", err)
		}
		statements, ok := decoded["statements"].([]any)
		if !ok || len(statements) != 1 {
			t.Fatalf("expected one statement, got %#v", decoded["statements"])
		}
		statement, ok := statements[0].(map[string]any)
		if !ok {
			t.Fatalf("expected statement object, got %#v", statements[0])
		}
		findings, ok := statement["findings"].([]any)
		if !ok || len(findings) < 2 {
			t.Fatalf("expected at least 2 findings, got %#v", statement["findings"])
		}
		var foundConcurrent, foundNoData bool
		for _, f := range findings {
			finding, ok := f.(map[string]any)
			if !ok {
				continue
			}
			if finding["rule_id"] == "ddl.pg.refresh_materialized_view.concurrently.warn" {
				foundConcurrent = true
			}
			if finding["rule_id"] == "ddl.pg.refresh_materialized_view.no_data.notice" {
				foundNoData = true
			}
		}
		if !foundConcurrent {
			t.Fatalf("expected concurrently.warn, got %#v", findings)
		}
		if !foundNoData {
			t.Fatalf("expected no_data.notice, got %#v", findings)
		}
	})
}

func TestAuditCommandPostgreSQLAlterTableUnsupportedActionRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "set_schema_advisory",
			sql:        "ALTER TABLE users SET SCHEMA archive;",
			wantRuleID: "ddl.pg.alter.set_schema.advisory",
		},
		{
			name:       "disable_trigger_warn",
			sql:        "ALTER TABLE users DISABLE TRIGGER trg_users_audit;",
			wantRuleID: "ddl.pg.alter.disable_trigger.warn",
		},
		{
			name:       "disable_trigger_all_warn",
			sql:        "ALTER TABLE users DISABLE TRIGGER ALL;",
			wantRuleID: "ddl.pg.alter.disable_trigger.warn",
		},
		{
			name:       "replica_identity_full_warn",
			sql:        "ALTER TABLE users REPLICA IDENTITY FULL;",
			wantRuleID: "ddl.pg.alter.replica_identity_full.warn",
		},
		{
			name:       "replica_identity_using_index_notice",
			sql:        "ALTER TABLE users REPLICA IDENTITY USING INDEX users_pkey;",
			wantRuleID: "ddl.pg.alter.replica_identity_using_index.notice",
		},
		{
			name:       "detach_partition_warn",
			sql:        "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04;",
			wantRuleID: "ddl.pg.alter.detach_partition.warn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) == 0 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			found := false
			for _, f := range findings {
				finding, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if finding["rule_id"] == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule_id %s, got %#v", tt.wantRuleID, findings)
			}
		})
	}
}

func TestAuditCommandPostgreSQLTypeLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "create_type_enum_notice",
			sql:         "CREATE TYPE color AS ENUM ('red', 'green', 'blue');",
			wantRuleIDs: []string{"ddl.pg.create_type.enum.notice"},
		},
		{
			name:        "alter_type_add_value_with_position",
			sql:         "ALTER TYPE color ADD VALUE 'yellow' AFTER 'green';",
			wantRuleIDs: []string{"ddl.pg.alter_type.add_value.advisory", "ddl.pg.alter_type.add_value.position.notice"},
		},
		{
			name:        "drop_type_cascade",
			sql:         "DROP TYPE IF EXISTS color CASCADE;",
			wantRuleIDs: []string{"ddl.pg.drop_type.advisory", "ddl.pg.drop_type.cascade.warn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			stderr := &strings.Builder{}

			code := Execute(
				context.Background(),
				[]string{"audit", "--sql", tt.sql, "--dialect", "postgresql", "--format", "json"},
				strings.NewReader(""),
				stdout,
				stderr,
			)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout=%q\nstderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
			}
			if unsupported, ok := decoded["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := decoded["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one rendered statement, got %#v", decoded["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) == 0 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
			}

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, f := range findings {
				finding, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if _, expected := wantRuleIDs[finding["rule_id"].(string)]; expected {
					wantRuleIDs[finding["rule_id"].(string)] = true
				}
			}
			for ruleID, found := range wantRuleIDs {
				if !found {
					t.Fatalf("expected finding with rule_id %s, got %#v", ruleID, findings)
				}
			}
		})
	}
}
