// Package audit verifies the T05-A5-R1 partial-snapshot and same-identity corrections.
// input: real parser statements and AuditSQL requests against partial provider snapshots
// output: definite member-absence findings, conservative invalidation gaps, and zero same-identity destination gaps
// pos: T05-A5-R1 shared regression for known-empty collections, declaration-level gates, and the destination-conflict exception
// note: if this file changes, update this header and module README.md.
package audit

import (
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t05A5DropIndexExists = "ddl.alter.drop_index.exists.require"
	t05A5DropPKExists    = "ddl.alter.drop_primary_key.exists.require"
	t05A5ChangeExists    = "ddl.alter.change_column.exists.require"
	t05A5ChangeCompat    = "ddl.alter.change_column.compatibility.require"
	t05A5RenameExists    = "ddl.alter.rename_column.exists.require"
)

// t05A5R1PartialTable is a present table whose column set is withheld.
// Nil index and constraint slices with Unknown=false are loaded empty sets.
func t05A5R1PartialTable(nilCollections bool) *spec.TableSnapshot {
	shape := &spec.TableSnapshot{
		Exists: true,
		Schema: "golden",
		Table:  &spec.Table{Schema: "golden", Name: "t"},
	}
	if !nilCollections {
		shape.Indexes = []spec.Index{}
		shape.Constraints = []spec.Constraint{}
	}
	return shape
}

func t05A5R1Banner(dialect spec.Dialect) string {
	if dialect == spec.DialectTiDB {
		return "8.0.11-TiDB-v8.5.0"
	}
	return "8.4.10"
}

func t05A5R1Providers(shape *spec.TableSnapshot, dialect spec.Dialect, rename bool) (MetadataProvider, *t05PresentProvider) {
	inner := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": shape}}
	if rename {
		return &t05BannerPresent{inner: inner, banner: t05A5R1Banner(dialect)}, inner
	}
	return inner, inner
}

// t05A5R1Audit runs the shared AuditSQL entry and the same parser path's
// ordered pre-state. Findings come from AuditSQL. The snapshot the next
// statement reads comes from enrichment, because the public result does not
// carry the table snapshot.
func t05A5R1Audit(t *testing.T, sql string, dialect spec.Dialect, shape *spec.TableSnapshot, rename bool, policy string) (report.Result, []spec.Statement) {
	t.Helper()
	before := cloneTableSnapshot(shape)
	provider, inner := t05A5R1Providers(shape, dialect, rename)
	result := t05A5AuditWithPolicy(t, sql, dialect, provider, "", policy)
	if !reflect.DeepEqual(shape, before) {
		t.Fatalf("provider snapshot changed: got %#v want %#v", shape, before)
	}
	if len(inner.calls) != 1 || inner.calls[0] != "golden.t" {
		t.Fatalf("provider ledger = %#v", inner.calls)
	}
	enrichProvider, _ := t05A5R1Providers(shape, dialect, rename)
	enriched := enrichA3(t, sql, dialect, enrichProvider)
	if !reflect.DeepEqual(shape, before) {
		t.Fatalf("provider snapshot changed during enrich: got %#v want %#v", shape, before)
	}
	return result, enriched
}

func t05A5R1Post(t *testing.T, enriched []spec.Statement) *spec.TableSnapshot {
	t.Helper()
	if len(enriched) != 2 || enriched[1].Metadata == nil || enriched[1].Metadata.TargetTable == nil {
		t.Fatalf("statement 2 has no present pre-state: %#v", enriched)
	}
	return enriched[1].Metadata.TargetTable
}

func t05A5R1Pre(t *testing.T, enriched []spec.Statement) *spec.TableSnapshot {
	t.Helper()
	if len(enriched) == 0 || enriched[0].Metadata == nil || enriched[0].Metadata.TargetTable == nil {
		t.Fatalf("statement 1 has no pre-state: %#v", enriched)
	}
	return enriched[0].Metadata.TargetTable
}

