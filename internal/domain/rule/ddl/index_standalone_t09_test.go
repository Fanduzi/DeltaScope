// Package ddl verifies the T09 standalone-CREATE-INDEX unification contract.
// input: real MySQL-family spec shapes — standalone CREATE INDEX carries its
// index under the create_index alter action, never in DDL.Indexes — plus
// target-table snapshots for lifecycle candidates
// output: the same ddl.index.* findings a create-table entry produces, plus
// lifecycle duplicate/redundant verdicts against the known existing index set
// and honest target_table.indexes gaps when that set is unknowable
// pos: domain DDL rule test coverage for issue #88 T09 first-path unification
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// mysqlCreateIndexStatement builds the spec shape the MySQL/TiDB extractor
// actually produces for a standalone CREATE [UNIQUE] INDEX: the index lives
// under Alter[].Index.Definition with action "create_index" and DDL.Indexes
// stays empty. Alter.Name is intentionally unset — the real extractor leaves
// it empty for this action.
func mysqlCreateIndexStatement(indexes ...spec.Index) spec.Statement {
	alters := make([]spec.Alter, 0, len(indexes))
	for i := range indexes {
		def := indexes[i]
		alters = append(alters, spec.Alter{
			Action: "create_index",
			Index:  &spec.AlterIndex{Definition: &def},
		})
	}
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateIndex,
			Table:     &spec.Table{Name: "users"},
			Alter:     alters,
		},
	}
}

// mysqlDropIndexStatement builds the standalone DROP INDEX shape: the name is
// carried under Index.OldName, alter.Name is empty.
func mysqlDropIndexStatement(name string) spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationDropIndex,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{{
				Action: "drop_index",
				Index:  &spec.AlterIndex{OldName: name},
			}},
		},
	}
}

// alterAddIndexStatement builds an ALTER TABLE … ADD INDEX shape.
func alterAddIndexStatement(alters ...spec.Alter) spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter:     alters,
		},
	}
}

func alterAddIndex(name string, kind spec.IndexKind, columns ...string) spec.Alter {
	return spec.Alter{
		Action: "add_index",
		Name:   name,
		Index: &spec.AlterIndex{Definition: &spec.Index{
			Name: name, Kind: kind, Columns: columns,
		}},
	}
}

func alterAddUniqueConstraint(name string, columns ...string) spec.Alter {
	return spec.Alter{
		Action: "add_constraint",
		Name:   name,
		Index: &spec.AlterIndex{Definition: &spec.Index{
			Name: name, Kind: spec.IndexKindUnique, Columns: columns,
		}},
		Constraint: &spec.Constraint{Type: "unique", Name: name, Columns: columns},
	}
}

// withTargetSnapshot attaches a target-table pre-state.
func withTargetSnapshot(st spec.Statement, snap *spec.TableSnapshot) spec.Statement {
	st.Metadata = &spec.Metadata{TargetTable: snap}
	return st
}

func snapshotWithIndexes(indexes ...spec.Index) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists:  true,
		Table:   &spec.Table{Name: "users"},
		Columns: []spec.Column{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		Indexes: indexes,
	}
}

func gapReasons(gaps []rule.EvidenceGap) []string {
	out := make([]string, 0, len(gaps))
	for _, g := range gaps {
		out = append(out, g.ReasonCode)
	}
	return out
}

// --- columns.max_count ----------------------------------------------------

func TestT09CreateIndexColumnsMaxCountApplies(t *testing.T) {
	t.Parallel()
	statement := mysqlCreateIndexStatement(spec.Index{
		Name: "idx_wide", Kind: spec.IndexKindSecondary,
		Columns: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
	})
	statementRule, err := newIndexColumnsMaxCountRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning,
		Params: map[string]any{"limit": 8},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	if !statementRule.AppliesTo(statement) {
		t.Fatal("ddl.index.columns.max_count must apply to standalone CREATE INDEX")
	}
	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding bound to idx_wide, got %d", len(findings))
	}
	if findings[0].Metadata["index"] != "idx_wide" {
		t.Fatalf("finding must name the statement's own index, got %+v", findings[0].Metadata)
	}
}

