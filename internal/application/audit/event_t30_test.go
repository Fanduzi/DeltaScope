package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/ddl"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/dml"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// event_t30_test.go pins the T30 first golden path (issue #109): event
// definitions model their ON SCHEDULE/completion/enabled state and audit
// the DO body through the same bounded member machinery — a DISABLED
// event never skips body audit, an ENABLED one surfaces the scheduled
// execution warning, and TiDB stays a vendor boundary. Nothing executes
// the body or mutates outer state.

func t30Registry(t *testing.T) *rule.Registry {
	t.Helper()
	registry := rule.NewRegistry()
	cfg := policy.Policy{Rules: map[string]policy.RulePolicy{
		"dml.where.require":                {Enabled: true},
		"ddl.create_event.notice":          {Enabled: true},
		"ddl.alter_event.notice":           {Enabled: true},
		"ddl.drop_event.notice":            {Enabled: true},
		"ddl.event.enabled_execution.warn": {Enabled: true},
	}}
	if err := dml.Register(registry, cfg); err != nil {
		t.Fatalf("register dml: %v", err)
	}
	if err := ddl.Register(registry, cfg); err != nil {
		t.Fatalf("register ddl: %v", err)
	}
	return registry
}

func t30Evaluate(t *testing.T, sql string, dialect spec.Dialect) report.Result {
	t.Helper()
	parsed, err := Parse(context.Background(), sql, dialect)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	stmts, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	res, err := EvaluateStatements(context.Background(), t30Registry(t), stmts)
	if err != nil {
		t.Fatalf("evaluate %q: %v", sql, err)
	}
	return res
}

func TestT30GoldenPathDisabledEventBodyAudited(t *testing.T) {
	res := t30Evaluate(t, "CREATE EVENT golden_ev ON SCHEDULE EVERY 1 DAY DISABLE DO DELETE FROM t", spec.DialectMySQL)
	var blocker *rule.Finding
	for i := range res.Statements[0].Findings {
		if res.Statements[0].Findings[i].RuleID == "dml.where.require" && res.Statements[0].Findings[i].Level == rule.LevelBlocker {
			blocker = &res.Statements[0].Findings[i]
		}
	}
	if blocker == nil {
		t.Fatalf("a DISABLED event must still audit its body DELETE, got %+v", res.Statements[0].Findings)
	}
	if blocker.Metadata["event"] != "golden_ev" {
		t.Fatalf("expected event=golden_ev metadata, got %+v", blocker.Metadata)
	}
	if res.Verdict != report.VerdictReject {
		t.Fatalf("expected reject verdict, got %v", res.Verdict)
	}
	var notice, enabledWarn bool
	for _, f := range res.Statements[0].Findings {
		switch f.RuleID {
		case "ddl.create_event.notice":
			notice = true
		case "ddl.event.enabled_execution.warn":
			enabledWarn = true
		}
	}
	if !notice {
		t.Fatal("create event must emit its lifecycle notice")
	}
	if enabledWarn {
		t.Fatal("a DISABLED event must not claim scheduled execution risk")
	}
}

func TestT30EnabledEveryEventWarns(t *testing.T) {
	res := t30Evaluate(t, "CREATE EVENT ev ON SCHEDULE EVERY 1 HOUR ENABLE DO SELECT 1", spec.DialectMySQL)
	var warn *rule.Finding
	for i := range res.Statements[0].Findings {
		if res.Statements[0].Findings[i].RuleID == "ddl.event.enabled_execution.warn" {
			warn = &res.Statements[0].Findings[i]
		}
	}
	if warn == nil {
		t.Fatalf("an ENABLED event must surface scheduled execution risk, got %+v", res.Statements[0].Findings)
	}
	if warn.Metadata["repeating"] != true || warn.Metadata["schedule_kind"] != "every" {
		t.Fatalf("the warning must record the repeating EVERY schedule, got %+v", warn.Metadata)
	}
	if warn.Metadata["event"] != "ev" {
		t.Fatalf("the warning must name the event, got %+v", warn.Metadata)
	}
}

func TestT30EnabledAtEventNotRepeating(t *testing.T) {
	res := t30Evaluate(t, "CREATE EVENT ev ON SCHEDULE AT CURRENT_TIMESTAMP DO SELECT 1", spec.DialectMySQL)
	var warn *rule.Finding
	for i := range res.Statements[0].Findings {
		if res.Statements[0].Findings[i].RuleID == "ddl.event.enabled_execution.warn" {
			warn = &res.Statements[0].Findings[i]
		}
	}
	if warn == nil {
		t.Fatal("a one-shot ENABLED event still schedules execution")
	}
	if warn.Metadata["repeating"] != false || warn.Metadata["schedule_kind"] != "at" {
		t.Fatalf("the warning must record the one-shot AT schedule, got %+v", warn.Metadata)
	}
}

