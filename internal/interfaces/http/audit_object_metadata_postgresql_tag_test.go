//go:build postgresql

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
)

func TestHTTPAuditProjectsAmbiguousObjectMetadata(t *testing.T) {
	testReg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) {
		c.Dialect = "postgresql"
		c.Port = 5432
		c.Database = "app"
	})
	handler, err := NewHandler("", "test-build", WithRegistry(testReg))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		objectSnapshot: &spec.ObjectSnapshot{
			Schema:              "public",
			Type:                "publication",
			Name:                "my_pub",
			Status:              spec.MetadataStatusAmbiguous,
			Exists:              false,
			AmbiguousCandidates: []string{"app.my_pub", "public.my_pub"},
		},
	}
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "inferred",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	body := `{"sql":"CREATE PUBLICATION my_pub FOR ALL TABLES","dialect":"postgresql","connection_id":"test-pg","schema":"public"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	statements, ok := payload["statements"].([]any)
	if !ok || len(statements) == 0 {
		t.Fatalf("expected statements, got %#v", payload["statements"])
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
		if strings.HasPrefix(f["rule_id"].(string), "ddl.pg.create_publication") {
			findingMeta, _ = f["metadata"].(map[string]any)
			break
		}
	}
	if findingMeta == nil {
		t.Fatalf("expected create_publication finding, got %#v", findings)
	}

	if findingMeta["metadata_status"] != "ambiguous" {
		t.Fatalf("expected metadata_status=ambiguous, got %#v", findingMeta["metadata_status"])
	}
	if findingMeta["metadata_object_type"] != "publication" {
		t.Fatalf("expected metadata_object_type=publication, got %#v", findingMeta["metadata_object_type"])
	}
	if findingMeta["metadata_object_name"] != "my_pub" {
		t.Fatalf("expected metadata_object_name=my_pub, got %#v", findingMeta["metadata_object_name"])
	}

	candidates, ok := findingMeta["metadata_ambiguous_candidates"].([]any)
	if !ok || len(candidates) != 2 {
		t.Fatalf("expected 2 ambiguous candidates, got %#v", findingMeta["metadata_ambiguous_candidates"])
	}

	sensitiveKeys := []string{"password", "secret", "conninfo", "connection", "host", "port", "options", "query", "body", "definition", "comment", "label"}
	for _, key := range sensitiveKeys {
		if _, ok := findingMeta["metadata_"+key]; ok {
			t.Fatalf("sensitive key metadata_%q leaked into finding metadata", key)
		}
	}
}
