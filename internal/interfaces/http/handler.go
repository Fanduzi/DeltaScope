// Package httpapi exposes the HTTP adapter for DeltaScope.
// input: HTTP requests carrying SQL audit payloads plus service-level config/version wiring
// output: JSON audit, health, version, and structured error responses
// pos: interface adapter between net/http and the public DeltaScope audit API
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	apppolicy "github.com/Fanduzi/DeltaScope/internal/application/policy"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

type auditRequest struct {
	SQL     string             `json:"sql"`
	Dialect deltascope.Dialect `json:"dialect,omitempty"`
}

type errorEnvelope struct {
	Error serviceError `json:"error"`
}

type serviceError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewHandler returns the JSON HTTP adapter for DeltaScope.
func NewHandler(configPath, version string) (http.Handler, error) {
	if configPath != "" {
		if _, err := apppolicy.Load(configPath); err != nil {
			return nil, err
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": version})
	})
	mux.HandleFunc("POST /v1/audit", func(w http.ResponseWriter, r *http.Request) {
		handleAudit(w, r, configPath)
	})

	return mux, nil
}

func handleAudit(w http.ResponseWriter, r *http.Request, configPath string) {
	defer r.Body.Close()

	var request auditRequest
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

	result, err := deltascope.Audit(r.Context(), deltascope.Request{
		SQL:        request.SQL,
		Dialect:    request.Dialect,
		ConfigPath: configPath,
	})
	if err != nil {
		status, code := mapAuditError(err)
		writeError(w, status, code, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func mapAuditError(err error) (status int, code string) {
	switch {
	case errors.Is(err, appaudit.ErrEmptySQL), errors.Is(err, appaudit.ErrUnknownDialect):
		return http.StatusBadRequest, "bad_request"
	case strings.Contains(err.Error(), "load policy:"):
		return http.StatusInternalServerError, "config_invalid"
	default:
		return http.StatusBadRequest, "bad_request"
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{
		Error: serviceError{
			Code:    code,
			Message: message,
		},
	})
}
