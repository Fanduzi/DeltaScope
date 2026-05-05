// Package httpapi verifies HTTP request binding and response mapping.
// input: synthetic HTTP requests against the DeltaScope HTTP adapter
// output: focused coverage for health, version, rule/capability discovery, audit success, and structured error responses
// pos: interface adapter test coverage for the HTTP service surface
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	ifaceconn "github.com/Fanduzi/DeltaScope/internal/interfaces/metadata"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func TestHandlerHealthz(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() == "" {
		t.Fatalf("expected health body")
	}
}

func TestHandlerVersion(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("test-build")) {
		t.Fatalf("expected version payload, got %q", rec.Body.String())
	}
}

func TestHandlerRulesListReturnsCatalogJSON(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/rules", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) == 0 {
		t.Fatalf("expected non-empty rules array, got %#v", payload["rules"])
	}
	if count, ok := payload["count"].(float64); !ok || int(count) != len(rules) {
		t.Fatalf("expected count to match rules length, got %#v", payload["count"])
	}
	firstRule, ok := rules[0].(map[string]any)
	if !ok || firstRule["rule_id"] == "" {
		t.Fatalf("expected first rule object with rule_id, got %#v", rules[0])
	}
}

func TestHandlerRulesSearchFiltersByQuery(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/rules?query=where", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["query"] != "where" {
		t.Fatalf("expected query echo, got %#v", payload["query"])
	}
	rules, ok := payload["rules"].([]any)
	if !ok || len(rules) == 0 {
		t.Fatalf("expected filtered rules array, got %#v", payload["rules"])
	}
	for _, item := range rules {
		ruleValue, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected rule object, got %#v", item)
		}
		text := strings.ToLower(ruleValue["rule_id"].(string) + " " + ruleValue["summary"].(string) + " " + ruleValue["description"].(string))
		if !strings.Contains(text, "where") {
			t.Fatalf("expected filtered rule to match query, got %#v", ruleValue)
		}
	}
}

func TestHandlerRuleShowReturnsOneRule(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/rules/dml.where.require", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["rule_id"] != "dml.where.require" {
		t.Fatalf("expected requested rule_id, got %#v", payload["rule_id"])
	}
	if payload["summary"] == "" {
		t.Fatalf("expected rule summary, got %#v", payload)
	}
}

func TestHandlerRuleShowReturnsNotFoundForUnknownRule(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/rules/not.real.rule", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"not_found"`)) {
		t.Fatalf("expected not_found error code, got %q", rec.Body.String())
	}
}

func TestHandlerCapabilitiesReturnsSurfaceSummary(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/capabilities", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["transport"] != "http" {
		t.Fatalf("expected http transport, got %#v", payload["transport"])
	}
	endpoints, ok := payload["endpoints"].([]any)
	if !ok || len(endpoints) == 0 {
		t.Fatalf("expected endpoints array, got %#v", payload["endpoints"])
	}
	expected := map[string]bool{
		"POST /v1/audit":          false,
		"GET /v1/rules":           false,
		"GET /v1/rules/{rule_id}": false,
		"GET /v1/capabilities":    false,
	}
	for _, item := range endpoints {
		value, ok := item.(string)
		if ok {
			if _, exists := expected[value]; exists {
				expected[value] = true
			}
		}
	}
	for endpoint, seen := range expected {
		if !seen {
			t.Fatalf("expected endpoint %q in capabilities payload, got %#v", endpoint, endpoints)
		}
	}
	dialects, ok := payload["dialects"].([]any)
	if !ok {
		t.Fatalf("expected dialects array, got %#v", payload["dialects"])
	}
	foundPostgreSQL := false
	for _, item := range dialects {
		if item == "postgresql" {
			foundPostgreSQL = true
			break
		}
	}
	if !foundPostgreSQL {
		t.Fatalf("expected capabilities payload to include postgresql, got %#v", dialects)
	}
}