func t05A5R1AssertColumnsWithheld(t *testing.T, shape *spec.TableSnapshot) {
	t.Helper()
	if shape == nil || !shape.Exists || shape.Columns != nil || shape.FindColumn("c") != nil || shape.FindColumn("c2") != nil {
		t.Fatalf("column set = %#v", shape)
	}
}

func t05A5R1AssertKnownEmpty(t *testing.T, shape *spec.TableSnapshot, nilCollections bool) {
	t.Helper()
	t05A5R1AssertColumnsWithheld(t, shape)
	if shape.IndexesUnknown || shape.ConstraintsUnknown || shape.PrimaryKeyUnknown || shape.PrimaryKey != nil {
		t.Fatalf("known-empty flags changed: pkUnknown=%v indexesUnknown=%v constraintsUnknown=%v pk=%#v", shape.PrimaryKeyUnknown, shape.IndexesUnknown, shape.ConstraintsUnknown, shape.PrimaryKey)
	}
	if nilCollections {
		if shape.Indexes != nil || shape.Constraints != nil {
			t.Fatalf("nil collections became indexes=%#v constraints=%#v", shape.Indexes, shape.Constraints)
		}
		return
	}
	if shape.Indexes == nil || len(shape.Indexes) != 0 || shape.Constraints == nil || len(shape.Constraints) != 0 {
		t.Fatalf("empty collections = indexes %#v constraints %#v", shape.Indexes, shape.Constraints)
	}
}

