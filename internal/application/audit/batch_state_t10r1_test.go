// Package audit verifies the T10-R1 primary-key identity corrections and
// member-level advanced-index facts: the implicit primary role promoted onto
// the first qualifying UNIQUE index (MySQL invisible_indexes.result L247-289,
// TiDB GetPrimaryKey), and TiDB's PRIMARY name resolving to a real index
// object only when the PK is not the row handle.
// input: ALTER INDEX / declared-invisibility statements against snapshots
// carrying explicit-PK, qualifying-UNIQUE, and handle-PK shapes
// output: compatibility refusals on the primary role holder, bounded gaps
// when the role cannot be proven, and no falsely published invisible
// post-state for successors to read
// pos: application-layer ordered-state regression for issue #89 T10-R1; the
// transitions under test live in batch_state_index.go and the shared
// determination in internal/domain/spec
// note: if this file changes, update this header and module README.md.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t10r1Dialects are the two observed anchors the shared primary-role
// determination must agree on (MySQL 3522 == TiDB 3522).
var t10r1Dialects = []struct {
	dialect spec.Dialect
	banner  string
}{
	{spec.DialectMySQL, "8.0.46"},
	{spec.DialectTiDB, "8.0.11-TiDB-v8.5.0"},
}

// t10r1CompatVisibilityPolicy isolates the compatibility rule to the
// visibility feature so sibling rules cannot mask the refusal.
func t10r1CompatVisibilityPolicy(t *testing.T) string {
	return t10Policy(t,
		t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - visibility\n"},
	)
}

// t10r1Table builds a snapshot with explicit columns (nullability controlled)
// plus optional primary key and index members.
func t10r1Table(name string, columns []spec.Column, pk *spec.Index, indexes ...spec.Index) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists:     true,
		Table:      &spec.Table{Name: name},
		Columns:    columns,
		PrimaryKey: pk,
		Indexes:    indexes,
	}
}

// A1: an existing table with no explicit primary key and exactly one
// qualifying candidate — ALTER INDEX u_a INVISIBLE is the native 3522
// refusal, and a successor must never read a published invisible state.
func TestT10R1ImplicitPrimaryUniqueInvisibleRefused(t *testing.T) {
	t.Parallel()
	for _, tc := range t10r1Dialects {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{
				"t": t10r1Table("t",
					[]spec.Column{{Name: "a", Type: "int", NotNull: true}, {Name: "b", Type: "int"}},
					nil,
					spec.Index{Name: "u_a", Kind: spec.IndexKindUnique, Columns: []string{"a"}},
				),
			}}
			result := t10Audit(t, tc.dialect,
				"ALTER TABLE t ALTER INDEX u_a INVISIBLE;\nALTER TABLE t ALTER INDEX u_a VISIBLE;",
				provider, t10Policy(t,
					t10Rule{id: t10AlterIndexExist},
					t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - visibility\n"},
				),
			)
			compat := t05FindingsByRule(result, 0, t10IndexCompat)
			if len(compat) != 1 || compat[0].Metadata["reason_code"] != "primary_index_visibility" {
				t.Fatalf("implicit primary role must refuse invisibility, got %+v", result.Statements[0].Findings)
			}
			if result.Verdict != report.VerdictReject {
				t.Fatalf("blocker refusal must reject, got %s", result.Verdict)
			}
			// The refused statement publishes no invisible post-state: the
			// successor reads existence/index evidence gaps from the
			// tombstone, never a member already flipped invisible.
			stmt1 := result.Statements[1]
			if len(t05GapsByRule(result, 1, t10AlterIndexExist)) == 0 {
				t.Fatalf("refused transition must not publish state — successor needs existence gaps, got findings=%+v gaps=%+v",
					stmt1.Findings, stmt1.EvidenceGaps)
			}
		})
	}
}

// A2: the same root cause on the declaration path — a UNIQUE index declared
// INVISIBLE while holding the implicit primary role is refused at create
// time on both dialects.
func TestT10R1CreateTableImplicitPrimaryInvisibleRefused(t *testing.T) {
	t.Parallel()
	for _, tc := range t10r1Dialects {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: tc.banner}
			result := t10Audit(t, tc.dialect,
				"CREATE TABLE t (a INT NOT NULL, UNIQUE KEY u_a (a) INVISIBLE);\nALTER TABLE t ALTER INDEX u_a VISIBLE;",
				provider, t10Policy(t,
					t10Rule{id: t10AlterIndexExist},
					t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - visibility\n"},
				),
			)
			compat := t05FindingsByRule(result, 0, t10IndexCompat)
			if len(compat) != 1 || compat[0].Metadata["reason_code"] != "primary_index_visibility" {
				t.Fatalf("declared implicit-PK invisibility must refuse, got %+v", result.Statements[0].Findings)
			}
			stmt1 := result.Statements[1]
			if len(t05GapsByRule(result, 1, t10AlterIndexExist)) == 0 {
				t.Fatalf("failed CREATE publishes no table — successor needs gaps, got %+v", stmt1.EvidenceGaps)
			}
		})
	}
}

