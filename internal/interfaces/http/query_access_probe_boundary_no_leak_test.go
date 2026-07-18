// Package httpapi adds no-leak regression coverage for the MySQL/TiDB
// builtin-effect identity feasibility probe boundary on the HTTP surface. The
// live Docker probes live in
// internal/infrastructure/metadata/mysql/builtin_effect_identity_live_probes_test.go
// (build tag: integration) and do NOT introduce a new HTTP path. This file
// therefore tests the UNCHANGED HTTP surface (`POST /v1/query-access/analyze`)
// under MySQL/TiDB function-bearing SQL with injected markers and under
// injected driver-error-like text, and asserts those markers, identity facts,
// candidates, session/context data, manifest data, raw SQL, and `severity` are
// absent from the HTTP response body.
//
// input: HTTP requests with MySQL/TiDB function-bearing SQL carrying injected
//
//	marker function names, literals, DSN/credential-shaped strings, and
//	driver-error-like text (via the analyzeQueryAccess function variable)
//
// output: HTTP response body contains none of the injected markers, no
//
//	identity/candidate/manifest/session fields, no raw SQL, no `severity` field
//
// pos: HTTP no-leak regression for the MySQL/TiDB probe boundary; no new HTTP
//
//	path is introduced or exercised here
//
// note: if this file changes, update this header, the module README.md, and
//
//	the decision record's no-leak evidence section. See
//	docs/decisions/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility.md
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

// mysqlTiDBHTTPNoLeakMarkers is the canonical marker set for the MySQL/TiDB
// builtin-identity probe boundary on the HTTP surface.
var mysqlTiDBHTTPNoLeakMarkers = []string{
	"my_secret_udf",
	"SECRET_LITERAL",
	"P@ssw0rd",
	"postgres://user:pass@host",
	"mysql://root:root@host:3406",
	"driver: bad conn",
	"severity",
	"identity",
	"candidate",
	"manifest",
	"session",
	"dsn",
}

// TestHandlerQueryAccess_MySQLTiDBProbeBoundary_NoLeak verifies the unchanged
// HTTP surface does not leak injected markers, identity facts, candidates,
// session/context, manifest, raw SQL, or severity when analyzing MySQL/TiDB
// function-bearing SQL that references marker function names and literals.
//
// This test does NOT call t.Parallel() at the top level because other tests
// in this file replace the package-level analyzeQueryAccess function variable;
// top-level serialization prevents a race on that variable. Subtests use
// t.Parallel() for throughput.
func TestHandlerQueryAccess_MySQLTiDBProbeBoundary_NoLeak(t *testing.T) {

	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	cases := []struct {
		name    string
		sql     string
		dialect string
	}{
		{name: "mysql_udf_marker_function", sql: "SELECT my_secret_udf(id) FROM users", dialect: "mysql"},
		{name: "tidb_udf_marker_function", sql: "SELECT my_secret_udf(id) FROM users", dialect: "tidb"},
		{name: "mysql_count_with_marker_literal", sql: "SELECT COUNT('SECRET_LITERAL') FROM users", dialect: "mysql"},
		{name: "tidb_count_with_marker_literal", sql: "SELECT COUNT('SECRET_LITERAL') FROM users", dialect: "tidb"},
		{name: "mysql_qualified_marker_function", sql: "SELECT app.my_secret_udf(id) FROM users", dialect: "mysql"},
		{name: "tidb_qualified_marker_function", sql: "SELECT app.my_secret_udf(id) FROM users", dialect: "tidb"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := fmt.Sprintf(`{"sql":%q,"dialect":%q,"mode":"strict"}`, tc.sql, tc.dialect)
			req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("%s: expected 200, got %d: %s", tc.name, rec.Code, rec.Body.String())
			}

			respBody := rec.Body.String()

			// Assert no-leak on response body.
			for _, marker := range mysqlTiDBHTTPNoLeakMarkers {
				if strings.Contains(respBody, marker) {
					t.Errorf("%s: HTTP response leaked marker %q: %s", tc.name, marker, respBody)
				}
			}

			// The raw SQL must not appear in the response body.
			if strings.Contains(respBody, tc.sql) {
				t.Errorf("%s: HTTP response must not embed raw SQL: %s", tc.name, respBody)
			}

			// Verify no forbidden top-level fields in JSON.
			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("%s: decode response: %v", tc.name, err)
			}
			forbiddenTopLevel := []string{
				"severity", "identity", "candidate", "manifest", "session",
				"context", "dsn", "raw_sql", "driver_error",
			}
			for _, field := range forbiddenTopLevel {
				if _, ok := payload[field]; ok {
					t.Errorf("%s: HTTP JSON must not carry top-level field %q: %s", tc.name, field, respBody)
				}
			}

			// Function-bearing SQL must yield indeterminate admission.
			if payload["admission"] != "indeterminate" {
				t.Errorf("%s: expected indeterminate admission, got %#v", tc.name, payload["admission"])
			}
		})
	}
}