func TestAuditSQLT05A5R1KnownEmptyMembersStayAbsent(t *testing.T) {
	t.Parallel()
	indexSQL := []struct {
		name   string
		sql    string
		rename bool
	}{
		{name: "change", sql: "ALTER TABLE t CHANGE COLUMN c c2 INT;\nALTER TABLE t DROP INDEX ix;", rename: false},
		{name: "rename", sql: "ALTER TABLE t RENAME COLUMN c TO c2;\nALTER TABLE t DROP INDEX ix;", rename: true},
	}
	for _, action := range indexSQL {
		for _, nilCollections := range []bool{true, false} {
			for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
				action, nilCollections, dialect := action, nilCollections, dialect
				t.Run(action.name+"/"+collectionLabel(nilCollections)+"/"+string(dialect), func(t *testing.T) {
					t.Parallel()
					shape := t05A5R1PartialTable(nilCollections)
					result, enriched := t05A5R1Audit(t, action.sql, dialect, shape, action.rename, t05PolicyPath(t, map[string]string{t05A5DropIndexExists: ""}))
					t05A5R1AssertKnownEmpty(t, t05A5R1Pre(t, enriched), nilCollections)
					t05A5R1AssertKnownEmpty(t, t05A5R1Post(t, enriched), nilCollections)
					findings := t05FindingsByRule(result, 1, t05A5DropIndexExists)
					gaps := t05GapsByRule(result, 1, t05A5DropIndexExists)
					if len(findings) != 1 || len(gaps) != 0 || findings[0].Level != rule.LevelBlocker || findings[0].Metadata["exists"] != false || findings[0].Metadata["name"] != "ix" || findings[0].Metadata["action"] != "drop_index" {
						t.Fatalf("drop index findings=%#v gaps=%#v", findings, gaps)
					}
				})
			}
		}
	}

	pkSQL := []struct {
		name   string
		sql    string
		rename bool
	}{
		{name: "change", sql: "ALTER TABLE t CHANGE COLUMN c c2 INT;\nALTER TABLE t DROP PRIMARY KEY;", rename: false},
		{name: "rename", sql: "ALTER TABLE t RENAME COLUMN c TO c2;\nALTER TABLE t DROP PRIMARY KEY;", rename: true},
	}
	for _, action := range pkSQL {
		for _, nilCollections := range []bool{true, false} {
			for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
				action, nilCollections, dialect := action, nilCollections, dialect
				t.Run("pk/"+action.name+"/"+collectionLabel(nilCollections)+"/"+string(dialect), func(t *testing.T) {
					t.Parallel()
					shape := t05A5R1PartialTable(nilCollections)
					result, enriched := t05A5R1Audit(t, action.sql, dialect, shape, action.rename, t05PolicyPath(t, map[string]string{t05A5DropPKExists: ""}))
					t05A5R1AssertKnownEmpty(t, t05A5R1Post(t, enriched), nilCollections)
					findings := t05FindingsByRule(result, 1, t05A5DropPKExists)
					gaps := t05GapsByRule(result, 1, t05A5DropPKExists)
					if len(findings) != 1 || len(gaps) != 0 || findings[0].Level != rule.LevelBlocker || findings[0].Metadata["exists"] != false || findings[0].Metadata["action"] != "drop_primary_key" {
						t.Fatalf("drop primary key findings=%#v gaps=%#v", findings, gaps)
					}
				})
			}
		}
	}

	t.Run("unknown index stays unknown", func(t *testing.T) {
		t.Parallel()
		shape := t05A5R1PartialTable(true)
		shape.IndexesUnknown = true
		result, enriched := t05A5R1Audit(t, indexSQL[0].sql, spec.DialectMySQL, shape, false, t05PolicyPath(t, map[string]string{t05A5DropIndexExists: ""}))
		post := t05A5R1Post(t, enriched)
		t05A5R1AssertColumnsWithheld(t, post)
		if !post.IndexesUnknown {
			t.Fatalf("indexes unknown cleared: %#v", post)
		}
		if len(t05FindingsByRule(result, 1, t05A5DropIndexExists)) != 0 || len(t05GapsByRule(result, 1, t05A5DropIndexExists)) != 1 || t05GapsByRule(result, 1, t05A5DropIndexExists)[0].ReasonCode != "unknown_table_state" {
			t.Fatalf("unknown index consumer = findings %#v gaps %#v", result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
		}
	})

	t.Run("unknown constraint stays unknown", func(t *testing.T) {
		t.Parallel()
		shape := t05A5R1PartialTable(true)
		shape.ConstraintsUnknown = true
		result, enriched := t05A5R1Audit(t, pkSQL[0].sql, spec.DialectMySQL, shape, false, t05PolicyPath(t, map[string]string{t05A5DropPKExists: ""}))
		post := t05A5R1Post(t, enriched)
		if !post.ConstraintsUnknown || len(t05FindingsByRule(result, 1, t05A5DropPKExists)) != 0 || len(t05GapsByRule(result, 1, t05A5DropPKExists)) != 1 {
			t.Fatalf("unknown constraint consumer = post %#v findings %#v gaps %#v", post, result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
		}
	})

	t.Run("affected member is cleared and unrelated member stays", func(t *testing.T) {
		t.Parallel()
		shape := &spec.TableSnapshot{
			Exists: true, Schema: "golden", Table: &spec.Table{Schema: "golden", Name: "t"},
			Indexes: []spec.Index{
				{Name: "idx_c", Kind: spec.IndexKindSecondary, Columns: []string{"c"}},
				{Name: "idx_k", Kind: spec.IndexKindSecondary, Columns: []string{"k"}, PrefixParts: 1},
			},
			PrimaryKey:  &spec.Index{Name: "PRIMARY", Kind: spec.IndexKindPrimary, Columns: []string{"c"}},
			Constraints: []spec.Constraint{},
		}
		result, enriched := t05A5R1Audit(t, indexSQL[0].sql, spec.DialectMySQL, shape, false, t05PolicyPath(t, map[string]string{t05A5DropIndexExists: ""}))
		if shape.PrimaryKey == nil || len(shape.Indexes) != 2 {
			t.Fatalf("provider payload changed: %#v", shape)
		}
		post := t05A5R1Post(t, enriched)
		t05A5R1AssertColumnsWithheld(t, post)
		if !post.PrimaryKeyUnknown || post.PrimaryKey != nil || !post.IndexesUnknown || len(post.Indexes) != 1 || post.Indexes[0].Name != "idx_k" {
			t.Fatalf("affected scrub = %#v", post)
		}
		if len(t05FindingsByRule(result, 1, t05A5DropIndexExists)) != 0 || len(t05GapsByRule(result, 1, t05A5DropIndexExists)) != 1 {
			t.Fatalf("affected index consumer = %#v %#v", result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
		}
	})
}

func TestAuditSQLT05A5R1DeclarationGatePrecedesPartialSnapshot(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		sql        string
		invalidate bool
	}{
		{name: "primary key", sql: "ALTER TABLE t CHANGE COLUMN c c2 INT PRIMARY KEY NOT NULL;\nALTER TABLE t DROP PRIMARY KEY;", invalidate: true},
		{name: "auto increment", sql: "ALTER TABLE t CHANGE COLUMN c c2 INT AUTO_INCREMENT;\nALTER TABLE t DROP PRIMARY KEY;", invalidate: true},
		{name: "ordinary not null", sql: "ALTER TABLE t CHANGE COLUMN c c2 INT NOT NULL;\nALTER TABLE t DROP PRIMARY KEY;", invalidate: false},
	}
	for _, tc := range cases {
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			tc, dialect := tc, dialect
			t.Run(tc.name+"/"+string(dialect), func(t *testing.T) {
				t.Parallel()
				shape := t05A5R1PartialTable(false)
				result, enriched := t05A5R1Audit(t, tc.sql, dialect, shape, false, t05PolicyPath(t, map[string]string{t05A5DropPKExists: ""}))
				t05A5R1AssertKnownEmpty(t, t05A5R1Pre(t, enriched), false)
				findings := t05FindingsByRule(result, 1, t05A5DropPKExists)
				gaps := t05GapsByRule(result, 1, t05A5DropPKExists)
				if tc.invalidate {
					if enriched[1].Metadata == nil || enriched[1].Metadata.TargetTable != nil {
						t.Fatalf("declaration kept a present table: %#v", enriched[1].Metadata)
					}
					if len(findings) != 0 || len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
						t.Fatalf("drop primary key after declaration = findings %#v gaps %#v", findings, gaps)
					}
					return
				}
				t05A5R1AssertKnownEmpty(t, t05A5R1Post(t, enriched), false)
				if len(findings) != 1 || len(gaps) != 0 || findings[0].Metadata["exists"] != false {
					t.Fatalf("ordinary not null = findings %#v gaps %#v", findings, gaps)
				}
			})
		}
	}
}