// Same refusal through the add_index and standalone create_index entries —
// the determination is shared, not per-action.
func TestT10R1ImplicitPrimaryInvisibleSharedEntries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sql  string
	}{
		{name: "alter add_unique", sql: "ALTER TABLE t ADD UNIQUE KEY u_a (a) INVISIBLE"},
		{name: "create unique index", sql: "CREATE UNIQUE INDEX u_a ON t (a) INVISIBLE"},
	}
	for _, tc := range t10r1Dialects {
		for _, entry := range cases {
			tc, entry := tc, entry
			t.Run(string(tc.dialect)+"/"+entry.name, func(t *testing.T) {
				t.Parallel()
				provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{
					"t": t10r1Table("t",
						[]spec.Column{{Name: "a", Type: "int", NotNull: true}}, nil),
				}}
				result := t10Audit(t, tc.dialect, entry.sql, provider, t10r1CompatVisibilityPolicy(t))
				compat := t05FindingsByRule(result, 0, t10IndexCompat)
				if len(compat) != 1 || compat[0].Metadata["reason_code"] != "primary_index_visibility" {
					t.Fatalf("%s must refuse via the shared determination, got %+v", entry.name, result.Statements[0].Findings)
				}
			})
		}
	}
}

// Control: with an explicit primary key the qualifying UNIQUE is an ordinary
// member — invisibility is allowed (the compat rule stays silent; only the
// policy warning fires when enabled).
func TestT10R1ExplicitPrimaryKeepsUniqueHidable(t *testing.T) {
	t.Parallel()
	for _, tc := range t10r1Dialects {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{
				"t": t10r1Table("t",
					[]spec.Column{{Name: "a", Type: "int", NotNull: true}, {Name: "b", Type: "int", NotNull: true}},
					&spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"b"}},
					spec.Index{Name: "u_a", Kind: spec.IndexKindUnique, Columns: []string{"a"}},
				),
			}}
			result := t10Audit(t, tc.dialect,
				"ALTER TABLE t ALTER INDEX u_a INVISIBLE;",
				provider, t10r1CompatVisibilityPolicy(t),
			)
			if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
				t.Fatalf("explicit PK removes the role — compat must stay silent, got %+v", result.Statements[0].Findings)
			}
		})
	}
}

// Control: a UNIQUE over a nullable column never holds the role.
func TestT10R1NullableUniqueNotImplicitPrimary(t *testing.T) {
	t.Parallel()
	for _, tc := range t10r1Dialects {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{
				"t": t10r1Table("t",
					[]spec.Column{{Name: "a", Type: "int"}},
					nil,
					spec.Index{Name: "u_a", Kind: spec.IndexKindUnique, Columns: []string{"a"}},
				),
			}}
			result := t10Audit(t, tc.dialect,
				"ALTER TABLE t ALTER INDEX u_a INVISIBLE;",
				provider, t10r1CompatVisibilityPolicy(t),
			)
			if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
				t.Fatalf("nullable member disqualifies the role, got %+v", result.Statements[0].Findings)
			}
		})
	}
}

// Control: an expression-member UNIQUE is provably not the role holder —
// invisible is allowed on both dialects (native-observed e4).
func TestT10R1ExpressionUniqueNotImplicitPrimary(t *testing.T) {
	t.Parallel()
	for _, tc := range t10r1Dialects {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{
				"t": t10r1Table("t",
					[]spec.Column{{Name: "x", Type: "int", NotNull: true}},
					nil,
					spec.Index{Name: "u_f", Kind: spec.IndexKindUnique, ExpressionCount: 1, HasExpressionKeys: true},
				),
			}}
			result := t10Audit(t, tc.dialect,
				"ALTER TABLE t ALTER INDEX u_f INVISIBLE;",
				provider, t10r1CompatVisibilityPolicy(t),
			)
			if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
				t.Fatalf("expression-member unique never holds the role, got %+v", result.Statements[0].Findings)
			}
		})
	}
}

