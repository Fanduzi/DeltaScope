package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

const t18HTTPViewSQL = "CREATE VIEW fresh AS SELECT id FROM visible_v;"

type t18HTTPViewProvider struct {
	mode  string
	reads []string
}

func (*t18HTTPViewProvider) LoadInstanceFacts(context.Context, deltascope.Dialect, string) (*deltascope.InstanceFacts, error) {
	return &deltascope.InstanceFacts{Version: "8.0.46"}, nil
}

func (p *t18HTTPViewProvider) LoadTableSnapshot(_ context.Context, _ deltascope.Dialect, schema, table string) (*deltascope.TableSnapshot, error) {
	p.reads = append(p.reads, schema+"."+table)
	identity := &spec.Table{Schema: schema, Name: table}
	switch table {
	case "fresh":
		return &spec.TableSnapshot{Schema: schema, Table: identity}, nil
	case "visible_v":
		if p.mode == "error" {
			return nil, errors.New("t18_private_provider_error")
		}
		view := &spec.ViewSnapshot{
			Security: "definer", DefinerKind: "named", DefinerPrincipal: "t18_private_definer",
		}
		if p.mode == "known" {
			view.Definition = &spec.ViewDefinition{
				SQL:          "SELECT id FROM golden.t18_private_hidden_leaf WHERE 't18_private_literal'='t18_private_literal'",
				Sources:      []spec.Table{{Schema: "golden", Name: "t18_private_hidden_leaf"}},
				SourcesKnown: true, ShapeKnown: true,
				Bindings: []spec.ViewColumnBinding{{Column: "id", Output: "id"}},
			}
		}
		return &spec.TableSnapshot{
			Schema: schema, Table: identity, Exists: true, RelationKind: "view", View: view,
			Columns: []spec.Column{{Name: "id", Type: "int"}},
		}, nil
	case "t18_private_hidden_leaf":
		return &spec.TableSnapshot{
			Schema: schema, Table: identity, Exists: true, RelationKind: "table",
			Columns: []spec.Column{{Name: "id", Type: "int"}},
		}, nil
	default:
		return nil, nil
	}
}