func TestAuditSQLT05A5R1SameIdentityTargetHasNoGap(t *testing.T) {
	t.Parallel()
	same := []struct {
		name   string
		sql    string
		ruleID string
		rename bool
	}{
		{name: "change", sql: "ALTER TABLE t CHANGE COLUMN c c INT;", ruleID: t05A5ChangeTarget, rename: false},
		{name: "change case", sql: "ALTER TABLE t CHANGE COLUMN c C INT;", ruleID: t05A5ChangeTarget, rename: false},
		{name: "rename", sql: "ALTER TABLE t RENAME COLUMN c TO c;", ruleID: t05A5RenameTarget, rename: true},
		{name: "rename case", sql: "ALTER TABLE t RENAME COLUMN c TO C;", ruleID: t05A5RenameTarget, rename: true},
	}
	for _, action := range same {
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			for _, knowledge := range []string{"unknown", "columns-nil"} {
				action, dialect, knowledge := action, dialect, knowledge
				t.Run(action.name+"/"+knowledge+"/"+string(dialect), func(t *testing.T) {
					t.Parallel()
					var provider MetadataProvider
					version := ""
					if knowledge == "columns-nil" {
						inner := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": t05A5R1PartialTable(true)}}
						provider = inner
						if action.rename {
							provider = &t05BannerPresent{inner: inner, banner: t05A5R1Banner(dialect)}
						}
					} else if action.rename {
						version = "8.4.10"
						if dialect == spec.DialectTiDB {
							version = "8.5.0"
						}
					}
					result := t05A5AuditWithPolicy(t, action.sql, dialect, provider, version, t05PolicyPath(t, map[string]string{action.ruleID: ""}))
					findings := t05FindingsByRule(result, 0, action.ruleID)
					gaps := t05GapsByRule(result, 0, action.ruleID)
					if len(findings) != 0 || len(gaps) != 0 {
						t.Fatalf("same identity = findings %#v gaps %#v", findings, gaps)
					}
				})
			}
		}
	}

	t.Run("different name still gaps", func(t *testing.T) {
		t.Parallel()
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			dialect := dialect
			t.Run(string(dialect), func(t *testing.T) {
				t.Parallel()
				sql := "ALTER TABLE t CHANGE COLUMN c c2 INT;"
				unknown := t05A5AuditWithPolicy(t, sql, dialect, nil, "", t05PolicyPath(t, map[string]string{t05A5ChangeTarget: ""}))
				if len(t05FindingsByRule(unknown, 0, t05A5ChangeTarget)) != 0 || len(t05GapsByRule(unknown, 0, t05A5ChangeTarget)) != 1 || t05GapsByRule(unknown, 0, t05A5ChangeTarget)[0].ReasonCode != "unknown_table_state" {
					t.Fatalf("unknown different name = %#v %#v", unknown.Statements[0].Findings, unknown.Statements[0].EvidenceGaps)
				}
				withheld := t05A5AuditWithPolicy(t, sql, dialect, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": t05A5R1PartialTable(true)}}, "", t05PolicyPath(t, map[string]string{t05A5ChangeTarget: ""}))
				if len(t05FindingsByRule(withheld, 0, t05A5ChangeTarget)) != 0 || len(t05GapsByRule(withheld, 0, t05A5ChangeTarget)) != 1 {
					t.Fatalf("withheld different name = %#v %#v", withheld.Statements[0].Findings, withheld.Statements[0].EvidenceGaps)
				}
			})
		}
	})

	t.Run("real duplicate still conflicts", func(t *testing.T) {
		t.Parallel()
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			dialect := dialect
			t.Run(string(dialect), func(t *testing.T) {
				t.Parallel()
				shape := presentTable("golden", "t", "c", "c2")
				result, _ := t05A5R1Audit(t, "ALTER TABLE t CHANGE COLUMN c c2 INT;", dialect, shape, false, t05PolicyPath(t, map[string]string{t05A5ChangeTarget: ""}))
				findings := t05FindingsByRule(result, 0, t05A5ChangeTarget)
				if len(findings) != 1 || len(t05GapsByRule(result, 0, t05A5ChangeTarget)) != 0 || findings[0].Metadata["exists"] != true || findings[0].Metadata["source_column"] != "c" || findings[0].Metadata["target_column"] != "c2" {
					t.Fatalf("duplicate = %#v gaps %#v", findings, result.Statements[0].EvidenceGaps)
				}
			})
		}
	})

	t.Run("confirmed absent adds nothing", func(t *testing.T) {
		t.Parallel()
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			dialect := dialect
			t.Run(string(dialect), func(t *testing.T) {
				t.Parallel()
				shape := absentTable("golden", "t")
				result, _ := t05A5R1Audit(t, "ALTER TABLE t CHANGE COLUMN c c INT;", dialect, shape, false, t05PolicyPath(t, map[string]string{t05A5ChangeTarget: ""}))
				if len(t05FindingsByRule(result, 0, t05A5ChangeTarget)) != 0 || len(t05GapsByRule(result, 0, t05A5ChangeTarget)) != 0 {
					t.Fatalf("absent = %#v %#v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
				}
			})
		}
	})

	t.Run("disabled and inapplicable stay silent", func(t *testing.T) {
		t.Parallel()
		disabled := t05A5AuditWithPolicy(t, "ALTER TABLE t CHANGE COLUMN c c INT;", spec.DialectMySQL, nil, "", t05PolicyPath(t, map[string]string{t05RuleAlterRequire: ""}))
		if len(t05FindingsByRule(disabled, 0, t05A5ChangeTarget)) != 0 || len(t05GapsByRule(disabled, 0, t05A5ChangeTarget)) != 0 {
			t.Fatalf("disabled target rule spoke: %#v %#v", disabled.Statements[0].Findings, disabled.Statements[0].EvidenceGaps)
		}
		for _, sql := range []string{
			"ALTER TABLE t MODIFY COLUMN c INT;",
			"ALTER TABLE t DROP INDEX ix;",
			"ALTER TABLE t CHANGE COLUMN c c INT, ADD COLUMN d INT;",
		} {
			result := t05A5AuditWithPolicy(t, sql, spec.DialectMySQL, nil, "", t05PolicyPath(t, map[string]string{t05A5ChangeTarget: ""}))
			if len(t05FindingsByRule(result, 0, t05A5ChangeTarget)) != 0 || len(t05GapsByRule(result, 0, t05A5ChangeTarget)) != 0 {
				t.Fatalf("%s target gaps = %#v %#v", sql, result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
			}
		}
	})

	t.Run("source compatibility and version stay independent", func(t *testing.T) {
		t.Parallel()
		change := "ALTER TABLE t CHANGE COLUMN c c INT;"
		source := t05A5AuditWithPolicy(t, change, spec.DialectMySQL, &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": t05A5R1PartialTable(true)}}, "", t05PolicyPath(t, map[string]string{
			t05A5ChangeTarget: "",
			t05A5ChangeExists: "",
		}))
		if len(t05FindingsByRule(source, 0, t05A5ChangeTarget)) != 0 || len(t05GapsByRule(source, 0, t05A5ChangeTarget)) != 0 || len(t05GapsByRule(source, 0, t05A5ChangeExists)) != 1 {
			t.Fatalf("source existence = findings %#v gaps %#v", source.Statements[0].Findings, source.Statements[0].EvidenceGaps)
		}
		compat := t05A5AuditWithPolicy(t, change, spec.DialectTiDB, nil, "", t05PolicyPath(t, map[string]string{
			t05A5ChangeTarget: "",
			t05A5ChangeCompat: "      required: true\n",
		}))
		if len(t05GapsByRule(compat, 0, t05A5ChangeTarget)) != 0 || len(t05GapsByRule(compat, 0, t05A5ChangeCompat)) != 1 {
			t.Fatalf("compatibility = %#v", compat.Statements[0].EvidenceGaps)
		}
		rename := t05A5AuditWithPolicy(t, "ALTER TABLE t RENAME COLUMN c TO c;", spec.DialectMySQL, nil, "", t05PolicyPath(t, map[string]string{
			t05A5RenameTarget:  "",
			t05A5RenameVersion: "      required: true\n",
			t05A5RenameExists:  "",
		}))
		if len(t05FindingsByRule(rename, 0, t05A5RenameTarget)) != 0 || len(t05GapsByRule(rename, 0, t05A5RenameTarget)) != 0 || len(t05GapsByRule(rename, 0, t05A5RenameVersion)) != 1 || len(t05GapsByRule(rename, 0, t05A5RenameExists)) != 1 {
			t.Fatalf("rename neighbors = findings %#v gaps %#v", rename.Statements[0].Findings, rename.Statements[0].EvidenceGaps)
		}
	})
}

func collectionLabel(nilCollections bool) string {
	if nilCollections {
		return "nil"
	}
	return "empty"
}