func TestT30AlterEventEnableWarns(t *testing.T) {
	res := t30Evaluate(t, "ALTER EVENT ev ENABLE", spec.DialectMySQL)
	var warn, notice bool
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "ddl.event.enabled_execution.warn" && f.Metadata["event"] == "ev" {
			warn = true
		}
		if f.RuleID == "ddl.alter_event.notice" {
			notice = true
		}
	}
	if !warn {
		t.Fatalf("ALTER EVENT ... ENABLE schedules execution and must warn, got %+v", res.Statements[0].Findings)
	}
	if !notice {
		t.Fatal("ALTER EVENT must emit its lifecycle notice")
	}
}

func TestT30AlterEventDisableNoWarn(t *testing.T) {
	res := t30Evaluate(t, "ALTER EVENT ev DISABLE", spec.DialectMySQL)
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "ddl.event.enabled_execution.warn" {
			t.Fatalf("ALTER EVENT ... DISABLE must not warn: %+v", f)
		}
	}
}

func TestT30DisableOnSlaveStillSchedules(t *testing.T) {
	for _, sql := range []string{
		"CREATE EVENT ev ON SCHEDULE EVERY 1 DAY DISABLE ON SLAVE DO SELECT 1",
		"ALTER EVENT ev DISABLE ON SLAVE",
	} {
		res := t30Evaluate(t, sql, spec.DialectMySQL)
		var warn *rule.Finding
		for i := range res.Statements[0].Findings {
			if res.Statements[0].Findings[i].RuleID == "ddl.event.enabled_execution.warn" {
				warn = &res.Statements[0].Findings[i]
			}
		}
		if warn == nil {
			t.Fatalf("%q: DISABLE ON SLAVE still executes on the primary and must warn, got %+v", sql, res.Statements[0].Findings)
		}
		if warn.Metadata["status"] != "disable_on_slave" {
			t.Fatalf("%q: the warning must record disable_on_slave status, got %+v", sql, warn.Metadata)
		}
	}
}

func TestT30AlterWithoutStatusOwesEnabledGap(t *testing.T) {
	res := t30Evaluate(t, "ALTER EVENT ev ON SCHEDULE EVERY 2 DAY", spec.DialectMySQL)
	var gap bool
	for _, g := range res.Statements[0].EvidenceGaps {
		if g.RuleID == "ddl.event.enabled_execution.warn" && g.ReasonCode == "missing_event_state" {
			gap = true
		}
	}
	if !gap {
		t.Fatalf("an ALTER with no status clause must owe event.enabled_state, got %+v", res.Statements[0].EvidenceGaps)
	}
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "ddl.event.enabled_execution.warn" {
			t.Fatalf("an unproven enabled state must not fabricate the warning: %+v", f)
		}
	}
}

func TestT30CompoundBodyMembersAudited(t *testing.T) {
	res := t30Evaluate(t, "CREATE EVENT ev ON SCHEDULE EVERY 1 DAY DISABLE DO BEGIN DELETE FROM audit_log; UPDATE t SET x = 1 WHERE id = 1; END", spec.DialectMySQL)
	var found bool
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" && f.Level == rule.LevelBlocker && f.Metadata["event_member"] == 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("compound member 0 must audit with event_member metadata, got %+v", res.Statements[0].Findings)
	}
}

func TestT30DropEventLifecycle(t *testing.T) {
	res := t30Evaluate(t, "DROP EVENT IF EXISTS ev", spec.DialectMySQL)
	var notice bool
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "ddl.drop_event.notice" {
			notice = true
		}
	}
	if !notice {
		t.Fatalf("expected drop event notice, got %+v", res.Statements[0].Findings)
	}
}

