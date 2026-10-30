package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t16Policy() policy.RulePolicy {
	return policy.RulePolicy{Enabled: true, Params: map[string]any{"requires_metadata": true}}
}

// t16Statement builds a statement with bound projections the way the ordered
// state publishes them: primary snapshot plus aligned TargetTables.
func t16Statement(dialect spec.Dialect, operation spec.DDLOperation, targets []spec.Table, bindings ...*spec.TableSnapshot) spec.Statement {
	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: dialect,
		DDL: &spec.DDL{
			Operation: operation,
			Table:     &spec.Table{Name: targets[0].Name, Schema: targets[0].Schema},
			Targets:   append([]spec.Table(nil), targets...),
			Options:   map[string]string{},
		},
	}
	metadata := &spec.Metadata{Schema: "golden"}
	if len(bindings) > 0 {
		metadata.TargetTable = bindings[0]
		metadata.TargetTables = make([]spec.TargetBinding, len(bindings))
		for i, snapshot := range bindings {
			metadata.TargetTables[i] = spec.TargetBinding{Table: targets[i], Snapshot: snapshot}
		}
	}
	statement.Metadata = metadata
	return statement
}

func t16PresentTable(schema, name string) *spec.TableSnapshot {
	return &spec.TableSnapshot{Exists: true, Schema: schema, Table: &spec.Table{Schema: schema, Name: name}}
}

func t16AbsentTable(schema, name string) *spec.TableSnapshot {
	return &spec.TableSnapshot{Exists: false, Schema: schema, Table: &spec.Table{Schema: schema, Name: name}}
}

func t16TempTable(schema, name string) *spec.TableSnapshot {
	snapshot := t16PresentTable(schema, name)
	snapshot.Temporary = true
	return snapshot
}

func t16MustEval(t *testing.T, r rule.StatementRule, statement spec.Statement) []rule.Finding {
	t.Helper()
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate %s: %v", r.ID(), err)
	}
	return findings
}

func t16Gaps(t *testing.T, r rule.StatementRule, statement spec.Statement) []rule.EvidenceGap {
	t.Helper()
	reporter, ok := r.(rule.EvidenceReporter)
	if !ok {
		t.Fatalf("rule %s must implement rule.EvidenceReporter", r.ID())
	}
	return reporter.EvidenceGaps(statement)
}

func TestT16DropExistenceMultiTarget(t *testing.T) {
	t.Parallel()
	ruleOut, err := newTableOperationExistenceRule(ruleIDTableDropExistsRequire, spec.DDLOperationDropTable, "drop table", rule.LevelBlocker, t16Policy())
	if err != nil {
		t.Fatalf("construct drop rule: %v", err)
	}
	targets := []spec.Table{{Schema: "golden", Name: "a"}, {Schema: "golden", Name: "m"}, {Schema: "golden", Name: "b"}}
	t.Run("missing_member_blocks", func(t *testing.T) {
		statement := t16Statement(spec.DialectMySQL, spec.DDLOperationDropTable, targets,
			t16PresentTable("golden", "a"), t16AbsentTable("golden", "m"), t16PresentTable("golden", "b"))
		findings := t16MustEval(t, ruleOut, statement)
		if len(findings) != 1 {
			t.Fatalf("expected exactly one blocker for the absent member, got %+v", findings)
		}
		if findings[0].Metadata["table"] != "m" {
			t.Fatalf("finding must name the absent member, got %+v", findings[0].Metadata)
		}
	})
	t.Run("all_present_clean", func(t *testing.T) {
		statement := t16Statement(spec.DialectMySQL, spec.DDLOperationDropTable, targets,
			t16PresentTable("golden", "a"), t16PresentTable("golden", "m"), t16PresentTable("golden", "b"))
		if findings := t16MustEval(t, ruleOut, statement); len(findings) != 0 {
			t.Fatalf("all-present multi-drop must stay clean, got %+v", findings)
		}
	})
	t.Run("unknown_member_gaps_only", func(t *testing.T) {
		statement := t16Statement(spec.DialectMySQL, spec.DDLOperationDropTable, targets,
			t16PresentTable("golden", "a"), nil, t16PresentTable("golden", "b"))
		if findings := t16MustEval(t, ruleOut, statement); len(findings) != 0 {
			t.Fatalf("unknown member must not fabricate a finding, got %+v", findings)
		}
		gaps := t16Gaps(t, ruleOut, statement)
		if len(gaps) != 1 || gaps[0].ReasonCode != gapReasonUnknownTableState {
			t.Fatalf("unknown member must emit one unknown_table_state gap, got %+v", gaps)
		}
	})
	t.Run("if_exists_suppresses_absent_member", func(t *testing.T) {
		statement := t16Statement(spec.DialectMySQL, spec.DDLOperationDropTable, targets,
			t16PresentTable("golden", "a"), t16AbsentTable("golden", "m"), t16PresentTable("golden", "b"))
		statement.DDL.Options["if_exists"] = "true"
		if findings := t16MustEval(t, ruleOut, statement); len(findings) != 0 {
			t.Fatalf("IF EXISTS must suppress absent-member findings, got %+v", findings)
		}
	})
	t.Run("if_exists_keeps_unknown_gap", func(t *testing.T) {
		statement := t16Statement(spec.DialectMySQL, spec.DDLOperationDropTable, targets,
			t16PresentTable("golden", "a"), nil, t16PresentTable("golden", "b"))
		statement.DDL.Options["if_exists"] = "true"
		if gaps := t16Gaps(t, ruleOut, statement); len(gaps) != 1 {
			t.Fatalf("IF EXISTS must keep unknown-member gaps, got %+v", gaps)
		}
	})
}