func TestHandlerAuditReturnsJSONResult(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["verdict"] == "" {
		t.Fatalf("expected verdict in response, got %+v", payload)
	}
	contextValue, ok := payload["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object, got %#v", payload["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("expected offline mode, got %#v", contextValue["mode"])
	}
	statements, ok := payload["statements"].([]any)
	if !ok || len(statements) == 0 {
		t.Fatalf("expected statements array, got %#v", payload["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first statement object, got %#v", statements[0])
	}
	impact, ok := statement["impact"].(map[string]any)
	if !ok {
		t.Fatalf("expected statement impact object, got %#v", statement["impact"])
	}
	if impact["risk_level"] != "high" {
		t.Fatalf("expected high risk level, got %#v", impact["risk_level"])
	}
	if impact["confidence"] != "high" {
		t.Fatalf("expected high confidence, got %#v", impact["confidence"])
	}
	if impact["source"] != "shape" {
		t.Fatalf("expected shape source, got %#v", impact["source"])
	}
	if ratio, ok := impact["estimated_ratio"].(float64); !ok || ratio != 1 {
		t.Fatalf("expected estimated_ratio 1, got %#v", impact["estimated_ratio"])
	}
	reasonCodes, ok := impact["reason_codes"].([]any)
	if !ok || len(reasonCodes) != 1 || reasonCodes[0] != "missing_where" {
		t.Fatalf("expected missing_where reason code, got %#v", impact["reason_codes"])
	}
}

func TestHandlerAuditReturnsMetadataAwareContextForDirectConnection(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		if request.Connection.Host != "127.0.0.1" || request.Connection.User != "root" || request.Connection.Password != "secret" {
			t.Fatalf("unexpected connection config: %#v", request.Connection)
		}
		return &auditmeta.PreparedAudit{
			Client:        &metadataAuditTestClient{},
			Dialect:       spec.DialectMySQL,
			Schema:        "app",
			DialectSource: "detected",
			SchemaSource:  "request",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	handler, err := NewHandler("", "test-build", WithAuditFunc(func(_ context.Context, request deltascope.Request) (deltascope.Result, error) {
		if request.Schema != "app" {
			t.Fatalf("expected schema app, got %#v", request.Schema)
		}
		if request.MetadataProvider == nil {
			t.Fatalf("expected metadata provider")
		}
		return deltascope.Result{Verdict: deltascope.VerdictReject}, nil
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","connection":{"host":"127.0.0.1","port":3306,"user":"root","password":"secret","schema":"app"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	contextValue, ok := payload["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object, got %#v", payload["context"])
	}
	if contextValue["mode"] != "metadata-aware" {
		t.Fatalf("expected metadata-aware mode, got %#v", contextValue["mode"])
	}
	if contextValue["metadata_source"] != "direct" {
		t.Fatalf("expected direct metadata source, got %#v", contextValue["metadata_source"])
	}
}

func TestHandlerAuditRejectsExplicitPostgreSQLMetadataAwareRequestsOnUnsupportedBuild(t *testing.T) {
	if _, err := appaudit.Parse("SELECT 1", spec.DialectPostgreSQL); err == nil {
		t.Skip("skipping: real PG parser available, capability boundary test requires stub build")
	}
	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{detectDialect: spec.DialectPostgreSQL}
	prepareHTTPMetadataAudit = func(ctx context.Context, request auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return auditmeta.Prepare(ctx, auditmeta.Request{
			SQL:                  request.SQL,
			Connection:           request.Connection,
			RequestedDialect:     request.RequestedDialect,
			ExplicitDialect:      request.ExplicitDialect,
			ExplicitSchema:       request.ExplicitSchema,
			ExplicitSchemaSource: request.ExplicitSchemaSource,
			SchemaHint:           request.SchemaHint,
			OpenClient: func(auditmeta.ConnectionConfig) (auditmeta.Client, error) {
				return client, nil
			},
		})
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"insert into users(id) values (1) returning id;","dialect":"postgresql","connection":{"host":"127.0.0.1","port":5432,"user":"root","password":"secret"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	errPayload, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error envelope, got %#v", payload)
	}
	message, ok := errPayload["message"].(string)
	if !ok || message == "" {
		t.Fatalf("expected non-empty error message, got %#v", errPayload["message"])
	}
	if !strings.Contains(message, "PG-capable") {
		t.Fatalf("expected capability-boundary wording, got %q", message)
	}
	if strings.Contains(strings.ToLower(message), "resolve schema targets:") {
		t.Fatalf("did not expect metadata parse wrapper wording, got %q", message)
	}
	if strings.Contains(strings.ToLower(message), "possible dialect mismatch") {
		t.Fatalf("did not expect mismatch wording, got %q", message)
	}
	if strings.Contains(strings.ToLower(message), "if you are auditing postgresql") {
		t.Fatalf("did not expect heuristic suggestion wording, got %q", message)
	}
}

func TestHandlerAuditAcceptsPostgreSQLOfflineRequests(t *testing.T) {
	var captured deltascope.Request

	handler, err := NewHandler("", "test-build", WithAuditFunc(func(_ context.Context, request deltascope.Request) (deltascope.Result, error) {
		captured = request
		return deltascope.Result{Verdict: deltascope.VerdictPass}, nil
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"drop index idx_name;","dialect":"postgresql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if captured.Dialect != deltascope.DialectPostgreSQL {
		t.Fatalf("expected postgresql dialect in public audit request, got %#v", captured.Dialect)
	}
	if captured.MetadataProvider != nil {
		t.Fatalf("expected offline postgresql request to avoid metadata provider")
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	contextValue, ok := payload["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object, got %#v", payload["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("expected offline mode, got %#v", contextValue["mode"])
	}
	if contextValue["dialect"] != "postgresql" {
		t.Fatalf("expected postgresql dialect in context, got %#v", contextValue["dialect"])
	}
	if contextValue["metadata_source"] != "none" {
		t.Fatalf("expected metadata source none, got %#v", contextValue["metadata_source"])
	}
}

func TestHandlerAuditRejectsExplicitPostgreSQLOfflineRequestsOnUnsupportedBuild(t *testing.T) {
	if _, err := appaudit.Parse("SELECT 1", spec.DialectPostgreSQL); err == nil {
		t.Skip("skipping: real PG parser available, capability boundary test requires stub build")
	}
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"insert into users(id) values (1) returning id;","dialect":"postgresql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	errPayload, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error envelope, got %#v", payload)
	}
	if errPayload["code"] != "bad_request" {
		t.Fatalf("expected bad_request code, got %#v", errPayload["code"])
	}
	message, ok := errPayload["message"].(string)
	if !ok || message == "" {
		t.Fatalf("expected non-empty error message, got %#v", errPayload["message"])
	}
	if !strings.Contains(message, "PG-capable") {
		t.Fatalf("expected capability-boundary message, got %q", message)
	}
	if strings.Contains(strings.ToLower(message), "possible dialect mismatch") {
		t.Fatalf("did not expect mismatch wording, got %q", message)
	}
	if strings.Contains(strings.ToLower(message), "if you are auditing postgresql") {
		t.Fatalf("did not expect heuristic suggestion wording, got %q", message)
	}
}

func TestHandlerAuditReturnsFindingExplanationFields(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	finding, explanation := firstFindingByRuleID(t, payload, "dml.where.require")
	if finding["rule_id"] != "dml.where.require" {
		t.Fatalf("expected canonical where-rule finding, got %#v", finding["rule_id"])
	}
	assertExplanationFieldString(t, explanation, "summary")
	assertExplanationFieldString(t, explanation, "why")
	assertExplanationFieldString(t, explanation, "risk")
	assertExplanationFieldString(t, explanation, "suggestion")
}

func TestWriteJSONSerializesFindingExplanationMetadataFields(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, deltascope.Result{
		Statements: []deltascope.StatementResult{{
			Index: 0,
			Kind:  "ddl",
			Findings: []deltascope.Finding{{
				RuleID:  "ddl.table.exists.create.forbid",
				Level:   deltascope.LevelBlocker,
				Message: "table already exists",
				Explanation: &deltascope.FindingExplanation{
					Summary:    "Forbid create existing table",
					Why:        "Live metadata says the target table already exists.",
					Risk:       "Re-running the statement can fail or mask drift.",
					Suggestion: "Use IF NOT EXISTS or reconcile with the live schema first.",
					Metadata: &deltascope.ExplanationMetadata{
						Status: "limited",
						Note:   "metadata unavailable",
					},
				},
			}},
		}},
	})

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	_, explanation := firstFindingByRuleID(t, payload, "ddl.table.exists.create.forbid")
	metadata, ok := explanation["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected explanation metadata object, got %#v", explanation["metadata"])
	}
	assertExplanationFieldString(t, metadata, "status")
	assertExplanationFieldString(t, metadata, "note")
}

func TestWriteJSONSerializesStatementImpact(t *testing.T) {
	estimatedRows := int64(12)
	estimatedRatio := 0.25

	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, deltascope.Result{
		Statements: []deltascope.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Impact: &deltascope.Impact{
				EstimatedRows:  &estimatedRows,
				EstimatedRatio: &estimatedRatio,
				RiskLevel:      deltascope.ImpactRiskMedium,
				Confidence:     deltascope.ImpactConfidenceHigh,
				Source:         deltascope.ImpactSourceMetadata,
				ReasonCodes:    []string{"indexed_range"},
			},
		}},
	})

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	statements, ok := payload["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", payload["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	impact, ok := statement["impact"].(map[string]any)
	if !ok || impact["risk_level"] != "medium" || impact["confidence"] != "high" || impact["source"] != "metadata" {
		t.Fatalf("expected statement impact object, got %#v", statement["impact"])
	}
	if rows, ok := impact["estimated_rows"].(float64); !ok || rows != 12 {
		t.Fatalf("expected estimated_rows 12, got %#v", impact["estimated_rows"])
	}
	reasonCodes, ok := impact["reason_codes"].([]any)
	if !ok || len(reasonCodes) != 1 || reasonCodes[0] != "indexed_range" {
		t.Fatalf("expected indexed_range reason code, got %#v", impact["reason_codes"])
	}
}

func TestWriteJSONOmitsFindingExplanationWhenAbsent(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, deltascope.Result{
		Statements: []deltascope.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []deltascope.Finding{{
				RuleID:  "custom.rule",
				Level:   deltascope.LevelWarning,
				Message: "custom finding",
			}},
		}},
	})

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	finding := firstFindingWithoutExplanationByRuleID(t, payload, "custom.rule")
	if _, ok := finding["explanation"]; ok {
		t.Fatalf("expected explanation to be omitted, got %#v", finding["explanation"])
	}
}

func TestHandlerAuditRejectsInvalidJSON(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"invalid_json"`)) {
		t.Fatalf("expected invalid_json code, got %q", rec.Body.String())
	}
}

func TestHandlerAuditRejectsEmptySQL(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"   "}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"bad_request"`)) {
		t.Fatalf("expected bad_request code, got %q", rec.Body.String())
	}
}

