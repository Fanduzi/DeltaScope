// Package spec tests the T10 shared advanced-index assessment: the same
// answer the compatibility rule and the ordered-state publication gate
// consume, computed from declared index facts plus the bound version.
package spec

import (
	"fmt"
	"testing"
)

func t10Version(product, version string, validated bool) *VersionIdentity {
	identity := &VersionIdentity{Product: product, Version: version, ValidatedRange: validated}
	// versionAtLeast compares the numeric components, not the string.
	if _, err := fmt.Sscanf(version, "%d.%d.%d", &identity.Major, &identity.Minor, &identity.Patch); err != nil {
		panic("malformed test version " + version)
	}
	return identity
}

func t10AlterIndexStatement(dialect Dialect, name, visibility string, version *VersionIdentity) Statement {
	return Statement{
		Dialect:  dialect,
		Kind:     KindDDL,
		Metadata: &Metadata{Version: version},
		DDL: &DDL{
			Operation: DDLOperationAlterTable,
			Table:     &Table{Schema: "golden", Name: "t"},
			Alter: []Alter{{
				Action: "alter_index",
				Index: &AlterIndex{
					OldName:    name,
					Definition: &Index{Visibility: visibility, VisibilityExplicit: true},
				},
			}},
		},
	}
}

func t10CreateIndexStatement(dialect Dialect, index Index, version *VersionIdentity) Statement {
	return Statement{
		Dialect:  dialect,
		Kind:     KindDDL,
		Metadata: &Metadata{Version: version},
		DDL: &DDL{
			Operation: DDLOperationCreateIndex,
			Table:     &Table{Schema: "golden", Name: "t"},
			Alter: []Alter{{
				Action: "create_index",
				Index:  &AlterIndex{Definition: &index},
			}},
		},
	}
}

func t10IssueReasons(a AdvancedIndexAssessment) map[string]bool {
	out := map[string]bool{}
	for _, issue := range a.Issues {
		out[issue.ReasonCode] = true
	}
	return out
}

func t10GapReasons(a AdvancedIndexAssessment) map[string]bool {
	out := map[string]bool{}
	for _, gap := range a.Gaps {
		out[gap.ReasonCode] = true
	}
	return out
}

// MySQL 5.7 provably rejects the visibility feature — a determined
// version_unsupported negative that must block publication.
func TestT10AssessVisibilityGateMySQL57(t *testing.T) {
	t.Parallel()
	a := AssessAdvancedIndexes(t10AlterIndexStatement(DialectMySQL, "i", ColumnVisibilityInvisible, t10Version("mysql", "5.7.44", true)))
	if !a.Applies || !a.HasFeature(AdvancedIndexFeatureVisibility) {
		t.Fatalf("visibility feature must register, got %+v", a.Features)
	}
	if !t10IssueReasons(a)[AdvancedReasonVersionUnsupported] || !a.InvalidTransition {
		t.Fatalf("5.7 must produce a blocking version_unsupported issue, got %+v", a.Issues)
	}
	for _, issue := range a.Issues {
		if issue.MinimumVersion != AdvancedIndexMinimumVisibilityMySQL {
			t.Fatalf("issue must name the 8.0.0 floor, got %+v", issue)
		}
	}
}

// MySQL 8.x answers the same statement cleanly: no issues, no gaps.
func TestT10AssessVisibilitySupportedMySQL80(t *testing.T) {
	t.Parallel()
	a := AssessAdvancedIndexes(t10AlterIndexStatement(DialectMySQL, "i", ColumnVisibilityInvisible, t10Version("mysql", "8.0.46", true)))
	if len(a.Issues) != 0 || len(a.Gaps) != 0 || a.InvalidTransition {
		t.Fatalf("supported visibility must stay clean, got issues=%+v gaps=%+v invalid=%v",
			a.Issues, a.Gaps, a.InvalidTransition)
	}
}

// Missing version evidence is a bounded gap, never a silent pass — and the
// statement still publishes (no determined negative).
func TestT10AssessMissingVersionGap(t *testing.T) {
	t.Parallel()
	a := AssessAdvancedIndexes(t10AlterIndexStatement(DialectMySQL, "i", ColumnVisibilityInvisible, nil))
	if !t10GapReasons(a)[AdvancedReasonMissingTargetVersion] {
		t.Fatalf("missing version must surface the bounded gap, got %+v", a.Gaps)
	}
	if len(a.Issues) != 0 || a.InvalidTransition {
		t.Fatalf("an unresolved answer must not fabricate a refusal, got issues=%+v invalid=%v", a.Issues, a.InvalidTransition)
	}
}

// Out-of-range targets produce the range gap, not the missing-version gap.
func TestT10AssessOutOfRangeGap(t *testing.T) {
	t.Parallel()
	a := AssessAdvancedIndexes(t10AlterIndexStatement(DialectMySQL, "i", ColumnVisibilityInvisible, t10Version("mysql", "9.9.9", false)))
	if !t10GapReasons(a)[AdvancedReasonOutOfValidatedRange] || t10GapReasons(a)[AdvancedReasonMissingTargetVersion] {
		t.Fatalf("out-of-range must produce the range gap only, got %+v", a.Gaps)
	}
}