func TestT16CreateExistenceConditional(t *testing.T) {
	t.Parallel()
	ruleOut, err := newTableExistenceRule(ruleIDTableExistsCreateForbid, false, rule.LevelBlocker, t16Policy())
	if err != nil {
		t.Fatalf("construct create-forbid rule: %v", err)
	}
	targets := []spec.Table{{Schema: "golden", Name: "t"}}
	build := func(present bool) spec.Statement {
		var snapshot *spec.TableSnapshot
		if present {
			snapshot = t16PresentTable("golden", "t")
		}
		return t16Statement(spec.DialectMySQL, spec.DDLOperationCreateTable, targets, snapshot)
	}
	t.Run("plain_present_blocks", func(t *testing.T) {
		if findings := t16MustEval(t, ruleOut, build(true)); len(findings) != 1 {
			t.Fatalf("plain create on present must block, got %+v", findings)
		}
	})
	t.Run("if_not_exists_suppresses_present", func(t *testing.T) {
		statement := build(true)
		statement.DDL.Options["if_not_exists"] = "true"
		if findings := t16MustEval(t, ruleOut, statement); len(findings) != 0 {
			t.Fatalf("IF NOT EXISTS must suppress the present finding, got %+v", findings)
		}
	})
	t.Run("if_not_exists_keeps_unknown_gap", func(t *testing.T) {
		statement := build(false)
		statement.DDL.Options["if_not_exists"] = "true"
		statement.Metadata.TargetTable = nil
		statement.Metadata.TargetTables = nil
		if gaps := t16Gaps(t, ruleOut, statement); len(gaps) != 1 {
			t.Fatalf("IF NOT EXISTS must keep the unknown-state gap, got %+v", gaps)
		}
	})
	t.Run("absent_clean", func(t *testing.T) {
		statement := t16Statement(spec.DialectMySQL, spec.DDLOperationCreateTable, targets, t16AbsentTable("golden", "t"))
		if findings := t16MustEval(t, ruleOut, statement); len(findings) != 0 {
			t.Fatalf("create on absent must stay clean, got %+v", findings)
		}
	})
}