// Ordering: with two qualifying candidates only the first-declared holds the
// role. In-batch declaration order is proven — u_b declared first refuses,
// u_a stays allowed on both dialects (native-observed imp2).
func TestT10R1ImplicitPrimaryOrderDeclared(t *testing.T) {
	t.Parallel()
	for _, tc := range t10r1Dialects {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: tc.banner}
			result := t10Audit(t, tc.dialect,
				"CREATE TABLE imp2 (a INT NOT NULL, b INT NOT NULL, UNIQUE KEY u_b (b), UNIQUE KEY u_a (a));\n"+
					"ALTER TABLE imp2 ALTER INDEX u_a INVISIBLE;\n"+
					"ALTER TABLE imp2 ALTER INDEX u_b INVISIBLE;",
				provider, t10r1CompatVisibilityPolicy(t),
			)
			if len(t05FindingsByRule(result, 1, t10IndexCompat)) != 0 {
				t.Fatalf("second-declared candidate is not the holder, got %+v", result.Statements[1].Findings)
			}
			compat := t05FindingsByRule(result, 2, t10IndexCompat)
			if len(compat) != 1 || compat[0].Metadata["reason_code"] != "primary_index_visibility" {
				t.Fatalf("first-declared candidate holds the role and refuses, got %+v", result.Statements[2].Findings)
			}
		})
	}
}

// Ordering gap: on a provider-loaded snapshot the candidate order is not the
// catalog order (the provider sorts by name) — with two qualifying
// candidates the holder cannot be proven and the answer is a bounded gap,
// never a guess at Indexes[0].
func TestT10R1ImplicitPrimaryOrderGap(t *testing.T) {
	t.Parallel()
	for _, tc := range t10r1Dialects {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{
				"t": t10r1Table("t",
					[]spec.Column{{Name: "a", Type: "int", NotNull: true}, {Name: "b", Type: "int", NotNull: true}},
					nil,
					spec.Index{Name: "u_a", Kind: spec.IndexKindUnique, Columns: []string{"a"}},
					spec.Index{Name: "u_b", Kind: spec.IndexKindUnique, Columns: []string{"b"}},
				),
			}}
			result := t10Audit(t, tc.dialect,
				"ALTER TABLE t ALTER INDEX u_a INVISIBLE;",
				provider, t10r1CompatVisibilityPolicy(t),
			)
			if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
				t.Fatalf("unproven candidate order must not guess a refusal, got %+v", result.Statements[0].Findings)
			}
			gaps := t05GapsByRule(result, 0, t10IndexCompat)
			if len(gaps) != 1 {
				t.Fatalf("unproven order must surface exactly one bounded gap, got %+v", result.Statements[0].EvidenceGaps)
			}
			if result.Statements[0].Coverage.Status != report.CoverageUnverified {
				t.Fatalf("the gap must lower coverage, got %s", result.Statements[0].Coverage.Status)
			}
		})
	}
}

// An unvouched primary-key collection leaves the role unproven — bounded
// gap, never a fabricated absence.
func TestT10R1ImplicitPrimaryUnknownPKGap(t *testing.T) {
	t.Parallel()
	for _, tc := range t10r1Dialects {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			snapshot := t10r1Table("t",
				[]spec.Column{{Name: "a", Type: "int", NotNull: true}},
				nil,
				spec.Index{Name: "u_a", Kind: spec.IndexKindUnique, Columns: []string{"a"}},
			)
			snapshot.PrimaryKeyUnknown = true
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{"t": snapshot}}
			result := t10Audit(t, tc.dialect,
				"ALTER TABLE t ALTER INDEX u_a INVISIBLE;",
				provider, t10r1CompatVisibilityPolicy(t),
			)
			if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
				t.Fatalf("unvouched PK must not fabricate a refusal, got %+v", result.Statements[0].Findings)
			}
			if len(t05GapsByRule(result, 0, t10IndexCompat)) != 1 {
				t.Fatalf("unvouched PK must surface a bounded gap, got %+v", result.Statements[0].EvidenceGaps)
			}
		})
	}
}

