===== MCP REAL AUDIT ERROR EXIT (metadata-aware path) =====
--- audit_tool.go:161-185 — the actual publicapi.Audit call and its error branch
     (a T26 metadataObjectReadError from enrich reaches `err` here;
     Result is the zero value, Diagnostics empty → line 170 false → line 183) ---

	result, err := publicapi.Audit(ctx, publicapi.Request{
		SQL:              input.SQL,
		Dialect:          publicapi.Dialect(prepared.Dialect),
		ConfigPath:       strings.TrimSpace(input.ConfigPath),
		Schema:           prepared.Schema,
		MetadataProvider: publicMetadataProvider{client: prepared.Client},
		TargetVersion:    input.TargetVersion,
	})
	if err != nil {
		if len(result.Diagnostics) > 0 {
			return toolDiagnosticError(mapAuditToolError(err), err.Error(), AuditSQLResult{...}), nil, nil
		}
		toolResult, toolErr := toolError(mapAuditToolError(err), err.Error())
		return toolResult, nil, toolErr
	}

--- audit_tool.go:334-347 — publicMetadataProvider.ResolveObject adapter
     (forwards the provider's ResolveObject; the bounded error travels
     back through AuditSQL -> err above unchanged) ---

func (p publicMetadataProvider) ResolveObject(ctx context.Context, dialect spec.Dialect, request spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	type objectResolver interface {
		ResolveObject(context.Context, spec.Dialect, spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error)
	}
	resolver, ok := p.client.(objectResolver)
	if !ok {
		return &spec.ObjectSnapshot{... Status: spec.MetadataStatusUnavailable}, nil
	}
	return resolver.ResolveObject(ctx, dialect, request)
}

--- tool_errors.go:45-88 — mapAuditToolError classifies the wrapped error;
     "metadata object read failed" is not *auditmeta.Error, not any
     errors.Is sentinel, and contains no keyword substring
     ("load policy:", "connection_ref", "connection", "open metadata
     connection", "detect dialect", "resolve schema for table") → default ---

	default:
		return "bad_request"
	}

--- tool_errors.go:17-25 — toolError places the message verbatim into
     TextContent.Text and StructuredContent.Message (bounded text only) ---

func toolError(code, message string) (*sdkmcp.CallToolResult, error) {
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: message},
		},
		StructuredContent: toolErrorPayload{Code: code, Message: message},
	}, nil
}