// --- naming ---------------------------------------------------------------

func TestT09CreateIndexNamingRulesApply(t *testing.T) {
	t.Parallel()
	secondaryRule, err := newIndexPrefixRequiredRule(ruleIDIndexSecondaryPrefixRequire,
		spec.IndexKindSecondary, "idx_", rule.LevelBlocker, policy.RulePolicy{
			Enabled: true, Level: rule.LevelBlocker,
			Params: map[string]any{"required": true, "prefix": "idx_"},
		})
	if err != nil {
		t.Fatalf("new secondary rule: %v", err)
	}
	bad := mysqlCreateIndexStatement(spec.Index{Name: "bad", Kind: spec.IndexKindSecondary, Columns: []string{"a"}})
	findings, err := secondaryRule.Evaluate(context.Background(), bad)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("bad standalone secondary name must produce 1 finding, got %d", len(findings))
	}

	uniqueRule, err := newIndexPrefixRequiredRule(ruleIDIndexUniquePrefixRequire,
		spec.IndexKindUnique, "uq_", rule.LevelBlocker, policy.RulePolicy{
			Enabled: true, Level: rule.LevelBlocker,
			Params: map[string]any{"required": true, "prefix": "uq_"},
		})
	if err != nil {
		t.Fatalf("new unique rule: %v", err)
	}
	badUnique := mysqlCreateIndexStatement(spec.Index{Name: "bad", Kind: spec.IndexKindUnique, Columns: []string{"a"}})
	findings, err = uniqueRule.Evaluate(context.Background(), badUnique)
	if err != nil {
		t.Fatalf("evaluate unique: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("bad standalone unique name must produce 1 finding, got %d", len(findings))
	}
}