// B: TiDB PRIMARY resolves to a real index object when the PK is not the row
// handle (NONCLUSTERED or non-integer clustered PK). VISIBLE on an already
// visible PRIMARY is a successful no-op — no finding, no state loss.
func TestT10R1TiDBPrimaryIndexObjectVisible(t *testing.T) {
	t.Parallel()
	table := t10r1Table("t",
		[]spec.Column{{Name: "a", Type: "int", NotNull: true}, {Name: "b", Type: "int"}},
		&spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"a"}},
		spec.Index{Name: "idx_b", Kind: spec.IndexKindSecondary, Columns: []string{"b"}},
	)
	table.PrimaryKeyIsHandle = false
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": table}}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;\nALTER TABLE t ALTER INDEX idx_b INVISIBLE;",
		provider, t10Policy(t,
			t10Rule{id: t10AlterIndexExist},
			t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - visibility\n"},
		),
	)
	if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("VISIBLE on an addressable PRIMARY is a successful no-op, got findings=%+v gaps=%+v",
			result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
	}
	if result.Statements[0].Coverage.Status != report.CoverageComplete {
		t.Fatalf("no-op success keeps complete coverage, got %s", result.Statements[0].Coverage.Status)
	}
	// The known table state must survive the no-op: stmt1 still resolves.
	if len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("the no-op must not tombstone known state, got %+v", result.Statements[1].EvidenceGaps)
	}
}

// B: the addressable PRIMARY still cannot go invisible — the refusal is the
// compat determined negative (3522), not a missing-index finding.
func TestT10R1TiDBPrimaryIndexObjectInvisibleRefused(t *testing.T) {
	t.Parallel()
	table := t10r1Table("t",
		[]spec.Column{{Name: "a", Type: "int", NotNull: true}, {Name: "b", Type: "int"}},
		&spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"a"}},
	)
	table.PrimaryKeyIsHandle = false
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": table}}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX `PRIMARY` INVISIBLE;",
		provider, t10Policy(t,
			t10Rule{id: t10AlterIndexExist},
			t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - visibility\n"},
		),
	)
	if len(t05FindingsByRule(result, 0, t10AlterIndexExist)) != 0 {
		t.Fatalf("addressable PRIMARY exists — no missing-index finding, got %+v", result.Statements[0].Findings)
	}
	compat := t05FindingsByRule(result, 0, t10IndexCompat)
	if len(compat) != 1 || compat[0].Metadata["reason_code"] != "primary_index_visibility" {
		t.Fatalf("invisible on the PK role is the 3522 refusal, got %+v", result.Statements[0].Findings)
	}
}

// B control: the single-int clustered PK (PKIsHandle) keeps the 1176-class
// missing-index answer — PRIMARY names no index object there.
func TestT10R1TiDBPrimaryHandleMissingIndex(t *testing.T) {
	t.Parallel()
	table := t10r1Table("t",
		[]spec.Column{{Name: "a", Type: "int", NotNull: true}},
		&spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"a"}},
	)
	table.PrimaryKeyIsHandle = true
	provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": table}}
	result := t10Audit(t, spec.DialectTiDB,
		"ALTER TABLE t ALTER INDEX `PRIMARY` VISIBLE;",
		provider, t10ExistsPolicy(t),
	)
	exists := t05FindingsByRule(result, 0, t10AlterIndexExist)
	if len(exists) != 1 || exists[0].Metadata["exists"] != false {
		t.Fatalf("PKIsHandle PRIMARY names no index object — exists blocker expected, got %+v", result.Statements[0].Findings)
	}
}

// Feature acceptance: a BLOB/TEXT member without a prefix is a determined
// product rejection (1170 on both anchors); a written prefix passes.
func TestT10R1PrefixRequiredMemberLegality(t *testing.T) {
	t.Parallel()
	for _, tc := range t10r1Dialects {
		tc := tc
		t.Run(string(tc.dialect), func(t *testing.T) {
			t.Parallel()
			table := t10r1Table("t",
				[]spec.Column{{Name: "b", Type: "text", NotNull: true}},
				nil,
			)
			provider := &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{"t": table}}
			result := t10Audit(t, tc.dialect,
				"CREATE INDEX i ON t(b);",
				provider, t10Policy(t,
					t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - prefix_length\n"},
				),
			)
			compat := t05FindingsByRule(result, 0, t10IndexCompat)
			if len(compat) != 1 || compat[0].Metadata["reason_code"] != "prefix_required" {
				t.Fatalf("unprefixed blob/text member must refuse (1170), got %+v", result.Statements[0].Findings)
			}

			provider = &t10Provider{version: tc.banner, tables: map[string]*spec.TableSnapshot{"t": table}}
			result = t10Audit(t, tc.dialect,
				"CREATE INDEX i ON t(b(10));",
				provider, t10Policy(t,
					t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - prefix_length\n"},
				),
			)
			if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
				t.Fatalf("a written prefix must pass, got %+v", result.Statements[0].Findings)
			}
		})
	}
}