func TestT16LikeSourceExistence(t *testing.T) {
	t.Parallel()
	ruleOut, err := newLikeSourceExistsRequireRule(ruleIDTableLikeSourceExistsRequire, rule.LevelBlocker, t16Policy())
	if err != nil {
		t.Fatalf("construct like rule: %v", err)
	}
	targets := []spec.Table{{Schema: "golden", Name: "dst"}}
	build := func(dialect spec.Dialect, source *spec.TableSnapshot) spec.Statement {
		statement := t16Statement(dialect, spec.DDLOperationCreateTable, targets, t16AbsentTable("golden", "dst"))
		statement.DDL.ReferTable = &spec.Table{Schema: "golden", Name: "s"}
		statement.Metadata.SourceTable = source
		return statement
	}
	t.Run("absent_source_blocks", func(t *testing.T) {
		findings := t16MustEval(t, ruleOut, build(spec.DialectMySQL, t16AbsentTable("golden", "s")))
		if len(findings) != 1 || findings[0].Metadata["table"] != "s" {
			t.Fatalf("absent LIKE source must block naming the source, got %+v", findings)
		}
	})
	t.Run("present_source_clean", func(t *testing.T) {
		if findings := t16MustEval(t, ruleOut, build(spec.DialectMySQL, t16PresentTable("golden", "s"))); len(findings) != 0 {
			t.Fatalf("present LIKE source must stay clean, got %+v", findings)
		}
	})
	t.Run("unknown_source_gaps", func(t *testing.T) {
		statement := build(spec.DialectMySQL, nil)
		if findings := t16MustEval(t, ruleOut, statement); len(findings) != 0 {
			t.Fatalf("unknown LIKE source must not fabricate a finding, got %+v", findings)
		}
		if gaps := t16Gaps(t, ruleOut, statement); len(gaps) != 1 {
			t.Fatalf("unknown LIKE source must emit one gap, got %+v", gaps)
		}
	})
	t.Run("tidb_temp_source_clean_for_existence", func(t *testing.T) {
		// The existence premise is answered — proven temp source exists —
		// so this rule stays silent; the dedicated TiDB temp rule owns 8006.
		statement := build(spec.DialectTiDB, t16TempTable("golden", "s"))
		if findings := t16MustEval(t, ruleOut, statement); len(findings) != 0 {
			t.Fatalf("proven temp source must not produce an existence finding, got %+v", findings)
		}
		if gaps := t16Gaps(t, ruleOut, statement); len(gaps) != 0 {
			t.Fatalf("proven temp source must not gap the existence rule, got %+v", gaps)
		}
	})
}

func TestT16CreateAsSourceExistence(t *testing.T) {
	t.Parallel()
	ruleOut, err := newCreateAsSourceExistsRequireRule(ruleIDTableCreateAsSourceExistsRequire, rule.LevelBlocker, t16Policy())
	if err != nil {
		t.Fatalf("construct ctas rule: %v", err)
	}
	targets := []spec.Table{{Schema: "golden", Name: "dst"}}
	build := func(dialect spec.Dialect, sources ...*spec.TableSnapshot) spec.Statement {
		statement := t16Statement(dialect, spec.DDLOperationCreateTable, targets, t16AbsentTable("golden", "dst"))
		statement.DDL.HasSelect = true
		statement.DDL.SelectSources = []spec.Table{{Schema: "golden", Name: "sa"}, {Schema: "golden", Name: "sb"}}
		statement.Metadata.SourceTables = sources
		return statement
	}
	t.Run("tidb_boundary_not_evaluated", func(t *testing.T) {
		statement := build(spec.DialectTiDB, t16AbsentTable("golden", "sa"), t16PresentTable("golden", "sb"))
		if ruleOut.AppliesTo(statement) {
			t.Fatal("TiDB CTAS is a vendor boundary — the rule must not apply")
		}
	})
	t.Run("absent_member_blocks", func(t *testing.T) {
		findings := t16MustEval(t, ruleOut, build(spec.DialectMySQL, t16PresentTable("golden", "sa"), t16AbsentTable("golden", "sb")))
		if len(findings) != 1 || findings[0].Metadata["table"] != "sb" {
			t.Fatalf("absent CTAS member must block naming it, got %+v", findings)
		}
	})
	t.Run("unknown_member_gaps", func(t *testing.T) {
		statement := build(spec.DialectMySQL, t16PresentTable("golden", "sa"), nil)
		if findings := t16MustEval(t, ruleOut, statement); len(findings) != 0 {
			t.Fatalf("unknown CTAS member must not fabricate a finding, got %+v", findings)
		}
		if gaps := t16Gaps(t, ruleOut, statement); len(gaps) != 1 {
			t.Fatalf("unknown CTAS member must emit one gap, got %+v", gaps)
		}
	})
	t.Run("unvouched_members_gap", func(t *testing.T) {
		statement := build(spec.DialectMySQL, nil, nil)
		if gaps := t16Gaps(t, ruleOut, statement); len(gaps) != 2 {
			t.Fatalf("unvouched CTAS members must gap per member, got %+v", gaps)
		}
	})
}

