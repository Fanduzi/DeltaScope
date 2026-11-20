//go:build postgresql

package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
)

type destructiveTargetClient struct{ *metadataAuditTestClient }

func (c *destructiveTargetClient) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	c.tableCalls = append(c.tableCalls, table)
	switch table {
	case "absent":
		return &spec.TableSnapshot{Schema: schema, PGRelation: spec.PGRelationState{Name: table}}, nil
	case "partition_root":
		return &spec.TableSnapshot{Schema: schema, Exists: true, Table: &spec.Table{Name: table}, PGRelation: spec.PGRelationState{Name: table, Kind: "p"}}, nil
	default:
		return &spec.TableSnapshot{Schema: schema, Exists: true, Table: &spec.Table{Name: table}, PGRelation: spec.PGRelationState{Name: table, Kind: "v"}}, nil
	}
}

func (c *destructiveTargetClient) ResolveObject(_ context.Context, _ spec.Dialect, request spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if request.Type == "truncate_references" && len(request.PGDestructiveTargets) > 0 {
		return &spec.ObjectSnapshot{PGDestructiveReferences: &spec.PGDestructiveReferences{Targets: request.PGDestructiveTargets, BlockedPosition: 1}}, nil
	}
	return nil, nil
}

func TestHTTPPGDestructiveTargets(t *testing.T) {
	h, err := NewHandler("", "test-build", WithRegistry(newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })))
	if err != nil {
		t.Fatal(err)
	}
	client := &destructiveTargetClient{&metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL}}
	old := prepareHTTPMetadataAudit
	t.Cleanup(func() { prepareHTTPMetadataAudit = old })
	prepareHTTPMetadataAudit = func(context.Context, auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"DROP TABLE IF EXISTS app.absent, app.v, app.v","dialect":"postgresql","connection_id":"test-pg"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-API-Key", "test-key-value")
	out := httptest.NewRecorder()
	h.ServeHTTP(out, request)
	body := out.Body.String()
	if out.Code != http.StatusOK || !strings.Contains(body, `"ddl.table.drop.exists.require"`) || strings.Count(body, `"relation_wrong_kind"`) != 1 || strings.Contains(body, "PGDestructiveTableRelations") || !slices.Equal(client.tableCalls, []string{"absent", "v"}) {
		t.Fatalf("code=%d calls=%v body=%s", out.Code, client.tableCalls, body)
	}

	request = httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"TRUNCATE app.partition_root","dialect":"postgresql","connection_id":"test-pg"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-API-Key", "test-key-value")
	out = httptest.NewRecorder()
	h.ServeHTTP(out, request)
	if out.Code != http.StatusOK || !strings.Contains(out.Body.String(), `"truncate_referenced_by_external_foreign_key"`) || strings.Contains(out.Body.String(), "IneligibleOnlyPosition") {
		t.Fatalf("code=%d body=%s", out.Code, out.Body.String())
	}
}