// A prefix-length member keeps its position and length per member — the
// (a(10),b) vs (a,b(10)) distinction rides KeyParts ordinals.
func TestT10R1PrefixMemberFacts(t *testing.T) {
	t.Parallel()
	table := t10r1Table("t",
		[]spec.Column{{Name: "a", Type: "varchar(64)", NotNull: true}, {Name: "b", Type: "int", NotNull: true}},
		nil,
		spec.Index{Name: "i", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b"}},
	)
	table.Indexes[0].SetKeyParts([]spec.IndexKeyPart{
		{Ordinal: 1, Kind: spec.IndexKeyPartColumn, Column: "a", PrefixLength: intPtr(10)},
		{Ordinal: 2, Kind: spec.IndexKeyPartColumn, Column: "b"},
	})
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": table}}
	result := t10Audit(t, spec.DialectMySQL,
		"ALTER TABLE t ALTER INDEX i INVISIBLE;",
		provider, t10VisibilityPolicy(t),
	)
	if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
		t.Fatalf("prefixed index must stay clean, got %+v", result.Statements[0].Findings)
	}
	index := provider.tables["t"].Indexes[0]
	if len(index.KeyParts) != 2 || index.KeyParts[0].PrefixLength == nil || *index.KeyParts[0].PrefixLength != 10 ||
		index.KeyParts[1].PrefixLength != nil || index.KeyParts[1].Ordinal != 2 {
		t.Fatalf("member prefix/ordinal facts lost: %+v", index.KeyParts)
	}
}

func intPtr(v int) *int { return &v }

// FULLTEXT on a non-string column is a determined 1283; SPATIAL binds the
// geometry family and requires NOT NULL (1687/1252).
func TestT10R1FulltextSpatialMemberLegality(t *testing.T) {
	t.Parallel()
	table := t10r1Table("t",
		[]spec.Column{
			{Name: "i", Type: "int", NotNull: true},
			{Name: "txt", Type: "text", NotNull: true},
			{Name: "g", Type: "geometry", NotNull: true},
			{Name: "gn", Type: "geometry"},
		},
		nil,
	)
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": table}}
	policy := func(t *testing.T) string {
		return t10Policy(t, t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - fulltext\n        - spatial\n"})
	}
	cases := []struct {
		name, sql, wantReason string
	}{
		{"fulltext on int", "CREATE FULLTEXT INDEX ft ON t(i);", "fulltext_column_type"},
		{"fulltext on text", "CREATE FULLTEXT INDEX ft ON t(txt);", ""},
		{"spatial on int", "CREATE SPATIAL INDEX sp ON t(i);", "spatial_column_type"},
		{"spatial on nullable geometry", "CREATE SPATIAL INDEX sp ON t(gn);", "spatial_nullable"},
		{"spatial on notnull geometry", "CREATE SPATIAL INDEX sp ON t(g);", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := t10Audit(t, spec.DialectMySQL, tc.sql, provider, policy(t))
			compat := t05FindingsByRule(result, 0, t10IndexCompat)
			if tc.wantReason == "" {
				if len(compat) != 0 {
					t.Fatalf("expected clean, got %+v", compat)
				}
				return
			}
			if len(compat) != 1 || compat[0].Metadata["reason_code"] != tc.wantReason {
				t.Fatalf("want %s, got %+v", tc.wantReason, result.Statements[0].Findings)
			}
		})
	}
}

// Expression member dependencies resolve against the column set: a missing
// reference is the determined 1054 rejection, not a guessed pass.
func TestT10R1ExpressionMemberDependency(t *testing.T) {
	t.Parallel()
	table := t10r1Table("t",
		[]spec.Column{{Name: "a", Type: "int", NotNull: true}},
		nil,
	)
	provider := &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{"t": table}}
	result := t10Audit(t, spec.DialectMySQL,
		"CREATE INDEX i ON t((nosuch+1));",
		provider, t10Policy(t,
			t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - expression_keys\n"},
		),
	)
	compat := t05FindingsByRule(result, 0, t10IndexCompat)
	if len(compat) != 1 || compat[0].Metadata["reason_code"] != "expression_column_missing" {
		t.Fatalf("expression member on a missing column must refuse (1054), got %+v", result.Statements[0].Findings)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("member rejection must reject the verdict, got %s", result.Verdict)
	}
}