func TestT16RenameExistence(t *testing.T) {
	t.Parallel()
	srcRule, err := newRenameSourceExistsRequireRule(ruleIDTableRenameSourceExistsRequire, rule.LevelBlocker, t16Policy())
	if err != nil {
		t.Fatalf("construct rename source rule: %v", err)
	}
	dstRule, err := newRenameDestExistsForbidRule(ruleIDTableRenameDestExistsForbid, rule.LevelBlocker, t16Policy())
	if err != nil {
		t.Fatalf("construct rename dest rule: %v", err)
	}
	pairs := []spec.Table{{Schema: "golden", Name: "t"}, {Schema: "golden", Name: "u"}}
	build := func(dialect spec.Dialect, src, dst *spec.TableSnapshot) spec.Statement {
		return t16Statement(dialect, spec.DDLOperationRenameTable, pairs, src, dst)
	}
	t.Run("absent_source_blocks", func(t *testing.T) {
		findings := t16MustEval(t, srcRule, build(spec.DialectMySQL, t16AbsentTable("golden", "t"), t16AbsentTable("golden", "u")))
		if len(findings) != 1 || findings[0].Metadata["table"] != "t" {
			t.Fatalf("absent rename source must block naming it, got %+v", findings)
		}
	})
	t.Run("present_dest_blocks", func(t *testing.T) {
		findings := t16MustEval(t, dstRule, build(spec.DialectMySQL, t16PresentTable("golden", "t"), t16PresentTable("golden", "u")))
		if len(findings) != 1 || findings[0].Metadata["table"] != "u" {
			t.Fatalf("present rename dest must block naming it, got %+v", findings)
		}
	})
	t.Run("absent_dest_clean", func(t *testing.T) {
		if findings := t16MustEval(t, dstRule, build(spec.DialectMySQL, t16PresentTable("golden", "t"), t16AbsentTable("golden", "u"))); len(findings) != 0 {
			t.Fatalf("absent rename dest must stay clean, got %+v", findings)
		}
	})
	t.Run("unknown_endpoints_gap", func(t *testing.T) {
		statement := build(spec.DialectMySQL, nil, nil)
		if findings := t16MustEval(t, srcRule, statement); len(findings) != 0 {
			t.Fatalf("unknown rename endpoints must not fabricate findings, got %+v", findings)
		}
		if gaps := t16Gaps(t, srcRule, statement); len(gaps) != 1 {
			t.Fatalf("unknown rename source must emit one gap, got %+v", gaps)
		}
		if gaps := t16Gaps(t, dstRule, statement); len(gaps) != 1 {
			t.Fatalf("unknown rename dest must emit one gap, got %+v", gaps)
		}
	})
	t.Run("swap_dest_occupied_first", func(t *testing.T) {
		// RENAME TABLE t→u, u→v evaluates pairs left-to-right: at pair1's
		// turn the destination u is still occupied (u is only vacated by
		// pair2), so a present u is a proven 1050 collision.
		statement := t16Statement(spec.DialectMySQL, spec.DDLOperationRenameTable,
			[]spec.Table{{Schema: "golden", Name: "t"}, {Schema: "golden", Name: "u"}, {Schema: "golden", Name: "u"}, {Schema: "golden", Name: "v"}},
			t16PresentTable("golden", "t"), t16PresentTable("golden", "u"), t16PresentTable("golden", "u"), t16AbsentTable("golden", "v"))
		findings := t16MustEval(t, dstRule, statement)
		if len(findings) != 1 || findings[0].Metadata["table"] != "u" {
			t.Fatalf("dest occupied at pair time must block naming it, got %+v", findings)
		}
	})
	t.Run("swap_dest_vacated_first", func(t *testing.T) {
		// RENAME TABLE u→v, t→u is the legal rotation: pair1 vacates u
		// before pair2 claims it as destination, so no collision fires.
		statement := t16Statement(spec.DialectMySQL, spec.DDLOperationRenameTable,
			[]spec.Table{{Schema: "golden", Name: "u"}, {Schema: "golden", Name: "v"}, {Schema: "golden", Name: "t"}, {Schema: "golden", Name: "u"}},
			t16PresentTable("golden", "u"), t16AbsentTable("golden", "v"), t16PresentTable("golden", "t"), t16PresentTable("golden", "u"))
		if findings := t16MustEval(t, dstRule, statement); len(findings) != 0 {
			t.Fatalf("dest vacated by an earlier pair must stay clean, got %+v", findings)
		}
	})
	t.Run("rename_form_required", func(t *testing.T) {
		statement := t16Statement(spec.DialectMySQL, spec.DDLOperationAlterTable, pairs[:1], t16PresentTable("golden", "t"))
		if srcRule.AppliesTo(statement) || dstRule.AppliesTo(statement) {
			t.Fatal("non-rename alter must not apply to rename rules")
		}
	})
}

