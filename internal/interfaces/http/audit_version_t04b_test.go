// Package httpapi verifies the T04-B/#83 target-version HTTP contract.
// input: HTTP audit JSON carrying target_version under the isolated key-length policy
// output: HTTP 200 with canonical version projection/evidence gaps, HTTP 400 for malformed or mismatched input
// pos: HTTP transport contract tests for version evidence (issue #83 T04-B slice)
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

const t04bKeyLengthRuleID = "ddl.index.key_length.max_bytes.require"
const t04bGoldenSQL = "CREATE TABLE t (c VARCHAR(255) CHARACTER SET utf8mb4, KEY idx_c (c)) ENGINE=InnoDB ROW_FORMAT=DYNAMIC;"

func writeT04BIsolatedPolicy(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t04b-isolated.yaml")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, id := range ids {
		if id == t04bKeyLengthRuleID {
			builder.WriteString("  " + strconv.Quote(id) + ":\n    enabled: true\n    level: blocker\n    params:\n      required: true\n")
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

func postT04BAudit(t *testing.T, handler http.Handler, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v\nbody=%s", err, rec.Body.String())
	}
	return rec.Code, payload
}

// The HTTP surface forwards target_version to the shared parser: malformed is
// 400, absent produces the bounded gap, valid projects the identity.
func TestHandlerT04BTargetVersionContract(t *testing.T) {
	handler, err := NewHandler(writeT04BIsolatedPolicy(t), "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	sqlJSON, _ := json.Marshal(t04bGoldenSQL)

	t.Run("absent version returns gap", func(t *testing.T) {
		code, payload := postT04BAudit(t, handler, fmt.Sprintf(`{"sql":%s,"dialect":"mysql"}`, sqlJSON))
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %#v", code, payload)
		}
		if c, _ := payload["coverage"].(map[string]any); c["status"] != "unverified" {
			t.Fatalf("expected unverified, got %#v", payload["coverage"])
		}
		stmts, _ := payload["statements"].([]any)
		first, _ := stmts[0].(map[string]any)
		gaps, _ := first["evidence_gaps"].([]any)
		if len(gaps) != 1 {
			t.Fatalf("expected one gap, got %#v", gaps)
		}
		gap, _ := gaps[0].(map[string]any)
		if gap["reason_code"] != "missing_target_version" {
			t.Fatalf("gap = %#v", gap)
		}
	})

	t.Run("valid version returns identity", func(t *testing.T) {
		code, payload := postT04BAudit(t, handler, fmt.Sprintf(`{"sql":%s,"dialect":"mysql","target_version":"8.4.10"}`, sqlJSON))
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %#v", code, payload)
		}
		version, _ := payload["version"].(map[string]any)
		if version["product"] != "mysql" || version["version"] != "8.4.10" || version["source"] != "target" {
			t.Fatalf("version = %#v", version)
		}
	})

	t.Run("malformed version is bad request", func(t *testing.T) {
		code, payload := postT04BAudit(t, handler, fmt.Sprintf(`{"sql":%s,"dialect":"mysql","target_version":"8.4"}`, sqlJSON))
		if code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %#v", code, payload)
		}
		errBody, _ := payload["error"].(map[string]any)
		if errBody["code"] != "bad_request" {
			t.Fatalf("expected bad_request code, got %#v", payload["error"])
		}
	})

	t.Run("out-of-range version returns range gap", func(t *testing.T) {
		code, payload := postT04BAudit(t, handler, fmt.Sprintf(`{"sql":%s,"dialect":"mysql","target_version":"9.0.0"}`, sqlJSON))
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %#v", code, payload)
		}
		stmts, _ := payload["statements"].([]any)
		first, _ := stmts[0].(map[string]any)
		gaps, _ := first["evidence_gaps"].([]any)
		if len(gaps) != 1 {
			t.Fatalf("expected one gap, got %#v", gaps)
		}
		if gap, _ := gaps[0].(map[string]any); gap["reason_code"] != "target_version_out_of_validated_range" {
			t.Fatalf("gap = %#v", gap)
		}
	})
}

// A malformed target_version is validated before any connection lookup or
// open: even a nonexistent connection must lose to the input error.
func TestExecuteAuditRequestT04BPreflightPrecedesConnection(t *testing.T) {
	previous := prepareHTTPMetadataAudit
	prepareCalled := false
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		prepareCalled = true
		return nil, errors.New("prepare must not run before target_version preflight")
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	reg := newTestRegistry(t, "test-conn")
	_, err := executeAuditRequest(context.Background(), auditRequest{
		SQL:           t04bGoldenSQL,
		Dialect:       "mysql",
		TargetVersion: "8.4",
		ConnectionID:  "nonexistent",
	}, "", func(context.Context, deltascope.Request) (deltascope.Result, error) {
		t.Fatalf("auditFn must not run for malformed target_version")
		return deltascope.Result{}, nil
	}, MetadataConfig{}, reg, "default-key")
	if !errors.Is(err, appaudit.ErrInvalidTargetVersion) {
		t.Fatalf("expected ErrInvalidTargetVersion before connection lookup, got %v", err)
	}
	if prepareCalled {
		t.Fatalf("metadata prepare must not run before target_version preflight")
	}
}
