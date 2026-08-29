//go:build postgresql

// Package cli verifies PostgreSQL-specific CLI constraint and index metadata audit behavior.
// input: PostgreSQL audit arguments and fake metadata clients
// output: PostgreSQL metadata finding and connection-contract coverage
// pos: interface-layer PostgreSQL-tagged CLI audit tests
// note: if this file changes, update this header and module README.md.
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
		[]string{"audit", "--sql", "drop index missing_idx;", "--host", "127.0.0.1", "--user", "root", "--database", "app", "--schema", "public", "--dialect", "postgresql", "--format", "json"},
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