func TestT09CreateIndexNamePatternKeywordApply(t *testing.T) {
	t.Parallel()
	statement := mysqlCreateIndexStatement(spec.Index{Name: "wide", Kind: spec.IndexKindSecondary, Columns: []string{"a"}})
	patternRule, err := newIdentifierPatternRule(ruleIDIndexNamePatternRequire, "index",
		rule.LevelBlocker, policy.RulePolicy{
			Enabled: true, Level: rule.LevelBlocker,
			Params: map[string]any{"required": true, "pattern": "^idx_"},
		}, selectIndexNames, appliesToDDLWithIndexes)
	if err != nil {
		t.Fatalf("new pattern rule: %v", err)
	}
	if !patternRule.AppliesTo(statement) {
		t.Fatal("ddl.index.name.pattern.require must see standalone create_index names")
	}
	findings, err := patternRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

// --- duplicate / redundancy against the known existing set ----------------

func TestT09CreateIndexDuplicateAgainstExisting(t *testing.T) {
	t.Parallel()
	statement := withTargetSnapshot(
		mysqlCreateIndexStatement(spec.Index{Name: "i_new", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b"}}),
		snapshotWithIndexes(spec.Index{Name: "i_ab", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b"}}),
	)
	statementRule, err := newDuplicateIndexForbiddenRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["index"] != "i_new" || findings[0].Metadata["duplicate"] != "i_ab" {
		t.Fatalf("expected duplicate finding i_new vs existing i_ab, got %+v", findings)
	}
}

func TestT09CreateIndexDuplicateSuppressesPreexistingPair(t *testing.T) {
	t.Parallel()
	// Two identical pre-existing indexes are not this statement's doing —
	// adding an unrelated index must not borrow their finding.
	statement := withTargetSnapshot(
		mysqlCreateIndexStatement(spec.Index{Name: "i_new", Kind: spec.IndexKindSecondary, Columns: []string{"c"}}),
		snapshotWithIndexes(
			spec.Index{Name: "i1", Kind: spec.IndexKindSecondary, Columns: []string{"a"}},
			spec.Index{Name: "i2", Kind: spec.IndexKindSecondary, Columns: []string{"a"}},
		),
	)
	statementRule, err := newDuplicateIndexForbiddenRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("pre-existing duplicate pair must not be attributed to this statement, got %+v", findings)
	}
}

func TestT09CreateIndexRedundantAgainstExisting(t *testing.T) {
	t.Parallel()
	statement := withTargetSnapshot(
		mysqlCreateIndexStatement(spec.Index{Name: "i_a", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
		snapshotWithIndexes(spec.Index{Name: "i_abc", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b", "c"}}),
	)
	statementRule, err := newRedundantLeftPrefixIndexRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["index"] != "i_a" {
		t.Fatalf("expected left-prefix redundant finding naming i_a, got %+v", findings)
	}
}

func TestT09CreateIndexUniqueOverlapAgainstExisting(t *testing.T) {
	t.Parallel()
	statement := withTargetSnapshot(
		mysqlCreateIndexStatement(spec.Index{Name: "s_cd", Kind: spec.IndexKindSecondary, Columns: []string{"b", "c"}}),
		snapshotWithIndexes(spec.Index{Name: "uq_bc", Kind: spec.IndexKindUnique, Columns: []string{"b", "c"}}),
	)
	statementRule, err := newRedundantUniqueOverlapIndexRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["index"] != "s_cd" {
		t.Fatalf("expected unique-overlap finding naming s_cd, got %+v", findings)
	}
}

// --- unknowable existing set reports a gap, never silence ------------------

type gapAwareRule interface {
	rule.StatementRule
	EvidenceGaps(spec.Statement) []rule.EvidenceGap
}

func TestT09CreateIndexUnknownExistingSetReportsGap(t *testing.T) {
	t.Parallel()
	snap := snapshotWithIndexes()
	snap.Indexes = nil
	snap.IndexesUnknown = true
	statement := withTargetSnapshot(
		mysqlCreateIndexStatement(spec.Index{Name: "i_new", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
		snap,
	)
	for _, build := range []struct {
		name  string
		build func() (rule.StatementRule, error)
	}{
		{"duplicate", func() (rule.StatementRule, error) {
			return newDuplicateIndexForbiddenRule(policy.RulePolicy{Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true}})
		}},
		{"left_prefix", func() (rule.StatementRule, error) {
			return newRedundantLeftPrefixIndexRule(policy.RulePolicy{Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true}})
		}},
		{"unique_overlap", func() (rule.StatementRule, error) {
			return newRedundantUniqueOverlapIndexRule(policy.RulePolicy{Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true}})
		}},
	} {
		t.Run(build.name, func(t *testing.T) {
			t.Parallel()
			r, err := build.build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			gr, ok := r.(gapAwareRule)
			if !ok {
				t.Fatalf("rule %s must implement EvidenceGaps", r.ID())
			}
			gaps := gr.EvidenceGaps(statement)
			if len(gaps) != 1 {
				t.Fatalf("expected exactly 1 gap for unknowable index set, got %+v", gaps)
			}
			found := false
			for _, fact := range gaps[0].RequiredFacts {
				if fact == "target_table.indexes" {
					found = true
				}
			}
			if !found {
				t.Fatalf("gap must require target_table.indexes, got %+v", gaps[0])
			}
			findings, err := r.Evaluate(context.Background(), statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("unknown existing set must not fabricate findings, got %+v", findings)
			}
		})
	}
}

// Offline (no snapshot at all) behaves identically: gap, no fabricated finding.
func TestT09CreateIndexNoSnapshotReportsGap(t *testing.T) {
	t.Parallel()
	statement := mysqlCreateIndexStatement(spec.Index{Name: "i_new", Kind: spec.IndexKindSecondary, Columns: []string{"a"}})
	r, err := newDuplicateIndexForbiddenRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	gr, ok := r.(gapAwareRule)
	if !ok {
		t.Fatal("duplicate rule must implement EvidenceGaps")
	}
	if gaps := gr.EvidenceGaps(statement); len(gaps) != 1 {
		t.Fatalf("offline create_index must report the missing existing-set fact, got %+v", gaps)
	}
}

// --- existence rules on the standalone entries ----------------------------

func TestT09AddIndexExistsForbidCoversCreateIndex(t *testing.T) {
	t.Parallel()
	statement := withTargetSnapshot(
		mysqlCreateIndexStatement(spec.Index{Name: "i_ab", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
		snapshotWithIndexes(spec.Index{Name: "i_ab", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
	)
	r, err := newAlterObjectExistenceRule(ruleIDAlterAddIndexExistsForbid,
		[]string{"add_constraint", "add_index", "create_index"}, "index", true,
		rule.LevelBlocker, policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker},
		alterIndexSubjectName, snapshotHasIndex)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("standalone CREATE INDEX name collision must report 1 blocker, got %+v", findings)
	}
}

func TestT09DropIndexExistsRequireCoversStandalone(t *testing.T) {
	t.Parallel()
	statement := withTargetSnapshot(
		mysqlDropIndexStatement("gone"),
		snapshotWithIndexes(spec.Index{Name: "i_ab", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
	)
	r, err := newAlterObjectExistenceRule(ruleIDAlterDropIndexExistsRequire,
		[]string{"drop_index"}, "index", false,
		rule.LevelBlocker, policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker},
		alterIndexTargetName, snapshotHasIndex)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("standalone DROP INDEX on a missing name must report 1 blocker, got %+v", findings)
	}
}

// --- constraint-vs-index naming separation --------------------------------

func TestT09ConstraintSyntaxDoesNotHitIndexNaming(t *testing.T) {
	t.Parallel()
	statement := alterAddIndexStatement(alterAddUniqueConstraint("badname", "a", "b"))
	uniqueNaming, err := newAlterAddedIndexPrefixRule(ruleIDAlterAddIndexUniquePrefixRequire,
		spec.IndexKindUnique, "uq_", rule.LevelBlocker, policy.RulePolicy{
			Enabled: true, Level: rule.LevelBlocker,
			Params: map[string]any{"required": true, "prefix": "uq_"},
		})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	if uniqueNaming.AppliesTo(statement) {
		findings, err := uniqueNaming.Evaluate(context.Background(), statement)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("ADD CONSTRAINT name must not be judged by index naming policy, got %+v", findings)
		}
		return
	}
	// AppliesTo=false is also an acceptable wiring (the constraint name is not
	// an index-name subject at all).
}

func TestT09ConstraintSyntaxHitsConstraintNaming(t *testing.T) {
	t.Parallel()
	statement := alterAddIndexStatement(alterAddUniqueConstraint("badname", "a", "b"))
	// Construct the same rule shape register.go wires: a namingRule on the
	// unique-key constraint family with the alter-constraint appliesTo.
	constraintNaming, err := newNamingPrefixRule(ruleIDConstraintUniqueKeyNamePrefixRequire,
		"unique key constraint", rule.LevelBlocker, policy.RulePolicy{
			Enabled: true, Level: rule.LevelBlocker,
			Params: map[string]any{"required": true, "prefix": "uc_"},
		}, selectUniqueConstraintNames, appliesToCreateTableOrAlterConstraintNames)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := constraintNaming.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("ADD CONSTRAINT unique name must reach the constraint naming family, got %+v", findings)
	}
}

// --- ALTER duplicate against existing (lifecycle widening) -----------------

func TestT09AlterAddIndexDuplicateAgainstExisting(t *testing.T) {
	t.Parallel()
	statement := withTargetSnapshot(
		alterAddIndexStatement(alterAddIndex("i_new", spec.IndexKindSecondary, "a", "b")),
		snapshotWithIndexes(spec.Index{Name: "i_ab", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b"}}),
	)
	r, err := newAlterAddedDuplicateIndexForbiddenRule(rule.LevelWarning, policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].RuleID != ruleIDAlterAddIndexDuplicateForbid {
		t.Fatalf("ALTER ADD INDEX duplicating an existing index must fire %s naming i_new, got %+v",
			ruleIDAlterAddIndexDuplicateForbid, findings)
	}
}

func TestT09AlterUniqueOverlapWithinStatement(t *testing.T) {
	t.Parallel()
	// Within-statement: the constraint-derived unique index and the plain
	// index share the ordered columns — fires with no metadata at all.
	statement := alterAddIndexStatement(
		alterAddUniqueConstraint("uq_ab", "a", "b"),
		alterAddIndex("s_ab", spec.IndexKindSecondary, "a", "b"),
	)
	r, err := newAlterAddedRedundantUniqueOverlapRule(rule.LevelWarning, policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].RuleID != ruleIDAlterAddIndexRedundantUniqueOverlapForbid ||
		findings[0].Metadata["index"] != "s_ab" {
		t.Fatalf("within-statement unique overlap must fire naming s_ab, got %+v", findings)
	}
}

func TestT09AlterUniqueOverlapAgainstExisting(t *testing.T) {
	t.Parallel()
	statement := withTargetSnapshot(
		alterAddIndexStatement(alterAddIndex("s_cd", spec.IndexKindSecondary, "b", "c")),
		snapshotWithIndexes(spec.Index{Name: "uq_bc", Kind: spec.IndexKindUnique, Columns: []string{"b", "c"}}),
	)
	r, err := newAlterAddedRedundantUniqueOverlapRule(rule.LevelWarning, policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].RuleID != ruleIDAlterAddIndexRedundantUniqueOverlapForbid {
		t.Fatalf("deferred ALTER unique-overlap rule must fire against existing set, got %+v", findings)
	}
}

func TestT09AlterUniqueOverlapSuppressesPreexistingPairs(t *testing.T) {
	t.Parallel()
	// A pre-existing secondary shadowing a pre-existing unique is not this
	// statement's doing.
	statement := withTargetSnapshot(
		alterAddIndexStatement(alterAddIndex("i_new", spec.IndexKindSecondary, "a")),
		snapshotWithIndexes(
			spec.Index{Name: "uq_ab", Kind: spec.IndexKindUnique, Columns: []string{"a", "b"}},
			spec.Index{Name: "s_ab", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b"}},
		),
	)
	r, err := newAlterAddedRedundantUniqueOverlapRule(rule.LevelWarning, policy.RulePolicy{
		Enabled: true, Level: rule.LevelWarning, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("pre-existing overlap pair must not report on an unrelated add, got %+v", findings)
	}
}

// --- lifecycle forbid on standalone entries --------------------------------

// A standalone DROP INDEX carries the action under the drop_index operation
// with DDL.Table populated by the extractor — the forbid rule must still see
// it, resolve the dropped name from Index.OldName, and bind the table.
func TestT09DropIndexForbidCoversStandalone(t *testing.T) {
	t.Parallel()
	r, err := newForbiddenAlterActionRule(ruleIDAlterDropIndexForbid, "drop_index", "drop index", rule.LevelWarning, policy.RulePolicy{
		Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	statement := mysqlDropIndexStatement("idx_gone")
	findings, err := r.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("standalone DROP INDEX must fire drop_index.forbid once, got %+v", findings)
	}
	f := findings[0]
	if f.Level != rule.LevelBlocker || f.Metadata["name"] != "idx_gone" ||
		f.Metadata["table"] != "users" || f.Metadata["action"] != "drop_index" {
		t.Fatalf("finding bound wrong subject: %+v", f.Metadata)
	}
}
