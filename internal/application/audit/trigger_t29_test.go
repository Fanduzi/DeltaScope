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
		"dml.where.require":                           {Enabled: true},
		"ddl.create_trigger.notice":                   {Enabled: true},
		"ddl.drop_trigger.notice":                     {Enabled: true},
		"ddl.create_trigger.ordering.version.require": {Enabled: true},
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
	u := t29Feature(res, "trigger.body.old_new")
	if u == nil {
		t.Fatal("OLD in an INSERT trigger must surface trigger.body.old_new")
	}
	cols, _ := u.Metadata["old_columns"].([]string)
	if len(cols) != 1 || cols[0] != "y" {
		t.Fatalf("old_columns must pin the OLD column dependency, got %v", u.Metadata["old_columns"])
	}
}

func TestT29NewOnDeleteInvalidBounded(t *testing.T) {
	res := t29Evaluate(t, "CREATE TRIGGER tr AFTER DELETE ON t FOR EACH ROW INSERT INTO audit_log VALUES(NEW.id)", spec.DialectMySQL)
	u := t29Feature(res, "trigger.body.old_new")
	if u == nil {
		t.Fatal("NEW in a DELETE trigger must surface trigger.body.old_new")
	}
	cols, _ := u.Metadata["new_columns"].([]string)
	if len(cols) != 1 || cols[0] != "id" {
		t.Fatalf("new_columns must pin the NEW column dependency, got %v", u.Metadata["new_columns"])
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

func TestT29IllegalOldNewMemberBoundsWhileSiblingsAudit(t *testing.T) {
	res := t29Evaluate(t, "CREATE TRIGGER tr BEFORE INSERT ON t FOR EACH ROW BEGIN DELETE FROM audit_log; UPDATE t SET x = OLD.y; END", spec.DialectMySQL)
	var blocker bool
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" && f.Metadata["trigger_member"] == 0 {
			blocker = true
		}
	}
	if !blocker {
		t.Fatalf("sibling member 0 must still audit its static risk, got %+v", res.Statements[0].Findings)
	}
	u := t29Feature(res, "trigger.body.old_new")
	if u == nil {
		t.Fatal("illegal OLD use must bound the offending member, not the statement")
	}
	if u.Metadata["trigger_member"] != 1 {
		t.Fatalf("the boundary must name member 1, got %v", u.Metadata)
	}
}

func TestT29OrderingVersionBoundary(t *testing.T) {
	const orderingSQL = "CREATE TRIGGER tr BEFORE INSERT ON t FOR EACH ROW FOLLOWS other DELETE FROM audit_log"
	mysql := func(version string, major, minor, patch int) *spec.VersionIdentity {
		return &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: version, Major: major, Minor: minor, Patch: patch, ValidatedRange: true}
	}
	for _, tc := range []struct {
		name        string
		version     *spec.VersionIdentity
		wantFinding bool
		wantGap     bool
	}{
		{name: "pre-5.7.2 incompatible", version: mysql("5.7.0", 5, 7, 0), wantFinding: true},
		{name: "5.7.44 supported", version: mysql("5.7.44", 5, 7, 44)},
		{name: "8.0.46 supported", version: mysql("8.0.46", 8, 0, 46)},
		{name: "missing version gap", version: nil, wantGap: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := Parse(context.Background(), orderingSQL, spec.DialectMySQL)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			stmts, err := Extract(context.Background(), parsed)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if tc.version != nil {
				stmts[0].Metadata = &spec.Metadata{Version: tc.version}
			}
			res, err := EvaluateStatements(context.Background(), t29Registry(t), stmts)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			var orderingFinding, versionGap bool
			for _, f := range res.Statements[0].Findings {
				if f.RuleID == "ddl.create_trigger.ordering.version.require" {
					orderingFinding = true
				}
			}
			for _, g := range res.Statements[0].EvidenceGaps {
				if g.RuleID == "ddl.create_trigger.ordering.version.require" && g.ReasonCode == "missing_target_version" {
					versionGap = true
				}
			}
			if orderingFinding != tc.wantFinding {
				t.Fatalf("ordering finding=%v want %v (findings %+v)", orderingFinding, tc.wantFinding, res.Statements[0].Findings)
			}
			if versionGap != tc.wantGap {
				t.Fatalf("version gap=%v want %v (gaps %+v)", versionGap, tc.wantGap, res.Statements[0].EvidenceGaps)
			}
		})
	}
}