func t18HTTPViewPolicy(t *testing.T) string {
	t.Helper()
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var config strings.Builder
	config.WriteString("rules:\n")
	for _, id := range ids {
		switch id {
		case "ddl.table.denylist.forbid":
			fmt.Fprintf(&config, "  %s:\n    enabled: true\n    level: blocker\n    params:\n      tables: [t18_private_hidden_leaf]\n", strconv.Quote(id))
		case "ddl.view.source.valid":
			fmt.Fprintf(&config, "  %s:\n    enabled: true\n    level: blocker\n", strconv.Quote(id))
		case "ddl.view.security.definer.warn":
			fmt.Fprintf(&config, "  %s:\n    enabled: true\n    level: warning\n", strconv.Quote(id))
		default:
			fmt.Fprintf(&config, "  %s:\n    enabled: false\n", strconv.Quote(id))
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(config.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestT18HTTPViewPrivacy(t *testing.T) {
	for _, mode := range []string{"known", "unavailable", "error"} {
		t.Run(mode, func(t *testing.T) {
			provider := &t18HTTPViewProvider{mode: mode}
			var logs bytes.Buffer
			handler, err := NewHandler(t18HTTPViewPolicy(t), "test-build",
				WithAuditFunc(func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
					request.MetadataProvider = provider
					return deltascope.Audit(ctx, request)
				}),
				WithMiddlewareConfig(MiddlewareConfig{Logger: log.New(&logs, "", 0)}),
			)
			if err != nil {
				t.Fatalf("new handler: %v", err)
			}
			body, err := json.Marshal(map[string]any{"sql": t18HTTPViewSQL, "dialect": "mysql", "schema": "golden"})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			for _, sentinel := range []string{
				"t18_private_literal", "t18_private_definer", "t18_private_hidden_leaf", "t18_private_provider_error",
			} {
				if strings.Contains(rec.Body.String(), sentinel) || strings.Contains(logs.String(), sentinel) {
					t.Fatal("private catalog or provider value leaked through HTTP response or logs")
				}
			}
			if logs.Len() == 0 {
				t.Fatal("HTTP access logging was not captured")
			}
			if !containsT18HTTPViewRead(provider.reads, "golden.visible_v") {
				t.Fatal("view provider was not consulted")
			}
			if mode == "error" {
				var envelope struct {
					Error struct {
						Code    string `json:"code"`
						Message string `json:"message"`
					} `json:"error"`
					Verdict    *string         `json:"verdict"`
					Coverage   json.RawMessage `json:"coverage"`
					Statements json.RawMessage `json:"statements"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				if rec.Code != http.StatusBadRequest || envelope.Error.Code != "bad_request" ||
					envelope.Error.Message != "invalid request" || envelope.Verdict != nil ||
					envelope.Coverage != nil || envelope.Statements != nil {
					t.Fatalf("status=%d error=%#v", rec.Code, envelope.Error)
				}
				return
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d, want 200", rec.Code)
			}
			var result deltascope.Result
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			t18HTTPAssertViewAudit(t, result, mode)
			if got := containsT18HTTPViewRead(provider.reads, "golden.t18_private_hidden_leaf"); got != (mode == "known") {
				t.Fatalf("hidden dependency read=%t, want %t", got, mode == "known")
			}
		})
	}
}

func containsT18HTTPViewRead(reads []string, value string) bool {
	for _, read := range reads {
		if read == value {
			return true
		}
	}
	return false
}

func t18HTTPAssertViewAudit(t *testing.T, result deltascope.Result, mode string) {
	t.Helper()
	if len(result.Statements) != 1 || len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 ||
		result.Summary.Statements != 1 || result.Summary.Warnings != 1 || result.Summary.Notices != 0 {
		t.Fatal("unexpected HTTP audit statement, diagnostics, or summary")
	}
	statement := result.Statements[0]
	if statement.Index != 0 || statement.RawSQL != t18HTTPViewSQL {
		t.Fatal("HTTP statement did not preserve the authored index and SQL")
	}
	if mode == "known" {
		if result.Verdict != deltascope.VerdictReject || result.Coverage.Status != deltascope.CoverageComplete ||
			statement.Coverage.Status != deltascope.CoverageComplete ||
			result.Summary.Blockers != 1 || len(statement.Findings) != 2 || len(statement.EvidenceGaps) != 0 {
			t.Fatal("known definition did not produce complete reject with two findings")
		}
		var deny, security bool
		for _, finding := range statement.Findings {
			switch finding.RuleID {
			case "ddl.table.denylist.forbid":
				deny = finding.Level == deltascope.LevelBlocker && finding.Metadata["table"] == "visible_v" &&
					finding.Metadata["schema"] == "golden" && finding.Location != nil &&
					finding.Location.Line == 1 && finding.Location.Column == strings.Index(t18HTTPViewSQL, "visible_v")+1
			case "ddl.view.security.definer.warn":
				security = finding.Level == deltascope.LevelWarning
			}
		}
		if !deny || !security {
			t.Fatal("known definition findings did not identify the authored view and security warning")
		}
		return
	}
	if result.Verdict != deltascope.VerdictReview || result.Coverage.Status != deltascope.CoverageUnverified ||
		statement.Coverage.Status != deltascope.CoverageUnverified ||
		result.Summary.Blockers != 0 || len(statement.Findings) != 1 ||
		statement.Findings[0].RuleID != "ddl.view.security.definer.warn" ||
		statement.Findings[0].Level != deltascope.LevelWarning || len(statement.EvidenceGaps) != 2 {
		t.Fatal("unavailable definition did not produce unverified review with one warning and two gaps")
	}
	gaps := map[string]int{}
	for _, gap := range statement.EvidenceGaps {
		if !reflect.DeepEqual(gap.RequiredFacts, []string{"view.definition"}) {
			t.Fatal("unavailable definition gap has unexpected required facts")
		}
		gaps[gap.RuleID]++
	}
	if gaps["ddl.table.denylist.forbid"] != 1 || gaps["ddl.view.source.valid"] != 1 {
		t.Fatal("unavailable definition gaps did not belong to source and denylist rules")
	}
}