// TestHandlerQueryAccess_MySQLTiDBProbeBoundary_DriverErrorNoLeak is the ONLY
// no-leak test that injects DSN/credential/driver-error markers through a
// real error path (by replacing the package-level analyzeQueryAccess function
// variable with an error carrying all markers). The SDK and CLI no-leak tests
// cover only the normal path (function-name + literal markers) because those
// surfaces do not expose a live-connection error path; this HTTP test covers
// the error-boundary marker injection for the whole probe characterization.
func TestHandlerQueryAccess_MySQLTiDBProbeBoundary_DriverErrorNoLeak(t *testing.T) {
	previous := analyzeQueryAccess
	analyzeQueryAccess = func(_ context.Context, _ deltascope.QueryAccessRequest) (*deltascope.QueryAccessResult, error) {
		return nil, fmt.Errorf("driver: bad conn: mysql://root:root@host:3406/app?password=P@ssw0rd my_secret_udf SECRET_LITERAL")
	}
	t.Cleanup(func() { analyzeQueryAccess = previous })

	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(`{"sql":"SELECT my_secret_udf(id) FROM users","dialect":"mysql","mode":"strict"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, marker := range mysqlTiDBHTTPNoLeakMarkers {
		if strings.Contains(strings.ToLower(body), strings.ToLower(marker)) {
			t.Errorf("HTTP error response leaked marker %q: %s", marker, body)
		}
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	errObj, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %#v", payload["error"])
	}
	if errObj["message"] != "analysis failed" {
		t.Errorf("expected bounded message 'analysis failed', got %q", errObj["message"])
	}
	if errObj["code"] != "bad_request" {
		t.Errorf("expected bad_request code, got %q", errObj["code"])
	}
}

// TestHandlerQueryAccess_MySQLTiDBProbeBoundary_FunctionFreeAdmissibleNoLeak
// verifies the unchanged HTTP surface keeps function-free admissible MySQL/TiDB
// SELECTs leak-free under marker injection. Not parallel at top level for the
// same reason as TestHandlerQueryAccess_MySQLTiDBProbeBoundary_NoLeak.
func TestHandlerQueryAccess_MySQLTiDBProbeBoundary_FunctionFreeAdmissibleNoLeak(t *testing.T) {

	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	cases := []struct {
		name    string
		sql     string
		dialect string
	}{
		{name: "mysql_where_marker_literal", sql: "SELECT id FROM users WHERE name = 'SECRET_LITERAL'", dialect: "mysql"},
		{name: "tidb_where_marker_literal", sql: "SELECT id FROM users WHERE name = 'SECRET_LITERAL'", dialect: "tidb"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := fmt.Sprintf(`{"sql":%q,"dialect":%q,"mode":"strict"}`, tc.sql, tc.dialect)
			req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("%s: expected 200, got %d: %s", tc.name, rec.Code, rec.Body.String())
			}

			respBody := rec.Body.String()
			for _, marker := range mysqlTiDBHTTPNoLeakMarkers {
				if strings.Contains(respBody, marker) {
					t.Errorf("%s: HTTP response leaked marker %q: %s", tc.name, marker, respBody)
				}
			}
		})
	}
}