func TestT16TruncateInboundFK(t *testing.T) {
	t.Parallel()
	ruleOut, err := newTruncateInboundFKRule(ruleIDTableTruncateInboundFK, rule.LevelBlocker, t16Policy())
	if err != nil {
		t.Fatalf("construct truncate-fk rule: %v", err)
	}
	targets := []spec.Table{{Schema: "golden", Name: "t"}}
	build := func(snapshot *spec.TableSnapshot) spec.Statement {
		return t16Statement(spec.DialectMySQL, spec.DDLOperationTruncateTable, targets, snapshot)
	}
	t.Run("inbound_fk_blocks", func(t *testing.T) {
		snapshot := t16PresentTable("golden", "t")
		snapshot.ReferencedBy = []spec.Table{{Schema: "aux", Name: "child"}}
		findings := t16MustEval(t, ruleOut, build(snapshot))
		if len(findings) != 1 {
			t.Fatalf("proven inbound FK must block the truncate, got %+v", findings)
		}
	})
	t.Run("empty_referenced_clean", func(t *testing.T) {
		if findings := t16MustEval(t, ruleOut, build(t16PresentTable("golden", "t"))); len(findings) != 0 {
			t.Fatalf("empty inbound FK list must stay clean, got %+v", findings)
		}
	})
	t.Run("unknown_inbound_gaps", func(t *testing.T) {
		snapshot := t16PresentTable("golden", "t")
		snapshot.ReferencedByUnknown = true
		statement := build(snapshot)
		if findings := t16MustEval(t, ruleOut, statement); len(findings) != 0 {
			t.Fatalf("unknown inbound FK must not fabricate a finding, got %+v", findings)
		}
		gaps := t16Gaps(t, ruleOut, statement)
		if len(gaps) != 1 || gaps[0].ReasonCode != gapReasonUnknownTableState {
			t.Fatalf("unknown inbound FK must emit one gap, got %+v", gaps)
		}
	})
	t.Run("absent_target_clean", func(t *testing.T) {
		// Existence is owned by truncate.exists.require — this rule only
		// answers the inbound-FK premise on a proven-present target.
		statement := build(t16AbsentTable("golden", "t"))
		if findings := t16MustEval(t, ruleOut, statement); len(findings) != 0 {
			t.Fatalf("absent truncate target must stay clean here, got %+v", findings)
		}
	})
}

