package mcpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const t18MCPViewSQL = "CREATE VIEW fresh AS SELECT id FROM visible_v;"

type t18MCPViewClient struct {
	metadataOnlyClient
	mode  string
	mu    sync.Mutex
	reads []string
}

func (*t18MCPViewClient) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "8.0.46"}, nil
}

func (p *t18MCPViewClient) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	p.mu.Lock()
	p.reads = append(p.reads, schema+"."+table)
	p.mu.Unlock()
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

func (p *t18MCPViewClient) hasRead(value string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, read := range p.reads {
		if read == value {
			return true
		}
	}
	return false
}

type t18MCPViewLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *t18MCPViewLog) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(data)
}

func (b *t18MCPViewLog) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func t18MCPViewPolicy(t *testing.T) string {
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

func TestT18MCPViewPrivacy(t *testing.T) {
	for _, mode := range []string{"known", "unavailable", "error"} {
		t.Run(mode, func(t *testing.T) {
			client := &t18MCPViewClient{mode: mode}
			previous := prepareMetadataAudit
			prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
				return &auditmeta.PreparedAudit{
					Client: client, Dialect: spec.DialectMySQL, Schema: "golden",
					DialectSource: "request", SchemaSource: "request",
				}, nil
			}
			t.Cleanup(func() { prepareMetadataAudit = previous })
			var logs t18MCPViewLog
			server := NewServer(Config{Version: "test-build", Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
			session, err := connectClientSession(context.Background(), server)
			if err != nil {
				t.Fatalf("connect session: %v", err)
			}
			t.Cleanup(func() { _ = session.Close() })
			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name: "audit_sql",
				Arguments: map[string]any{
					"sql": t18MCPViewSQL, "dialect": "mysql", "config_path": t18MCPViewPolicy(t),
					"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "golden"},
				},
			})
			if err != nil {
				t.Fatalf("call audit_sql: %v", err)
			}
			raw, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Content) == 0 {
				t.Fatal("MCP result has no rendered content")
			}
			hasText := false
			for _, content := range result.Content {
				_, ok := content.(*sdkmcp.TextContent)
				hasText = hasText || ok
			}
			if !hasText {
				t.Fatal("MCP result has no rendered text")
			}
			for _, sentinel := range []string{
				"t18_private_literal", "t18_private_definer", "t18_private_hidden_leaf", "t18_private_provider_error",
			} {
				if strings.Contains(string(raw), sentinel) || strings.Contains(logs.String(), sentinel) {
					t.Fatal("private catalog or provider value leaked through MCP JSON or logs")
				}
				for _, content := range result.Content {
					if text, ok := content.(*sdkmcp.TextContent); ok && strings.Contains(text.Text, sentinel) {
						t.Fatal("private catalog or provider value leaked through MCP text")
					}
				}
			}
			if !client.hasRead("golden.visible_v") {
				t.Fatal("view provider was not consulted")
			}
			body, ok := result.StructuredContent.(map[string]any)
			if !ok {
				t.Fatalf("structured content has type %T", result.StructuredContent)
			}
			if mode == "error" {
				if !result.IsError || body["code"] != "bad_request" || body["verdict"] != nil ||
					body["coverage"] != nil || body["statements"] != nil {
					t.Fatal("provider error did not remain a bad_request tool error")
				}
				return
			}
			if result.IsError {
				t.Fatal("normal audit unexpectedly returned a tool error")
			}
			bodyJSON, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			var audit deltascope.Result
			if err := json.Unmarshal(bodyJSON, &audit); err != nil {
				t.Fatal(err)
			}
			t18MCPAssertViewAudit(t, audit, mode)
			if got := client.hasRead("golden.t18_private_hidden_leaf"); got != (mode == "known") {
				t.Fatalf("hidden dependency read=%t, want %t", got, mode == "known")
			}
		})
	}
}

func t18MCPAssertViewAudit(t *testing.T, result deltascope.Result, mode string) {
	t.Helper()
	if len(result.Statements) != 1 || len(result.Diagnostics) != 0 || len(result.Unsupported) != 0 ||
		result.Summary.Statements != 1 || result.Summary.Warnings != 1 || result.Summary.Notices != 0 {
		t.Fatal("unexpected MCP audit statement, diagnostics, or summary")
	}
	statement := result.Statements[0]
	if statement.Index != 0 || statement.RawSQL != t18MCPViewSQL {
		t.Fatal("MCP statement did not preserve the authored index and SQL")
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
					finding.Location.Line == 1 && finding.Location.Column == strings.Index(t18MCPViewSQL, "visible_v")+1
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