func TestT30TiDBVendorBoundary(t *testing.T) {
	for _, sql := range []string{
		"CREATE EVENT ev ON SCHEDULE EVERY 1 DAY DO SELECT 1",
		"ALTER EVENT ev DISABLE",
		"DROP EVENT ev",
	} {
		res := t30Evaluate(t, sql, spec.DialectTiDB)
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

func TestT30ScheduleAttributesModeled(t *testing.T) {
	parsed, err := Parse(context.Background(), "CREATE EVENT ev ON SCHEDULE EVERY 2 WEEK STARTS '2027-01-01 00:00:00' ENDS '2027-06-01 00:00:00' ON COMPLETION PRESERVE DISABLE DO SELECT 1", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmts, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	decl := stmts[0].DDL.Event
	if decl == nil {
		t.Fatalf("expected an event declaration, got %+v", stmts[0])
	}
	if decl.Schedule == nil || decl.Schedule.Kind != spec.EventScheduleEvery {
		t.Fatalf("expected an EVERY schedule, got %+v", decl.Schedule)
	}
	if decl.Schedule.Starts == "" || decl.Schedule.Ends == "" {
		t.Fatalf("STARTS/ENDS must be captured, got %+v", decl.Schedule)
	}
	if decl.OnCompletion == nil || !*decl.OnCompletion {
		t.Fatalf("ON COMPLETION PRESERVE must be modeled, got %+v", decl.OnCompletion)
	}
	if decl.Status != spec.EventStatusDisabled {
		t.Fatalf("DISABLE must be modeled, got %v", decl.Status)
	}
}

func TestT30AlterEventDoOnlyBody(t *testing.T) {
	res := t30Evaluate(t, "ALTER EVENT ev DO DELETE FROM t", spec.DialectMySQL)
	var blocker *rule.Finding
	for i := range res.Statements[0].Findings {
		f := &res.Statements[0].Findings[i]
		if f.RuleID == "dml.where.require" {
			blocker = f
		}
	}
	if blocker == nil {
		t.Fatal("a DO-only ALTER body must audit through the member machinery")
	}
	if blocker.Metadata["event"] != "ev" {
		t.Fatalf("expected event-scoped metadata, got %v", blocker.Metadata)
	}
	var gap *rule.EvidenceGap
	for i := range res.Statements[0].EvidenceGaps {
		if res.Statements[0].EvidenceGaps[i].ReasonCode == "missing_event_state" {
			gap = &res.Statements[0].EvidenceGaps[i]
		}
	}
	if gap == nil {
		t.Fatal("a status-free ALTER owes the missing_event_state evidence gap")
	}
	parsed, err := Parse(context.Background(), "ALTER EVENT ev DO DELETE FROM t", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmts, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stmts[0].DDL == nil || stmts[0].DDL.Event == nil || stmts[0].DDL.Event.Body == nil {
		t.Fatal("DO-only ALTER must model a replacement body")
	}
}

func TestT30DefinerClauseShape(t *testing.T) {
	const sql = "CREATE DEFINER='root'@'localhost' EVENT ev ON SCHEDULE EVERY 1 DAY DISABLE DO SELECT 1"
	parsed, err := Parse(context.Background(), sql, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stmts, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	decl := stmts[0].DDL.Event
	if decl == nil {
		t.Fatal("expected an event declaration")
	}
	if decl.Definer != "named" {
		t.Fatalf("DEFINER keeps only its clause shape, got %q", decl.Definer)
	}
	res := t30Evaluate(t, sql, spec.DialectMySQL)
	for _, f := range res.Statements[0].Findings {
		for _, v := range f.Metadata {
			if s, ok := v.(string); ok && (s == "root" || s == "localhost") {
				t.Fatalf("account payload leaked into metadata: %v", f.Metadata)
			}
		}
	}
}

func TestT30MalformedClauseFormsDecline(t *testing.T) {
	for _, sql := range []string{
		"CREATE EVENT ev ENABLE ON SCHEDULE EVERY 1 DAY DO SELECT 1",                  // status before schedule
		"CREATE EVENT ev ON SCHEDULE EVERY 1 DAY ENABLE DISABLE DO SELECT 1",          // duplicate status
		"CREATE EVENT ev ON SCHEDULE EVERY 1 DAY ON SCHEDULE EVERY 2 DAY DO SELECT 1", // duplicate schedule
		"CREATE EVENT ev ON SCHEDULE EVERY 1 DAY RENAME TO ev2 DO SELECT 1",           // RENAME is ALTER-only
		"CREATE DEFINER EVENT ev ON SCHEDULE EVERY 1 DAY DO SELECT 1",                 // DEFINER no payload
		"CREATE DEFINER= EVENT ev ON SCHEDULE EVERY 1 DAY DO SELECT 1",                // DEFINER= no value
		"CREATE EVENT ev ON SCHEDULE EVERY 1 DAY COMMENT DO SELECT 1",                 // COMMENT no string
		"ALTER EVENT ev",                                 // bare ALTER claims nothing
		"ALTER EVENT ev ENABLE ENABLE",                   // duplicate status
		"ALTER EVENT ev DISABLE ON SCHEDULE EVERY 1 DAY", // status before schedule
	} {
		parsed, err := Parse(context.Background(), sql, spec.DialectMySQL)
		if err != nil {
			continue // a parser error is a valid decline
		}
		stmts, err := Extract(context.Background(), parsed)
		if err != nil {
			continue
		}
		if len(stmts) > 0 && stmts[0].DDL != nil && stmts[0].DDL.Event != nil {
			t.Errorf("malformed form must not be claimed: %q", sql)
		}
	}
}