func TestT16TiDBLocalTemporaryRules(t *testing.T) {
	t.Parallel()
	type ruleCtor func(string, string, string, rule.Level, policy.RulePolicy) (rule.StatementRule, error)
	mkTemp := func(ruleID, form, label string) rule.StatementRule {
		r, err := newTiDBLocalTemporaryRule(ruleID, form, label, rule.LevelBlocker, t16Policy())
		if err != nil {
			t.Fatalf("construct %s: %v", ruleID, err)
		}
		return r
	}
	var _ ruleCtor = newTiDBLocalTemporaryRule
	alterRule := mkTemp(ruleIDTiDBLocalTemporaryAlter, "alter", "alter table")
	renameRule := mkTemp(ruleIDTiDBLocalTemporaryRename, "rename", "rename table")
	likeRule := mkTemp(ruleIDTiDBLocalTemporaryLike, "like", "create table like")
	targets := []spec.Table{{Schema: "golden", Name: "t"}}

	t.Run("alter_on_local_temp_blocks", func(t *testing.T) {
		statement := t16Statement(spec.DialectTiDB, spec.DDLOperationAlterTable, targets, t16TempTable("golden", "t"))
		if !alterRule.AppliesTo(statement) {
			t.Fatal("alter rule must apply to a plain TiDB alter")
		}
		if findings := t16MustEval(t, alterRule, statement); len(findings) != 1 {
			t.Fatalf("local temp alter must block, got %+v", findings)
		}
	})
	t.Run("alter_on_persistent_clean", func(t *testing.T) {
		statement := t16Statement(spec.DialectTiDB, spec.DDLOperationAlterTable, targets, t16PresentTable("golden", "t"))
		if findings := t16MustEval(t, alterRule, statement); len(findings) != 0 {
			t.Fatalf("persistent alter must stay clean, got %+v", findings)
		}
	})
	t.Run("rename_on_temp_endpoint_blocks", func(t *testing.T) {
		statement := t16Statement(spec.DialectTiDB, spec.DDLOperationRenameTable,
			[]spec.Table{{Schema: "golden", Name: "t"}, {Schema: "golden", Name: "u"}},
			t16TempTable("golden", "t"), t16AbsentTable("golden", "u"))
		if !renameRule.AppliesTo(statement) {
			t.Fatal("rename rule must apply to a rename statement")
		}
		if findings := t16MustEval(t, renameRule, statement); len(findings) != 1 {
			t.Fatalf("rename on a local temp endpoint must block, got %+v", findings)
		}
	})
	t.Run("like_from_temp_source_blocks", func(t *testing.T) {
		statement := t16Statement(spec.DialectTiDB, spec.DDLOperationCreateTable, targets, t16AbsentTable("golden", "t"))
		statement.DDL.ReferTable = &spec.Table{Schema: "golden", Name: "s"}
		statement.Metadata.SourceTable = t16TempTable("golden", "s")
		if !likeRule.AppliesTo(statement) {
			t.Fatal("like rule must apply to a LIKE create")
		}
		if findings := t16MustEval(t, likeRule, statement); len(findings) != 1 {
			t.Fatalf("LIKE from a local temp source must block, got %+v", findings)
		}
	})
	t.Run("mysql_never_applies", func(t *testing.T) {
		for name, r := range map[string]rule.StatementRule{"alter": alterRule, "rename": renameRule, "like": likeRule} {
			statement := t16Statement(spec.DialectMySQL, spec.DDLOperationAlterTable, targets, t16TempTable("golden", "t"))
			statement.DDL.ReferTable = &spec.Table{Schema: "golden", Name: "s"}
			if r.AppliesTo(statement) {
				t.Fatalf("MySQL must never apply to the %s rule", name)
			}
		}
	})
	t.Run("rename_alter_excluded_from_alter_form", func(t *testing.T) {
		statement := t16Statement(spec.DialectTiDB, spec.DDLOperationAlterTable, targets, t16TempTable("golden", "t"))
		statement.DDL.Alter = []spec.Alter{{Action: "rename_table"}}
		if alterRule.AppliesTo(statement) {
			t.Fatal("rename alter must be owned by the rename form, not alter")
		}
		if !renameRule.AppliesTo(statement) {
			t.Fatal("rename alter must apply to the rename form")
		}
	})
}

func TestT16LikeSourceNoMetadataGaps(t *testing.T) {
	t.Parallel()
	ruleOut, err := newLikeSourceExistsRequireRule(ruleIDTableLikeSourceExistsRequire, rule.LevelBlocker, t16Policy())
	if err != nil {
		t.Fatalf("construct like rule: %v", err)
	}
	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateTable,
			Table:      &spec.Table{Name: "dst"},
			ReferTable: &spec.Table{Schema: "golden", Name: "s"},
		},
	}
	if gaps := t16Gaps(t, ruleOut, statement); len(gaps) != 1 {
		t.Fatalf("LIKE source without metadata must emit one gap, got %+v", gaps)
	}
}
