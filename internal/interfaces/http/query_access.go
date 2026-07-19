// Package httpapi exposes the HTTP adapter for DeltaScope.
// input: HTTP query-access analysis requests carrying SQL text, dialect, mode, profile, and optional schema context
// output: JSON query access analysis results for the HTTP adapter
// pos: HTTP adapter glue between request-scoped inputs and the public DeltaScope query access API
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

type queryAccessRequest struct {
	SQL             string                                `json:"sql"`
	Dialect         deltascope.Dialect                    `json:"dialect,omitempty"`
	Mode            string                                `json:"mode,omitempty"`
	DefaultSchema   string                                `json:"default_schema,omitempty"`
	AnalysisProfile deltascope.QueryAccessAnalysisProfile `json:"profile,omitempty"`
}

var analyzeQueryAccess = deltascope.AnalyzeQueryAccess

func handleQueryAccess(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	r.Body = http.MaxBytesReader(w, r.Body, maxAuditRequestBodyBytes)

	var request queryAccessRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if err := decoder.Decode(&struct{}{}); err == nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain exactly one JSON object")
		return
	} else if !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain exactly one JSON object")
		return
	}

	sql := strings.TrimSpace(request.SQL)
	if sql == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "sql must not be empty")
		return
	}

	dialect := request.Dialect
	if dialect == "" {
		dialect = deltascope.DialectMySQL
	}

	mode := deltascope.QueryAccessMode(strings.TrimSpace(request.Mode))
	if mode == "" {
		mode = deltascope.QueryAccessModeStrict
	}
	if mode != deltascope.QueryAccessModeStrict && mode != deltascope.QueryAccessModeProjectionOnly {
		writeError(w, http.StatusBadRequest, "invalid_mode", fmt.Sprintf("invalid mode %q: must be strict or projection_only", mode))
		return
	}

	result, err := analyzeQueryAccess(r.Context(), deltascope.QueryAccessRequest{
		SQL:             request.SQL,
		Dialect:         dialect,
		Mode:            mode,
		DefaultSchema:   strings.TrimSpace(request.DefaultSchema),
		AnalysisProfile: request.AnalysisProfile,
	})
	if err != nil {
		status, code := mapQueryAccessError(err)
		writeError(w, status, code, mapQueryAccessErrorMessage(err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func mapQueryAccessError(err error) (int, string) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "request_timeout"
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout, "request_canceled"
	case errors.Is(err, deltascope.ErrInvalidQueryAccessAnalysisProfile):
		return http.StatusBadRequest, "invalid_profile"
	case errors.Is(err, deltascope.ErrQueryAccessAnalysisProfileDialectMismatch):
		return http.StatusBadRequest, "profile_dialect_mismatch"
	case strings.Contains(err.Error(), "unsupported dialect"):
		return http.StatusBadRequest, "bad_request"
	default:
		return http.StatusBadRequest, "bad_request"
	}
}

// mapQueryAccessErrorMessage returns a bounded error message safe for HTTP clients.
// It never exposes raw driver text, DSN, credentials, SQL, or catalog details.
func mapQueryAccessErrorMessage(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "request timed out"
	case errors.Is(err, context.Canceled):
		return "request canceled"
	case errors.Is(err, deltascope.ErrInvalidQueryAccessAnalysisProfile):
		return "invalid analysis profile"
	case errors.Is(err, deltascope.ErrQueryAccessAnalysisProfileDialectMismatch):
		return "analysis profile does not match dialect"
	case strings.Contains(err.Error(), "unsupported dialect"):
		return "unsupported dialect"
	default:
		return "analysis failed"
	}
}