func TestHandlerAuditRejectsOversizedRequestBody(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	oversizedSQL := strings.Repeat("a", 1<<20)
	body := bytes.NewBufferString(`{"sql":"` + oversizedSQL + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"invalid_json"`)) {
		t.Fatalf("expected invalid_json code, got %q", rec.Body.String())
	}
}

func TestNewHandlerRejectsInvalidConfigPath(t *testing.T) {
	if _, err := NewHandler("/tmp/deltascope-missing-config.yaml", "test-build"); err == nil {
		t.Fatalf("expected invalid config path to fail")
	}
}

func TestHandlerAuditRejectsMissingAPIKeyWhenAuthEnabled(t *testing.T) {
	handler, err := NewHandler("", "test-build", WithAuthConfig(AuthConfig{
		Enabled: true,
		Keys:    []string{"ds_test_key"},
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"auth_required"`)) {
		t.Fatalf("expected auth_required code, got %q", rec.Body.String())
	}
}

func TestHandlerAuditRejectsInvalidAPIKeyWhenAuthEnabled(t *testing.T) {
	handler, err := NewHandler("", "test-build", WithAuthConfig(AuthConfig{
		Enabled: true,
		Keys:    []string{"ds_test_key"},
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "wrong_key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"auth_invalid"`)) {
		t.Fatalf("expected auth_invalid code, got %q", rec.Body.String())
	}
}

func TestHandlerAuditAllowsValidAPIKeyWhenAuthEnabled(t *testing.T) {
	handler, err := NewHandler("", "test-build", WithAuthConfig(AuthConfig{
		Enabled: true,
		Keys:    []string{"ds_test_key"},
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "ds_test_key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerHealthzBypassesAuthWhenAllowPathConfigured(t *testing.T) {
	handler, err := NewHandler("", "test-build", WithAuthConfig(AuthConfig{
		Enabled:    true,
		Keys:       []string{"ds_test_key"},
		AllowPaths: []string{"/healthz"},
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerHealthzSetsRequestIDHeader(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	requestID := rec.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Fatalf("expected X-Request-ID header to be set")
	}
	if matched, _ := regexp.MatchString(`^req-[a-f0-9]{24}$`, requestID); !matched {
		t.Fatalf("unexpected request id format: %q", requestID)
	}
}

func TestRecoveryMiddlewareReturnsJSONEnvelopeOnPanic(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(recoveryMiddleware(log.New(io.Discard, "", 0)))
	router.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"internal_error"`)) {
		t.Fatalf("expected internal_error code, got %q", rec.Body.String())
	}
}

func TestHandlerAuditReturnsTimeoutWhenRequestDeadlineExceeded(t *testing.T) {
	handler, err := NewHandler(
		"",
		"test-build",
		WithMiddlewareConfig(MiddlewareConfig{RequestTimeout: 5 * time.Millisecond}),
		WithAuditFunc(func(ctx context.Context, _ deltascope.Request) (deltascope.Result, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return deltascope.Result{}, nil
			case <-ctx.Done():
				return deltascope.Result{}, ctx.Err()
			}
		}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"request_timeout"`)) {
		t.Fatalf("expected request_timeout code, got %q", rec.Body.String())
	}
}

func TestMapAuditErrorTimeout(t *testing.T) {
	status, code := mapAuditError(context.DeadlineExceeded)
	if status != http.StatusGatewayTimeout || code != "request_timeout" {
		t.Fatalf("unexpected timeout mapping: status=%d code=%s", status, code)
	}
}

func TestMapAuditErrorWrappedTimeout(t *testing.T) {
	status, code := mapAuditError(&auditmeta.Error{
		Kind:    auditmeta.ErrorConnectionOpen,
		Message: "open metadata connection: context deadline exceeded",
		Err:     context.DeadlineExceeded,
	})
	if status != http.StatusGatewayTimeout || code != "request_timeout" {
		t.Fatalf("unexpected wrapped timeout mapping: status=%d code=%s", status, code)
	}
}

func TestMapAuditErrorCanceled(t *testing.T) {
	status, code := mapAuditError(context.Canceled)
	if status != http.StatusRequestTimeout || code != "request_canceled" {
		t.Fatalf("unexpected canceled mapping: status=%d code=%s", status, code)
	}
}

func TestMapAuditErrorWrappedCanceled(t *testing.T) {
	status, code := mapAuditError(&auditmeta.Error{
		Kind:    auditmeta.ErrorDialectDetect,
		Message: "detect dialect: context canceled",
		Err:     context.Canceled,
	})
	if status != http.StatusRequestTimeout || code != "request_canceled" {
		t.Fatalf("unexpected wrapped canceled mapping: status=%d code=%s", status, code)
	}
}

func TestMapAuditErrorEmptySQL(t *testing.T) {
	status, code := mapAuditError(appaudit.ErrEmptySQL)
	if status != http.StatusBadRequest || code != "bad_request" {
		t.Fatalf("unexpected bad request mapping: status=%d code=%s", status, code)
	}
}

func TestMapAuditErrorSchemaHintRequired(t *testing.T) {
	status, code := mapAuditError(&auditmeta.Error{Kind: auditmeta.ErrorSchemaHintRequired, Message: "set schema"})
	if status != http.StatusBadRequest || code != "connection_invalid" {
		t.Fatalf("unexpected schema hint mapping: status=%d code=%s", status, code)
	}
}

func TestMapAuditErrorConnectionOpen(t *testing.T) {
	status, code := mapAuditError(&auditmeta.Error{Kind: auditmeta.ErrorConnectionOpen, Message: "open metadata connection: boom"})
	if status != http.StatusBadGateway || code != "connection_failed" {
		t.Fatalf("unexpected connection open mapping: status=%d code=%s", status, code)
	}
}

func TestMapAuditErrorConnectionValidation(t *testing.T) {
	status, code := mapAuditError(&ifaceconn.ConnectionInputError{
		Kind:    ifaceconn.ErrorKindValidation,
		Message: "connection must include at least one non-password field",
	})
	if status != http.StatusBadRequest || code != "connection_invalid" {
		t.Fatalf("unexpected connection validation mapping: status=%d code=%s", status, code)
	}
}

func TestMapAuditErrorPasswordEnv(t *testing.T) {
	status, code := mapAuditError(&ifaceconn.ConnectionInputError{
		Kind:    ifaceconn.ErrorKindPasswordLookup,
		Message: `password env "DB_PASS" is not set`,
	})
	if status != http.StatusBadRequest || code != "connection_invalid" {
		t.Fatalf("unexpected password env mapping: status=%d code=%s", status, code)
	}
}

func TestMapAuditErrorPlainTextConnectionMessageFallsBackToBadRequest(t *testing.T) {
	status, code := mapAuditError(errors.New("connection must include at least one non-password field"))
	if status != http.StatusBadRequest || code != "bad_request" {
		t.Fatalf("expected plain text connection message to avoid special classification: status=%d code=%s", status, code)
	}
}

func TestHandlerMetricsEndpoint(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	primeReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	primeRec := httptest.NewRecorder()
	handler.ServeHTTP(primeRec, primeReq)
	if primeRec.Code != http.StatusOK {
		t.Fatalf("prime request expected 200, got %d: %s", primeRec.Code, primeRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("deltascope_http_requests_total")) {
		t.Fatalf("expected metrics payload, got %q", rec.Body.String())
	}
}

func TestHandlerAuditRateLimitByAPIKey(t *testing.T) {
	handler, err := NewHandler(
		"",
		"test-build",
		WithAuthConfig(AuthConfig{
			Enabled: true,
			Keys:    []string{"ds_test_key"},
		}),
		WithMiddlewareConfig(MiddlewareConfig{
			RateLimit: RateLimitConfig{
				Enabled: true,
				RPS:     1,
				Burst:   1,
				KeyBy:   "api-key",
			},
		}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req1 := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-API-Key", "ds_test_key")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request expected 200, got %d: %s", rec1.Code, rec1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-API-Key", "ds_test_key")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request expected 429, got %d: %s", rec2.Code, rec2.Body.String())
	}
	if !bytes.Contains(rec2.Body.Bytes(), []byte(`"rate_limited"`)) {
		t.Fatalf("expected rate_limited code, got %q", rec2.Body.String())
	}
}

func TestHandlerAuditRateLimitByIPIgnoresForwardedForWhenNoTrustedProxies(t *testing.T) {
	handler, err := NewHandler(
		"",
		"test-build",
		WithMiddlewareConfig(MiddlewareConfig{
			RateLimit: RateLimitConfig{
				Enabled: true,
				RPS:     1,
				Burst:   1,
				KeyBy:   "ip",
			},
		}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req1 := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Forwarded-For", "198.51.100.10")
	req1.RemoteAddr = "10.0.0.2:20001"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request expected 200, got %d: %s", rec1.Code, rec1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Forwarded-For", "203.0.113.20")
	req2.RemoteAddr = "10.0.0.2:20002"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request expected 429 when remote ip is same, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestRequestRateLimitKeyIPUsesForwardedClientIP(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	rec := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(rec)
	if err := engine.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"}); err != nil {
		t.Fatalf("set trusted proxies: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/audit", nil)
	req.RemoteAddr = "10.0.0.2:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.8")
	ctx.Request = req

	got := requestRateLimitKey(ctx, "ip")
	if got != "203.0.113.8" {
		t.Fatalf("expected forwarded client ip, got %q", got)
	}
}

func TestLimiterStoreCleansExpiredEntries(t *testing.T) {
	store := newLimiterStore(rate.Limit(1), 1)
	store.ttl = time.Millisecond
	store.cleanupInterval = time.Millisecond
	store.nextCleanup = time.Now().Add(-time.Second)
	store.entries["stale"] = limiterEntry{
		limiter:  rate.NewLimiter(rate.Limit(1), 1),
		lastSeen: time.Now().Add(-time.Hour),
	}

	_ = store.Allow("fresh")

	store.mu.Lock()
	_, exists := store.entries["stale"]
	store.mu.Unlock()
	if exists {
		t.Fatalf("expected stale limiter entry to be evicted")
	}
}

func TestHandlerAuditDefaultPolicyDialectHygieneMySQLExcludesPostgreSQLRules(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"CREATE TABLE smoke_users (id bigint unsigned NOT NULL AUTO_INCREMENT, name varchar(64) NOT NULL DEFAULT '', created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='smoke users';","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	assertHTTPPayloadNoPGRuleIDs(t, payload)
}

func TestHandlerAuditDefaultPolicyDialectHygieneTiDBExcludesPostgreSQLRules(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"CREATE TABLE smoke_users (id bigint unsigned NOT NULL AUTO_INCREMENT, name varchar(64) NOT NULL DEFAULT '', created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='smoke users';","dialect":"tidb"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	assertHTTPPayloadNoPGRuleIDs(t, payload)
}

func assertHTTPPayloadNoPGRuleIDs(t *testing.T, payload map[string]any) {
	t.Helper()
	statements, ok := payload["statements"].([]any)
	if !ok {
		return
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
			if ruleID, _ := finding["rule_id"].(string); strings.HasPrefix(ruleID, "ddl.pg.") {
				t.Errorf("MySQL/TiDB default HTTP audit should not emit PG-only rule %q", ruleID)
			}
		}
	}
	globalFindings, ok := payload["global_findings"].([]any)
	if !ok {
		return
	}
	for _, rawFinding := range globalFindings {
		finding, ok := rawFinding.(map[string]any)
		if !ok {
			continue
		}
		if ruleID, _ := finding["rule_id"].(string); strings.HasPrefix(ruleID, "ddl.pg.") {
			t.Errorf("MySQL/TiDB default HTTP audit should not emit PG-only rule %q in global findings", ruleID)
		}
	}
}

func firstFindingByRuleID(t *testing.T, payload map[string]any, ruleID string) (map[string]any, map[string]any) {
	t.Helper()
	statements, ok := payload["statements"].([]any)
	if !ok {
		t.Fatalf("expected statements array, got %#v", payload["statements"])
	}
	for _, rawStatement := range statements {
		statement, ok := rawStatement.(map[string]any)
		if !ok {
			continue
		}
		findings, ok := statement["findings"].([]any)
		if !ok {
			continue
		}
		for _, rawFinding := range findings {
			finding, ok := rawFinding.(map[string]any)
			if !ok {
				continue
			}
			if finding["rule_id"] != ruleID {
				continue
			}
			explanation, ok := finding["explanation"].(map[string]any)
			if !ok {
				t.Fatalf("expected explanation on rule %q, got %#v", ruleID, finding["explanation"])
			}
			return finding, explanation
		}
	}
	t.Fatalf("expected finding for rule %q, got %#v", ruleID, payload)
	return nil, nil
}

func firstFindingWithoutExplanationByRuleID(t *testing.T, payload map[string]any, ruleID string) map[string]any {
	t.Helper()
	statements, ok := payload["statements"].([]any)
	if !ok {
		t.Fatalf("expected statements array, got %#v", payload["statements"])
	}
	for _, rawStatement := range statements {
		statement, ok := rawStatement.(map[string]any)
		if !ok {
			continue
		}
		findings, ok := statement["findings"].([]any)
		if !ok {
			continue
		}
		for _, rawFinding := range findings {
			finding, ok := rawFinding.(map[string]any)
			if !ok {
				continue
			}
			if finding["rule_id"] == ruleID {
				return finding
			}
		}
	}
	t.Fatalf("expected finding for rule %q, got %#v", ruleID, payload)
	return nil
}

func TestHandlerReadyz(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON body: %v", err)
	}
	if body["status"] != "ready" {
		t.Fatalf("expected status=ready, got %q", body["status"])
	}
}

func TestAccessLogMiddlewareEmitsJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf strings.Builder
	logger := log.New(&buf, "", 0)

	r := gin.New()
	r.Use(accessLogMiddleware(logger))
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected access log output, got empty")
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("expected JSON log line, got %q: %v", line, err)
	}
	for _, key := range []string{"method", "path", "status", "duration_ms", "request_id"} {
		if _, ok := entry[key]; !ok {
			t.Fatalf("missing JSON log field %q in %q", key, line)
		}
	}
}

func assertExplanationFieldString(t *testing.T, object map[string]any, key string) string {
	t.Helper()
	value, ok := object[key].(string)
	if !ok || value == "" {
		t.Fatalf("expected non-empty %q field, got %#v", key, object[key])
	}
	return value
}

func TestHandlerAuditReturnsSourceLocationInFindings(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	sql := `create table ok_users (
  id bigint unsigned not null auto_increment comment 'id',
  name varchar(32) not null default '' comment 'name',
  created_at datetime not null default current_timestamp comment 'created',
  updated_at datetime not null default current_timestamp on update current_timestamp comment 'updated',
  primary key (id)
) comment='ok users';

delete from users;`

	body, _ := json.Marshal(map[string]string{"sql": sql, "dialect": "mysql"})
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	statements, _ := payload["statements"].([]any)
	if len(statements) < 2 {
		t.Fatalf("expected at least 2 statements, got %d", len(statements))
	}

	deleteStmt, _ := statements[1].(map[string]any)
	findings, _ := deleteStmt["findings"].([]any)

	var whereFinding map[string]any
	for _, f := range findings {
		finding, _ := f.(map[string]any)
		if finding["rule_id"] == "dml.where.require" {
			whereFinding = finding
			break
		}
	}
	if whereFinding == nil {
		t.Fatal("expected dml.where.require finding in delete statement")
	}

	loc, _ := whereFinding["location"].(map[string]any)
	if loc == nil {
		t.Fatal("expected location object in dml.where.require finding")
	}
	line, _ := loc["line"].(float64)
	if line != 9 {
		t.Errorf("expected location.line=9, got %v", loc["line"])
	}
	column, _ := loc["column"].(float64)
	if column != 1 {
		t.Errorf("expected location.column=1, got %v", loc["column"])
	}
}

func TestHandlerAuditPostgreSQLAdvancedIndexFormsSupportedAndCovered(t *testing.T) {
	if _, err := appaudit.Parse("SELECT 1", spec.DialectPostgreSQL); err != nil {
		t.Skip("skipping: PG-capable build required for advanced index normalization test")
	}
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"CREATE INDEX idx_users_active_email ON users (email) WHERE active = true","dialect":"postgresql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if unsupported, ok := payload["unsupported"].([]any); ok && len(unsupported) != 0 {
		t.Fatalf("expected no unsupported details, got %#v", unsupported)
	}

	contextValue, ok := payload["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object, got %#v", payload["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("expected offline mode, got %#v", contextValue["mode"])
	}
	if contextValue["dialect"] != "postgresql" {
		t.Fatalf("expected postgresql dialect, got %#v", contextValue["dialect"])
	}

	statements, ok := payload["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", payload["statements"])
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
	for _, item := range findings {
		finding, ok := item.(map[string]any)
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

func TestHandlerAuditPostgreSQLAlterTableUnsupportedActionRuleCoverage(t *testing.T) {
	if _, err := appaudit.Parse("SELECT 1", spec.DialectPostgreSQL); err != nil {
		t.Skip("skipping: PG-capable build required for alter table unsupported action rule coverage test")
	}
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "set_schema_advisory",
			sql:        `ALTER TABLE users SET SCHEMA archive;`,
			wantRuleID: "ddl.pg.alter.set_schema.advisory",
		},
		{
			name:       "disable_trigger_warn",
			sql:        `ALTER TABLE users DISABLE TRIGGER trg_users_audit;`,
			wantRuleID: "ddl.pg.alter.disable_trigger.warn",
		},
		{
			name:       "disable_trigger_all_warn",
			sql:        `ALTER TABLE users DISABLE TRIGGER ALL;`,
			wantRuleID: "ddl.pg.alter.disable_trigger.warn",
		},
		{
			name:       "replica_identity_full_warn",
			sql:        `ALTER TABLE users REPLICA IDENTITY FULL;`,
			wantRuleID: "ddl.pg.alter.replica_identity_full.warn",
		},
		{
			name:       "replica_identity_using_index_notice",
			sql:        `ALTER TABLE users REPLICA IDENTITY USING INDEX users_pkey;`,
			wantRuleID: "ddl.pg.alter.replica_identity_using_index.notice",
		},
		{
			name:       "detach_partition_warn",
			sql:        `ALTER TABLE measurement DETACH PARTITION measurement_y2026m04;`,
			wantRuleID: "ddl.pg.alter.detach_partition.warn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"sql":"` + strings.ReplaceAll(tt.sql, `"`, `\"`) + `","dialect":"postgresql"}`
			req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if unsupported, ok := payload["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}

			statements, ok := payload["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", payload["statements"])
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
			for _, item := range findings {
				finding, ok := item.(map[string]any)
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

func TestHandlerAuditPostgreSQLRefreshMaterializedViewRuleCoverage(t *testing.T) {
	if _, err := appaudit.Parse("SELECT 1", spec.DialectPostgreSQL); err != nil {
		t.Skip("skipping: PG-capable build required for refresh materialized view rule coverage test")
	}
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	t.Run("basic_refresh_concurrently_warn", func(t *testing.T) {
		body := `{"sql":"REFRESH MATERIALIZED VIEW mv_stats;","dialect":"postgresql"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if unsupported, ok := payload["unsupported"].([]any); ok && len(unsupported) != 0 {
			t.Fatalf("expected no unsupported, got %#v", unsupported)
		}
		statements, ok := payload["statements"].([]any)
		if !ok || len(statements) != 1 {
			t.Fatalf("expected one statement, got %#v", payload["statements"])
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
		for _, item := range findings {
			finding, ok := item.(map[string]any)
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
		body := `{"sql":"REFRESH MATERIALIZED VIEW mv_stats WITH NO DATA;","dialect":"postgresql"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
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
		if !ok || len(statements) != 1 {
			t.Fatalf("expected one statement, got %#v", payload["statements"])
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
		for _, item := range findings {
			finding, ok := item.(map[string]any)
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

func TestHandlerAuditPostgreSQLAlterTableGapRuleCoverage(t *testing.T) {
	if _, err := appaudit.Parse("SELECT 1", spec.DialectPostgreSQL); err != nil {
		t.Skip("skipping: PG-capable build required for alter table gap rule coverage test")
	}
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "drop_column_advisory",
			sql:        `ALTER TABLE users DROP COLUMN email;`,
			wantRuleID: "ddl.pg.alter.drop_column.advisory",
		},
		{
			name:       "validate_constraint_advisory",
			sql:        `ALTER TABLE users VALIDATE CONSTRAINT chk_price;`,
			wantRuleID: "ddl.pg.alter.validate_constraint.advisory",
		},
		{
			name:       "add_column_nullable_notice",
			sql:        `ALTER TABLE users ADD COLUMN bio text;`,
			wantRuleID: "ddl.pg.alter.add_column.nullable.notice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"sql":"` + strings.ReplaceAll(tt.sql, `"`, `\"`) + `","dialect":"postgresql"}`
			req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if unsupported, ok := payload["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}

			statements, ok := payload["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", payload["statements"])
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
			for _, item := range findings {
				finding, ok := item.(map[string]any)
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

func TestHandlerAuditPostgreSQLAlterTableLoggedStateRuleCoverage(t *testing.T) {
	if _, err := appaudit.Parse("SELECT 1", spec.DialectPostgreSQL); err != nil {
		t.Skip("skipping: PG-capable build required for alter table logged state rule coverage test")
	}
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "set_logged_notice",
			sql:        `ALTER TABLE users SET LOGGED;`,
			wantRuleID: "ddl.pg.alter.set_logged.notice",
		},
		{
			name:       "set_unlogged_notice",
			sql:        `ALTER TABLE users SET UNLOGGED;`,
			wantRuleID: "ddl.pg.alter.set_unlogged.notice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"sql":"` + strings.ReplaceAll(tt.sql, `"`, `\"`) + `","dialect":"postgresql"}`
			req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if unsupported, ok := payload["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}

			statements, ok := payload["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", payload["statements"])
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
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if finding["rule_id"] == tt.wantRuleID {
					found = true
					if finding["level"] != "notice" {
						t.Errorf("expected level notice, got %v", finding["level"])
					}
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule_id %s, got %#v", tt.wantRuleID, findings)
			}
		})
	}
}

func TestHandlerAuditPostgreSQLTypeLifecycleRuleCoverage(t *testing.T) {
	if _, err := appaudit.Parse("SELECT 1", spec.DialectPostgreSQL); err != nil {
		t.Skip("skipping: PG-capable build required for type lifecycle rule coverage test")
	}
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "create_type_enum_notice",
			sql:         `CREATE TYPE color AS ENUM ('red', 'green', 'blue');`,
			wantRuleIDs: []string{"ddl.pg.create_type.enum.notice"},
		},
		{
			name:        "alter_type_add_value_with_position",
			sql:         `ALTER TYPE color ADD VALUE 'yellow' AFTER 'green';`,
			wantRuleIDs: []string{"ddl.pg.alter_type.add_value.advisory", "ddl.pg.alter_type.add_value.position.notice"},
		},
		{
			name:        "drop_type_cascade",
			sql:         `DROP TYPE IF EXISTS color CASCADE;`,
			wantRuleIDs: []string{"ddl.pg.drop_type.advisory", "ddl.pg.drop_type.cascade.warn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"sql":"` + strings.ReplaceAll(tt.sql, `"`, `\"`) + `","dialect":"postgresql"}`
			req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if unsupported, ok := payload["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}

			statements, ok := payload["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", payload["statements"])
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
			for _, item := range findings {
				finding, ok := item.(map[string]any)
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
		})
	}
}

func TestHandlerAuditPostgreSQLCompositeTypeLifecycleRuleCoverage(t *testing.T) {
	if _, err := appaudit.Parse("SELECT 1", spec.DialectPostgreSQL); err != nil {
		t.Skip("skipping: PG-capable build required for composite type lifecycle rule coverage test")
	}
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "create_type_composite_notice",
			sql:         `CREATE TYPE address AS (street text, city text);`,
			wantRuleIDs: []string{"ddl.pg.create_type.composite.notice"},
		},
		{
			name:        "alter_type_composite_rename_notice",
			sql:         `ALTER TYPE address RENAME TO mailing_address;`,
			wantRuleIDs: []string{"ddl.pg.alter_type.composite_rename.notice"},
		},
		{
			name:        "alter_type_composite_set_schema_notice",
			sql:         `ALTER TYPE address SET SCHEMA archive;`,
			wantRuleIDs: []string{"ddl.pg.alter_type.composite_set_schema.notice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"sql":"` + strings.ReplaceAll(tt.sql, `"`, `\"`) + `","dialect":"postgresql"}`
			req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if unsupported, ok := payload["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}

			statements, ok := payload["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", payload["statements"])
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
			for _, item := range findings {
				finding, ok := item.(map[string]any)
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
		})
	}
}

func TestHandlerAuditPostgreSQLDomainLifecycleRuleCoverage(t *testing.T) {
	if _, err := appaudit.Parse("SELECT 1", spec.DialectPostgreSQL); err != nil {
		t.Skip("skipping: PG-capable build required for domain lifecycle rule coverage test")
	}
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "create_domain_notice",
			sql:         `CREATE DOMAIN email AS text CHECK (VALUE <> '');`,
			wantRuleIDs: []string{"ddl.pg.create_domain.notice"},
		},
		{
			name:        "alter_domain_add_constraint",
			sql:         `ALTER DOMAIN email ADD CONSTRAINT email_not_empty CHECK (VALUE <> '');`,
			wantRuleIDs: []string{"ddl.pg.alter_domain.constraint.notice"},
		},
		{
			name:        "alter_domain_rename",
			sql:         `ALTER DOMAIN email RENAME TO contact_email;`,
			wantRuleIDs: []string{"ddl.pg.alter_domain.rename.notice"},
		},
		{
			name:        "drop_domain_cascade",
			sql:         `DROP DOMAIN IF EXISTS email CASCADE;`,
			wantRuleIDs: []string{"ddl.pg.drop_domain.advisory", "ddl.pg.drop_domain.cascade.warn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"sql":"` + strings.ReplaceAll(tt.sql, `"`, `\"`) + `","dialect":"postgresql"}`
			req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if unsupported, ok := payload["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}

			statements, ok := payload["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", payload["statements"])
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
			for _, item := range findings {
				finding, ok := item.(map[string]any)
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
		})
	}
}
