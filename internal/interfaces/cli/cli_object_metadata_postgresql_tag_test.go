//go:build postgresql

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditCommandProjectsNotFoundObjectMetadata(t *testing.T) {
	previous := newMetadataClient
	client := &fakeMetadataClient{
		detectDialect:  spec.DialectPostgreSQL,
		schemasByTable: map[string][]string{"users": {"public"}},
		objectSnapshot: &spec.ObjectSnapshot{
			Schema: "public",
			Type:   "schema",
			Name:   "old_schema",
			Status: spec.MetadataStatusNotFound,
			Exists: false,
		},
	}
	newMetadataClient = func(options auditConnectionOptions) (metadataClient, error) {
		client.options = options
		return client, nil
	}
	t.Cleanup(func() { newMetadataClient = previous })

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "DROP SCHEMA old_schema", "--host", "127.0.0.1", "--user", "root", "--dialect", "postgresql", "--schema", "public", "--format", "json"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)

	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d\nstdout=%q", exitOK, code, stdout.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v", err)
	}

	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) == 0 {
		t.Fatalf("expected statements, got %#v", decoded["statements"])
	}
	stmt, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement map, got %T", statements[0])
	}

	findings, ok := stmt["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings, got %#v", stmt["findings"])
	}

	var findingMeta map[string]any
	for _, item := range findings {
		f, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if f["rule_id"] == "ddl.pg.drop_schema.advisory" {
			findingMeta, _ = f["metadata"].(map[string]any)
			break
		}
	}
	if findingMeta == nil {
		t.Fatalf("expected drop_schema finding, got %#v", findings)
	}

	if findingMeta["metadata_status"] != "not_found" {
		t.Fatalf("expected metadata_status=not_found, got %#v", findingMeta["metadata_status"])
	}
	if findingMeta["metadata_object_type"] != "schema" {
		t.Fatalf("expected metadata_object_type=schema, got %#v", findingMeta["metadata_object_type"])
	}
	if findingMeta["metadata_object_name"] != "old_schema" {
		t.Fatalf("expected metadata_object_name=old_schema, got %#v", findingMeta["metadata_object_name"])
	}
	if findingMeta["metadata_exists"] != false {
		t.Fatalf("expected metadata_exists=false, got %#v", findingMeta["metadata_exists"])
	}

	sensitiveKeys := []string{"password", "secret", "conninfo", "connection", "host", "port", "options", "query", "body", "definition", "comment", "label"}
	for _, key := range sensitiveKeys {
		if _, ok := findingMeta["metadata_"+key]; ok {
			t.Fatalf("sensitive key metadata_%q leaked into finding metadata", key)
		}
	}
}
