package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/ddl"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/dml"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// trigger_t29_test.go pins the T29 first golden path (issue #108):
// trigger definitions audit their FOR EACH ROW bodies through the same
// bounded member machinery — OLD/NEW pseudo-columns resolve against the
// trigger target and event instead of tables, TiDB stays a vendor
// boundary, and nothing executes the body or mutates outer state.

func t29Registry(t *testing.T) *rule.Registry {
	t.Helper()
	registry := rule.NewRegistry()
	cfg := policy.Policy{Rules: map[string]policy.RulePolicy{
		"dml.where.require":         {Enabled: true},
		"ddl.create_trigger.notice": {Enabled: true},
		"ddl.drop_trigger.notice":   {Enabled: true},
	}}
	if err := dml.Register(registry, cfg); err != nil {
		t.Fatalf("register dml: %v", err)
	}
	if err := ddl.Register(registry, cfg); err != nil {
		t.Fatalf("register ddl: %v", err)
	}
	return registry
}

func t29Evaluate(t *testing.T, sql string, dialect spec.Dialect) report.Result {
	t.Helper()
	parsed, err := Parse(context.Background(), sql, dialect)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	stmts, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	res, err := EvaluateStatements(context.Background(), t29Registry(t), stmts)
	if err != nil {
		t.Fatalf("evaluate %q: %v", sql, err)
	}
	return res
}

func t29Feature(res report.Result, feature string) *spec.UnsupportedDetail {
	for i := range res.Unsupported {
		if res.Unsupported[i].Feature == feature {
			return &res.Unsupported[i]
		}
	}
	return nil
}

func TestT29GoldenPathTriggerBodyAudited(t *testing.T) {
	res := t29Evaluate(t, "CREATE TRIGGER tr BEFORE INSERT ON t FOR EACH ROW DELETE FROM audit_log", spec.DialectMySQL)
	var blocker *rule.Finding
	for i := range res.Statements[0].Findings {
		if res.Statements[0].Findings[i].RuleID == "dml.where.require" && res.Statements[0].Findings[i].Level == rule.LevelBlocker {
			blocker = &res.Statements[0].Findings[i]
		}
	}
	if blocker == nil {
		t.Fatalf("expected body DELETE blocker, got %+v", res.Statements[0].Findings)
	}
	if blocker.Metadata["trigger"] != "tr" {
		t.Fatalf("expected trigger=tr metadata, got %+v", blocker.Metadata)
	}
	if res.Verdict != report.VerdictReject {
		t.Fatalf("expected reject verdict, got %v", res.Verdict)
	}
}

func TestT29CompoundBodyInnerSQLAudited(t *testing.T) {
	res := t29Evaluate(t, "CREATE TRIGGER tr AFTER UPDATE ON t FOR EACH ROW BEGIN DELETE FROM audit_log; UPDATE t SET x = OLD.y WHERE id = 1; END", spec.DialectMySQL)
	var found bool
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" && f.Level == rule.LevelBlocker {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected compound-member DELETE blocker, got %+v", res.Statements[0].Findings)
	}
}

func TestT29OldNewLegalNotMissingTable(t *testing.T) {
	res := t29Evaluate(t, "CREATE TRIGGER tr BEFORE INSERT ON t FOR EACH ROW SET NEW.a = 1", spec.DialectMySQL)
	for _, f := range res.Statements[0].Findings {
		if strings.Contains(strings.ToLower(f.Message), "new") && f.Level == rule.LevelBlocker {
			t.Fatalf("legal NEW use must not be a missing-table blocker: %+v", f)
		}
	}
	if u := t29Feature(res, "trigger.body.old_new"); u != nil {
		t.Fatalf("legal NEW on INSERT must not gap: %+v", u)
	}
}

func TestT29OldOnInsertInvalidBounded(t *testing.T) {
	res := t29Evaluate(t, "CREATE TRIGGER tr BEFORE INSERT ON t FOR EACH ROW UPDATE t SET x = OLD.y WHERE id = 1", spec.DialectMySQL)
	if u := t29Feature(res, "trigger.body.old_new"); u == nil {
		t.Fatal("OLD in an INSERT trigger must surface trigger.body.old_new")
	}
}

func TestT29NewOnDeleteInvalidBounded(t *testing.T) {
	res := t29Evaluate(t, "CREATE TRIGGER tr AFTER DELETE ON t FOR EACH ROW INSERT INTO audit_log VALUES(NEW.id)", spec.DialectMySQL)
	if u := t29Feature(res, "trigger.body.old_new"); u == nil {
		t.Fatal("NEW in a DELETE trigger must surface trigger.body.old_new")
	}
}

func TestT29DropTriggerLifecycle(t *testing.T) {
	res := t29Evaluate(t, "DROP TRIGGER IF EXISTS tr", spec.DialectMySQL)
	var notice bool
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "ddl.drop_trigger.notice" {
			notice = true
		}
	}
	if !notice {
		t.Fatalf("expected drop trigger notice, got %+v", res.Statements[0].Findings)
	}
}

func TestT29TiDBVendorBoundary(t *testing.T) {
	for _, sql := range []string{
		"CREATE TRIGGER tr BEFORE INSERT ON t FOR EACH ROW SELECT 1",
		"DROP TRIGGER tr",
	} {
		res := t29Evaluate(t, sql, spec.DialectTiDB)
		var vendor bool
		for _, u := range res.Unsupported {
			if u.Reason == spec.UnsupportedVendorBoundaryReason && u.Metadata["boundary"] == "vendor" {
				vendor = true
			}
		}
		if !vendor {
			t.Fatalf("%q: expected TiDB vendor boundary, got %+v", sql, res.Unsupported)
		}
	}
}

func TestT29OrderClauseModeled(t *testing.T) {
	res := t29Evaluate(t, "CREATE TRIGGER tr BEFORE INSERT ON t FOR EACH ROW FOLLOWS other DELETE FROM audit_log", spec.DialectMySQL)
	var blocker bool
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" && f.Level == rule.LevelBlocker {
			blocker = true
		}
	}
	if !blocker {
		t.Fatalf("FOLLOWS form must still audit its body, got %+v", res.Statements[0].Findings)
	}
}
