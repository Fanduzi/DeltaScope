// Package httpapi exposes the HTTP adapter for DeltaScope.
// input: query-access JSON requests, catalog/schema hints, authorized runtime connection configuration, and the unified public online query access API
// output: bounded offline or alias-bound identity-routed online query-access JSON responses with stable identity error mapping and unchanged logging contracts
// pos: HTTP query-access adapter above offline analysis and the opaque unified online session boundary
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

	"github.com/Fanduzi/DeltaScope/internal/application/online"
	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

type queryAccessRequest struct {
	SQL             string                                `json:"sql"`
	Dialect         deltascope.Dialect                    `json:"dialect,omitempty"`
	Mode            string                                `json:"mode,omitempty"`
	DefaultSchema   string                                `json:"default_schema,omitempty"`
	AnalysisProfile deltascope.QueryAccessAnalysisProfile `json:"profile,omitempty"`
	ConnectionID    string                                `json:"connection_id,omitempty"`
}

var (
	analyzeQueryAccess                  = deltascope.AnalyzeQueryAccess
	openOnlineSession                   = online.OpenSession
	newOnlineQueryAccessSessionFromConn = deltascope.NewOnlineQueryAccessSessionFromConn
	analyzeOnlineQueryAccessWithSession = deltascope.AnalyzeOnlineQueryAccessWithSession
)

func handleQueryAccess(w http.ResponseWriter, r *http.Request, registry *runtimeconfig.Registry) {
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

	if strings.TrimSpace(request.ConnectionID) != "" {
		if request.AnalysisProfile != deltascope.QueryAccessAnalysisProfileEmpty {
			writeError(w, http.StatusBadRequest, "invalid_request", "profile is not allowed with connection_id")
			return
		}
		handleQueryAccessOnline(w, r, registry, request, dialect, mode)
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

func handleQueryAccessOnline(
	w http.ResponseWriter,
	r *http.Request,
	registry *runtimeconfig.Registry,
	request queryAccessRequest,
	_ deltascope.Dialect,
	mode deltascope.QueryAccessMode,
) {
	if registry == nil {
		writeError(w, http.StatusNotFound, "connection_not_found", "connection not found")
		return
	}

	conn, ok := registry.LookupConnection(request.ConnectionID)
	if !ok {
		writeError(w, http.StatusNotFound, "connection_not_found", "connection not found")
		return
	}

	principalID := PrincipalIDFromContext(r.Context())
	if err := registry.Authorize(principalID, request.ConnectionID, "query_access"); err != nil {
		mappedErr := mapRegistryAuthorizeError(err)
		status, code := mapOnlineSessionError(mappedErr)
		writeError(w, status, code, mapOnlineSessionErrorMessage(mappedErr))
		return
	}

	connectTimeout, _, _ := runtimeconfig.ParseConnectTimeout(conn.ConnectTimeout)

	connDialect := strings.ToLower(strings.TrimSpace(conn.Dialect))
	configuredSchema := strings.TrimSpace(conn.Schema)
	schema := configuredSchema
	database := strings.TrimSpace(conn.Database)
	if connDialect == "mysql" || connDialect == "tidb" {
		var err error
		database, schema, err = appqa.ResolveMySQLTiDBOnlineSchema(connDialect, database, configuredSchema, request.DefaultSchema)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "database, schema, and default_schema must match when set; use one catalog value")
			return
		}
	} else if strings.TrimSpace(request.DefaultSchema) != "" {
		schema = strings.TrimSpace(request.DefaultSchema)
	}

	sessionCfg := online.SessionConfig{
		Host:           strings.TrimSpace(conn.Host),
		Port:           conn.Port,
		Socket:         strings.TrimSpace(conn.Socket),
		User:           strings.TrimSpace(conn.User),
		Password:       conn.ResolvedPassword(),
		Database:       database,
		Schema:         schema,
		Dialect:        connDialect,
		ConnectTimeout: connectTimeout,
		TLSMode:        strings.ToLower(strings.TrimSpace(conn.TLSMode)),
		CACert:         conn.ResolvedCACert(),
	}

	session, err := openOnlineSession(r.Context(), sessionCfg)
	if err != nil {
		status, code := mapOnlineSessionError(err)
		writeError(w, status, code, mapOnlineSessionErrorMessage(err))
		return
	}
	defer session.Close()

	queryAccessSession, err := newOnlineQueryAccessSessionFromConn(r.Context(), session.Conn)
	if err != nil {
		code, message := mapOnlineQueryAccessConstructorError(err)
		writeError(w, http.StatusBadGateway, code, message)
		return
	}
	result, err := analyzeOnlineQueryAccessWithSession(r.Context(), queryAccessSession, deltascope.QueryAccessRequest{
		SQL:           request.SQL,
		Mode:          mode,
		DefaultSchema: schema,
	})
	if err != nil {
		status, code := mapQueryAccessError(err)
		writeError(w, status, code, mapQueryAccessErrorMessage(err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func mapOnlineQueryAccessConstructorError(err error) (string, string) {
	if errors.Is(err, deltascope.ErrOnlineQueryAccessPostgreSQLVersionUnsupported) {
		return "identity_error", online.PostgreSQLQueryAccessVersionRequirement
	}
	return "connection_failed", "connection failed"
}

func mapOnlineSessionError(err error) (int, string) {
	code, _, status := online.MapOnlineError(err)
	if status != 0 {
		return status, code
	}
	return http.StatusBadGateway, "connection_failed"
}

func mapOnlineSessionErrorMessage(err error) string {
	_, message, _ := online.MapOnlineError(err)
	if message != "" && message != "internal server error" {
		return message
	}
	return "connection failed"
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
