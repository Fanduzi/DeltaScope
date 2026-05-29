// Package mcpapi exposes the MCP adapter for DeltaScope.
// input: adapter and audit errors arising during MCP tool execution
// output: stable MCP tool error payloads with machine-readable codes and human-readable messages
// pos: shared error-shaping helpers for MCP tool handlers
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"errors"
	"strings"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type toolErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func toolError(code, message string) (*sdkmcp.CallToolResult, error) {
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: message},
		},
		StructuredContent: toolErrorPayload{Code: code, Message: message},
	}, nil
}

type toolDiagnosticErrorPayload struct {
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	Diagnostics []spec.Diagnostic `json:"diagnostics,omitempty"`
}

func toolDiagnosticError(code, message string, diagnostics []spec.Diagnostic) (*sdkmcp.CallToolResult, error) {
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: message},
		},
		StructuredContent: toolDiagnosticErrorPayload{Code: code, Message: message, Diagnostics: diagnostics},
	}, nil
}

func mapAuditToolError(err error) string {
	var prepErr *auditmeta.Error
	if errors.As(err, &prepErr) {
		switch prepErr.Kind {
		case auditmeta.ErrorInvalidSQL, auditmeta.ErrorDialectMismatch:
			return "bad_request"
		case auditmeta.ErrorSchemaHintRequired:
			return "connection_invalid"
		case auditmeta.ErrorSchemaLookupFailed, auditmeta.ErrorConnectionOpen, auditmeta.ErrorDialectDetect:
			return "connection_failed"
		}
	}

	switch {
	case errors.Is(err, appaudit.ErrEmptySQL), errors.Is(err, appaudit.ErrUnknownDialect):
		return "bad_request"
	case strings.Contains(err.Error(), "load policy:"),
		strings.Contains(err.Error(), "read connections config:"),
		strings.Contains(err.Error(), "connection_ref config:"):
		return "config_invalid"
	case strings.Contains(err.Error(), "connection_ref"),
		strings.Contains(err.Error(), "password source"),
		strings.Contains(err.Error(), "password env"),
		strings.Contains(err.Error(), "read password file:"),
		strings.Contains(err.Error(), "connection must include at least one non-password field"),
		strings.Contains(err.Error(), "connection must include host/user, socket/user, or connection_ref"),
		strings.Contains(err.Error(), "connection socket cannot be combined"),
		strings.Contains(err.Error(), "connection connect_timeout"):
		return "connection_invalid"
	case strings.Contains(err.Error(), "open metadata connection"), strings.Contains(err.Error(), "detect dialect"), strings.Contains(err.Error(), "resolve schema for table"):
		return "connection_failed"
	default:
		return "bad_request"
	}
}
