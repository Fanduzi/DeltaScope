// Package mcpapi verifies MCP panic recovery behavior.
// input: tool handler functions that panic intentionally
// output: regression coverage ensuring panics are caught and converted to structured errors
// pos: panic-recovery tests for MCP tool handler wrapper
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"bytes"
	"context"
	"io"
	"log"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRecoverTool_CatchesPanicAndReturnsStructuredError(t *testing.T) {
	var logBuf bytes.Buffer
	panicLog := log.New(&logBuf, "", 0)

	handler := func(_ context.Context, _ *sdkmcp.CallToolRequest, _ getCapabilitiesInput) (*sdkmcp.CallToolResult, any, error) {
		panic("intentional test panic")
	}

	wrapped := recoverTool(handler, panicLog)
	result, structured, err := wrapped(context.Background(), &sdkmcp.CallToolRequest{}, getCapabilitiesInput{})

	if err != nil {
		t.Fatalf("expected nil error from recovered panic, got: %v", err)
	}
	if structured != nil {
		t.Fatalf("expected nil structured content, got: %v", structured)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}

	payload, ok := result.StructuredContent.(toolErrorPayload)
	if !ok {
		t.Fatalf("expected toolErrorPayload, got %T", result.StructuredContent)
	}
	if payload.Code != "internal_error" {
		t.Fatalf("expected internal_error code, got %q", payload.Code)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "MCP panic recovered") {
		t.Error("log should contain panic recovery message")
	}
	if !strings.Contains(logOutput, "intentional test panic") {
		t.Error("log should contain panic message")
	}
	if !strings.Contains(logOutput, "Stack trace:") {
		t.Error("log should contain stack trace")
	}
}

func TestRecoverTool_StaysResponsiveAfterPanic(t *testing.T) {
	var logBuf bytes.Buffer
	panicLog := log.New(&logBuf, "", 0)

	panicHandler := func(_ context.Context, _ *sdkmcp.CallToolRequest, _ getCapabilitiesInput) (*sdkmcp.CallToolResult, any, error) {
		panic("first panic")
	}
	normalHandler := func(_ context.Context, _ *sdkmcp.CallToolRequest, _ getCapabilitiesInput) (*sdkmcp.CallToolResult, any, error) {
		return nil, "success", nil
	}

	wrappedPanic := recoverTool(panicHandler, panicLog)
	result1, _, _ := wrappedPanic(context.Background(), &sdkmcp.CallToolRequest{}, getCapabilitiesInput{})
	if result1 == nil || !result1.IsError {
		t.Fatal("first call should return error result")
	}

	wrappedNormal := recoverTool(normalHandler, panicLog)
	result2, structured2, err2 := wrappedNormal(context.Background(), &sdkmcp.CallToolRequest{}, getCapabilitiesInput{})
	if err2 != nil {
		t.Fatalf("second call should succeed: %v", err2)
	}
	if result2 != nil {
		t.Fatalf("second call should return nil result, got: %v", result2)
	}
	if structured2 != "success" {
		t.Fatalf("expected 'success', got: %v", structured2)
	}
}

func TestRecoverTool_LogsPanicWithStackTrace(t *testing.T) {
	var logBuf bytes.Buffer
	panicLog := log.New(&logBuf, "", 0)

	handler := func(_ context.Context, _ *sdkmcp.CallToolRequest, _ getCapabilitiesInput) (*sdkmcp.CallToolResult, any, error) {
		panic("test panic with stack")
	}

	wrapped := recoverTool(handler, panicLog)
	wrapped(context.Background(), &sdkmcp.CallToolRequest{}, getCapabilitiesInput{})

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "TestRecoverTool_LogsPanicWithStackTrace") {
		t.Errorf("stack trace should contain test function name, got:\n%s", logOutput)
	}
}

func TestRecoverTool_PassesThroughNormalResult(t *testing.T) {
	panicLog := log.New(io.Discard, "", 0)
	normalHandler := func(_ context.Context, _ *sdkmcp.CallToolRequest, _ listRulesInput) (*sdkmcp.CallToolResult, any, error) {
		return nil, listRulesResponse{Count: 5}, nil
	}

	wrapped := recoverTool(normalHandler, panicLog)
	result, structured, err := wrapped(context.Background(), &sdkmcp.CallToolRequest{}, listRulesInput{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for success, got: %v", result)
	}
	resp, ok := structured.(listRulesResponse)
	if !ok {
		t.Fatalf("expected listRulesResponse, got %T", structured)
	}
	if resp.Count != 5 {
		t.Fatalf("expected count 5, got %d", resp.Count)
	}
}
