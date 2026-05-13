//go:build postgresql

package deltascope

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type fakeMetadataProviderWithObjects struct {
	fakeMetadataProvider
	objectSnapshot *spec.ObjectSnapshot
	objectCalls    []spec.ObjectLookupRequest
}

func (f *fakeMetadataProviderWithObjects) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	f.objectCalls = append(f.objectCalls, req)
	return f.objectSnapshot, nil
}

func TestAuditProjectsConfirmedObjectMetadataThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		objectSnapshot: &spec.ObjectSnapshot{
			Schema: "public",
			Type:   "extension",
			Name:   "pg_trgm",
			Status: spec.MetadataStatusConfirmed,
			Exists: true,
			Attributes: map[string]string{
				"extension_version": "1.6",
				"type_kind":         "base",
				"password":          "should_be_filtered",
				"comment":           "also_filtered",
			},
		},
	}

	result, err := Audit(context.Background(), Request{
		SQL:              "DROP EXTENSION IF EXISTS pg_trgm",
		Dialect:          DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	var finding *Finding
	for i := range result.Statements {
		for j := range result.Statements[i].Findings {
			if result.Statements[i].Findings[j].RuleID == "ddl.pg.drop_extension.advisory" {
				f := result.Statements[i].Findings[j]
				finding = &f
				break
			}
		}
	}
	if finding == nil {
		t.Fatalf("expected drop_extension finding, got %d statements", len(result.Statements))
	}

	meta := finding.Metadata
	if meta["metadata_status"] != "confirmed" {
		t.Fatalf("expected metadata_status=confirmed, got %#v", meta["metadata_status"])
	}
	if meta["metadata_object_type"] != "extension" {
		t.Fatalf("expected metadata_object_type=extension, got %#v", meta["metadata_object_type"])
	}
	if meta["metadata_object_name"] != "pg_trgm" {
		t.Fatalf("expected metadata_object_name=pg_trgm, got %#v", meta["metadata_object_name"])
	}
	if meta["metadata_exists"] != true {
		t.Fatalf("expected metadata_exists=true, got %#v", meta["metadata_exists"])
	}
	if meta["metadata_extension_version"] != "1.6" {
		t.Fatalf("expected metadata_extension_version=1.6, got %#v", meta["metadata_extension_version"])
	}

	sensitiveKeys := []string{"password", "secret", "conninfo", "connection", "host", "port", "options", "query", "body", "definition", "comment", "label"}
	for _, key := range sensitiveKeys {
		if _, ok := meta["metadata_"+key]; ok {
			t.Fatalf("sensitive key metadata_%q leaked into finding metadata", key)
		}
	}

	if len(provider.objectCalls) != 1 {
		t.Fatalf("expected 1 ResolveObject call, got %d", len(provider.objectCalls))
	}
	if provider.objectCalls[0].Type != "extension" || provider.objectCalls[0].Name != "pg_trgm" {
		t.Fatalf("expected lookup for extension/pg_trgm, got %#v", provider.objectCalls[0])
	}
}