// TiDB descending is a product property, never a version gate: the
// descending_ignored issue fires even when version evidence is absent,
// and it must not block publication (the index stores ascending).
func TestT10AssessDescendingTiDBUnconditional(t *testing.T) {
	t.Parallel()
	for _, version := range []*VersionIdentity{nil, t10Version("tidb", "8.5.0", true)} {
		a := AssessAdvancedIndexes(t10CreateIndexStatement(DialectTiDB,
			Index{Name: "i", DescParts: 1, Columns: []string{"a"}}, version))
		if !t10IssueReasons(a)[AdvancedIndexReasonDescendingIgnored] {
			t.Fatalf("TiDB descending must always emit descending_ignored, version=%+v issues=%+v", version, a.Issues)
		}
		if a.InvalidTransition {
			t.Fatalf("descending_ignored succeeds natively — publication must proceed, version=%+v", version)
		}
	}
}

// MySQL below 8.0 accepts DESC syntax and stores ascending — the same
// silent-storage issue once the version is proven.
func TestT10AssessDescendingMySQL57(t *testing.T) {
	t.Parallel()
	a := AssessAdvancedIndexes(t10CreateIndexStatement(DialectMySQL,
		Index{Name: "i", DescParts: 1, Columns: []string{"a"}}, t10Version("mysql", "5.7.44", true)))
	if !t10IssueReasons(a)[AdvancedIndexReasonDescendingIgnored] || a.InvalidTransition {
		t.Fatalf("5.7 descending must emit non-blocking descending_ignored, got %+v invalid=%v", a.Issues, a.InvalidTransition)
	}
}

// Primary-key invisibility is rejected on every version of both products;
// authored PRIMARY KEY ... VISIBLE is a natively accepted no-op that must
// stay silent.
func TestT10AssessPrimaryVisibility(t *testing.T) {
	t.Parallel()
	invisible := Statement{
		Dialect:  DialectTiDB,
		Kind:     KindDDL,
		Metadata: &Metadata{Version: t10Version("tidb", "8.5.0", true)},
		DDL: &DDL{
			Operation:  DDLOperationCreateTable,
			Table:      &Table{Schema: "golden", Name: "t"},
			PrimaryKey: &Index{Name: "PRIMARY", Kind: IndexKindPrimary, Columns: []string{"id"}, Visibility: ColumnVisibilityInvisible, VisibilityExplicit: true},
		},
	}
	a := AssessAdvancedIndexes(invisible)
	if !t10IssueReasons(a)[AdvancedIndexReasonPrimaryVisibility] || !a.InvalidTransition {
		t.Fatalf("PK invisibility must produce a blocking refusal, got %+v invalid=%v", a.Issues, a.InvalidTransition)
	}

	visible := invisible
	visible.DDL = &DDL{
		Operation:  DDLOperationCreateTable,
		Table:      &Table{Schema: "golden", Name: "t"},
		PrimaryKey: &Index{Name: "PRIMARY", Kind: IndexKindPrimary, Columns: []string{"id"}, Visibility: ColumnVisibilityVisible, VisibilityExplicit: true},
	}
	a = AssessAdvancedIndexes(visible)
	if len(a.Issues) != 0 || a.InvalidTransition {
		t.Fatalf("authored PRIMARY VISIBLE is accepted natively — no refusal, got %+v", a.Issues)
	}
}

// ALTER INDEX `PRIMARY` on MySQL is the same refusal class — a determined
// negative on a proven version.
func TestT10AssessAlterIndexPrimaryMySQL(t *testing.T) {
	t.Parallel()
	a := AssessAdvancedIndexes(t10AlterIndexStatement(DialectMySQL, "primary", ColumnVisibilityInvisible, t10Version("mysql", "8.0.46", true)))
	if !t10IssueReasons(a)[AdvancedIndexReasonPrimaryVisibility] || !a.InvalidTransition {
		t.Fatalf("MySQL ALTER INDEX PRIMARY must refuse, got %+v invalid=%v", a.Issues, a.InvalidTransition)
	}
}

// A multi-valued member is also an expression member — both features
// register, and each gates on its own MySQL minimum.
func TestT10AssessMultiValuedFeaturePairing(t *testing.T) {
	t.Parallel()
	a := AssessAdvancedIndexes(t10CreateIndexStatement(DialectMySQL,
		Index{Name: "i", ExpressionCount: 1, HasExpressionKeys: true, MultiValuedParts: 1},
		t10Version("mysql", "8.0.13", true)))
	if !a.HasFeature(AdvancedIndexFeatureMultiValued) || !a.HasFeature(AdvancedIndexFeatureExpressionKeys) {
		t.Fatalf("multi-valued members are expression members too, got %+v", a.Features)
	}
	if !t10IssueReasons(a)[AdvancedReasonVersionUnsupported] || !a.InvalidTransition {
		t.Fatalf("8.0.13 is below the 8.0.17 multi-valued floor — must refuse, got %+v", a.Issues)
	}
	a80 := AssessAdvancedIndexes(t10CreateIndexStatement(DialectMySQL,
		Index{Name: "i", ExpressionCount: 1, HasExpressionKeys: true, MultiValuedParts: 1},
		t10Version("mysql", "8.0.46", true)))
	if len(a80.Issues) != 0 {
		t.Fatalf("8.0.46 satisfies both floors, got %+v", a80.Issues)
	}
}

// Non-MySQL-family dialects produce an empty assessment — the shared seam
// never activates outside its products.
func TestT10AssessNonMySQLFamilySilent(t *testing.T) {
	t.Parallel()
	a := AssessAdvancedIndexes(t10AlterIndexStatement(DialectPostgreSQL, "i", ColumnVisibilityInvisible, nil))
	if a.Applies || len(a.Features) != 0 || len(a.Issues) != 0 || len(a.Gaps) != 0 {
		t.Fatalf("PostgreSQL must not activate the assessment, got %+v", a)
	}
}
