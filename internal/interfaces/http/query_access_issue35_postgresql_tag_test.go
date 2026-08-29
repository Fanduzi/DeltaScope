//go:build postgresql

// Package httpapi_test verifies CLI and HTTP Query Access state parity.
// input: the issue-35 schema-qualified PostgreSQL offline query
// output: matching classification, admission, and reason-code JSON fields
// pos: cross-surface transport regression at the public CLI and HTTP seams
// note: if this file changes, update this header and module README.md.
package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/interfaces/cli"
	httpapi "github.com/Fanduzi/DeltaScope/internal/interfaces/http"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestIssue35QueryAccessCLIAndHTTPShareFinalState(t *testing.T) {
	const sqlText = "SELECT id FROM public.users"

	var cliOutput, cliErrors bytes.Buffer
	if code := cli.Execute(context.Background(), []string{
		"query-access", "analyze", "--sql", sqlText, "--dialect", "postgresql",
	}, &bytes.Buffer{}, &cliOutput, &cliErrors); code != 0 {
		t.Fatalf("CLI exit code = %d, want 0: %s", code, cliErrors.String())
	}

	var cliResult deltascope.QueryAccessResult
	if err := json.Unmarshal(cliOutput.Bytes(), &cliResult); err != nil {
		t.Fatalf("decode CLI result: %v", err)
	}

	handler, err := httpapi.NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new HTTP handler: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(`{"sql":"SELECT id FROM public.users","dialect":"postgresql"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var httpResult deltascope.QueryAccessResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &httpResult); err != nil {
		t.Fatalf("decode HTTP result: %v", err)
	}

	for surface, result := range map[string]deltascope.QueryAccessResult{
		"CLI":  cliResult,
		"HTTP": httpResult,
	} {
		if result.ReadClassification != deltascope.QueryAccessReadOnly || result.Admission != deltascope.QueryAccessAdmissible {
			t.Errorf("%s state = %q/%q, want read_only/admissible", surface, result.ReadClassification, result.Admission)
		}
		if len(result.ReasonCodes) != 0 {
			t.Errorf("%s reason_codes = %v, want empty", surface, result.ReasonCodes)
		}
	}
	if cliResult.ReadClassification != httpResult.ReadClassification ||
		cliResult.Admission != httpResult.Admission ||
		!reflect.DeepEqual(cliResult.ReasonCodes, httpResult.ReasonCodes) {
		t.Fatalf("CLI/HTTP state differs: CLI=%q/%q/%v HTTP=%q/%q/%v",
			cliResult.ReadClassification, cliResult.Admission, cliResult.ReasonCodes,
			httpResult.ReadClassification, httpResult.Admission, httpResult.ReasonCodes)
	}
}