// TiDB GLOBAL: the product requires a unique index on a partitioned table —
// non-unique or non-partitioned is a determined 8200, an unproven partition
// fact degrades to a bounded gap.
func TestT10R1TiDBGlobalIndexGate(t *testing.T) {
	t.Parallel()
	policy := func(t *testing.T) string {
		return t10Policy(t, t10Rule{id: t10IndexCompat, params: "      required: true\n      features:\n        - global\n"})
	}
	t.Run("non-partitioned refuses", func(t *testing.T) {
		notPartitioned := false
		table := t10r1Table("t", []spec.Column{{Name: "a", Type: "int", NotNull: true}}, nil)
		table.Partitioned = &notPartitioned
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": table}}
		result := t10Audit(t, spec.DialectTiDB,
			"CREATE UNIQUE INDEX gi ON t(a) GLOBAL;",
			provider, policy(t),
		)
		compat := t05FindingsByRule(result, 0, t10IndexCompat)
		if len(compat) != 1 || compat[0].Metadata["reason_code"] != "global_not_partitioned" {
			t.Fatalf("GLOBAL on non-partitioned must refuse (8200), got %+v", result.Statements[0].Findings)
		}
	})
	t.Run("non-unique refuses", func(t *testing.T) {
		partitioned := true
		table := t10r1Table("t", []spec.Column{{Name: "a", Type: "int", NotNull: true}}, nil)
		table.Partitioned = &partitioned
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": table}}
		result := t10Audit(t, spec.DialectTiDB,
			"CREATE INDEX gi ON t(a) GLOBAL;",
			provider, policy(t),
		)
		compat := t05FindingsByRule(result, 0, t10IndexCompat)
		if len(compat) != 1 || compat[0].Metadata["reason_code"] != "global_not_unique" {
			t.Fatalf("GLOBAL on non-unique must refuse (8200), got %+v", result.Statements[0].Findings)
		}
	})
	t.Run("unique partitioned passes", func(t *testing.T) {
		partitioned := true
		table := t10r1Table("t", []spec.Column{{Name: "a", Type: "int", NotNull: true}}, nil)
		table.Partitioned = &partitioned
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": table}}
		result := t10Audit(t, spec.DialectTiDB,
			"CREATE UNIQUE INDEX gi ON t(a) GLOBAL;",
			provider, policy(t),
		)
		if len(t05FindingsByRule(result, 0, t10IndexCompat)) != 0 {
			t.Fatalf("unique+partitioned GLOBAL must pass, got %+v", result.Statements[0].Findings)
		}
	})
	t.Run("unknown partition gaps", func(t *testing.T) {
		table := t10r1Table("t", []spec.Column{{Name: "a", Type: "int", NotNull: true}}, nil)
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": table}}
		result := t10Audit(t, spec.DialectTiDB,
			"CREATE UNIQUE INDEX gi ON t(a) GLOBAL;",
			provider, policy(t),
		)
		gaps := t05GapsByRule(result, 0, t10IndexCompat)
		if len(gaps) == 0 {
			t.Fatalf("unproven partition state must emit a bounded gap, got findings=%+v gaps=%+v",
				result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
	})
}

// Descending member facts keep per-member direction and ordinal order —
// (a DESC,b) and (a,b DESC) are distinct declarations.
func TestT10R1DescendingMemberFactsDistinct(t *testing.T) {
	t.Parallel()
	table := t10r1Table("t",
		[]spec.Column{{Name: "a", Type: "int", NotNull: true}, {Name: "b", Type: "int", NotNull: true}},
		nil,
	)
	table.Indexes = []spec.Index{
		{Name: "d1", Kind: spec.IndexKindSecondary, Columns: []string{"a", "b"}},
	}
	table.Indexes[0].SetKeyParts([]spec.IndexKeyPart{
		{Ordinal: 1, Kind: spec.IndexKeyPartColumn, Column: "a", Descending: true},
		{Ordinal: 2, Kind: spec.IndexKeyPartColumn, Column: "b"},
	})
	index := table.Indexes[0]
	if index.DescParts != 1 || len(index.KeyParts) != 2 ||
		!index.KeyParts[0].Descending || index.KeyParts[1].Descending ||
		index.KeyParts[0].Ordinal != 1 || index.KeyParts[1].Ordinal != 2 {
		t.Fatalf("member direction/ordinal facts lost: %+v", index.KeyParts)
	}
}
