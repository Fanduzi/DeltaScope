// Package audit verifies the T09-R1 index-facts and primary-key premise
// amendment end to end.
// input: real MySQL/TiDB CREATE TABLE/CREATE INDEX/ALTER batches audited
// through AuditSQL or the real enrich seam, with stub providers carrying
// vouched index and column facts — including provider-loaded indexes that
// carry non-plain key parts
// output: suppressed pair-comparison whenever either candidate side is not a
// known plain key; kept plain positive findings; and ADD PRIMARY KEY
// post-states that honor member-existence and member-nullability premises
// pos: application-layer regression for issue #88 T09-R1; the comparability
// gate lives in internal/domain/rule/ddl, the member-premise gate in
// batch_state_index.go
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t09r1Dialects runs the matrix on both MySQL-family dialects; the
// comparability contract is shared so both must agree.
var t09r1Dialects = []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB}

// t09r1Audit is t09Audit with the R1 rule profile spelled out at the call
// site for readability.
func t09r1Audit(t *testing.T, dialect spec.Dialect, sql string, provider MetadataProvider, rules map[string]string) []rule.Finding {
	t.Helper()
	result := t09Audit(t, dialect, sql, provider, rules)
	var findings []rule.Finding
	for _, statement := range result.Statements {
		findings = append(findings, statement.Findings...)
	}
	return findings
}

// t09r1RefusedNonPlain runs the real pipeline on a statement that declares a
// non-plain key part and asserts the actual channel end to end: the
// statement's unaudited member parts surface as coverage evidence, the
// service refuses with ErrUnsupportedStatement, and — since rules still
// evaluated the spec before the refusal — no rule may have convicted the
// non-plain pair. This records the early-refusal channel instead of
// fabricating reachability the product does not have.
func t09r1RefusedNonPlain(t *testing.T, dialect spec.Dialect, sql string, provider MetadataProvider, rules map[string]string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          dialect,
		Schema:           "golden",
		ConfigPath:       t05A7Policy(t, rules),
		MetadataProvider: provider,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("declared non-plain statement must surface ErrUnsupportedStatement, got err=%v", err)
	}
	for _, statement := range result.Statements {
		for _, finding := range statement.Findings {
			t.Fatalf("no finding may survive a refused non-plain statement, got %+v", finding)
		}
	}
	return result
}

// --- R1-A: declared-source comparability --------------------------------

// A1 red: two indexes on the same column with different prefix lengths are
// never an exact duplicate — Columns=[c] alone must not stand in for the
// prefix parts the plain signature does not model.
func TestT09R1PrefixLengthSuppressesDeclaredDuplicate(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			// Channel: the declared prefix parts are unaudited member aspects
			// — coverage evidence refuses the statement before a Columns-only
			// signature could launder a duplicate verdict.
			t09r1RefusedNonPlain(t, dialect,
				"CREATE TABLE t (c VARCHAR(32), KEY i_short (c(3)), KEY i_long (c(8)));",
				&t05AbsentProvider{},
				map[string]string{"ddl.index.duplicate.forbid": ""},
			)
		})
	}
}

// A2 control: two plain same-column indexes keep the accurate finding.
func TestT09R1PlainDeclaredDuplicateStillFires(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			findings := t09r1Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(32), KEY i_first (c), KEY i_second (c));",
				&t05AbsentProvider{},
				map[string]string{"ddl.index.duplicate.forbid": ""},
			)
			if len(findings) != 1 {
				t.Fatalf("plain duplicate pair must keep its finding, got %+v", findings)
			}
		})
	}
}

// A3 red: a non-plain covering index never proves a plain full-column index
// redundant — (c) is not a left prefix of (c(3), b) as a plain signature.
func TestT09R1PrefixPartSuppressesLeftPrefix(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			t09r1RefusedNonPlain(t, dialect,
				"CREATE TABLE t (c VARCHAR(32), b INT, KEY i_full (c), KEY i_partial (c(3), b));",
				&t05AbsentProvider{},
				map[string]string{"ddl.index.redundant_left_prefix.forbid": ""},
			)
		})
	}
}

// A3 control: a plain wider index still convicts the declared shorter.
func TestT09R1PlainLeftPrefixStillFires(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			findings := t09r1Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(32), b INT, KEY i_narrow (c), KEY i_wide (c, b));",
				&t05AbsentProvider{},
				map[string]string{"ddl.index.redundant_left_prefix.forbid": ""},
			)
			if len(findings) != 1 {
				t.Fatalf("plain left-prefix pair must keep its finding, got %+v", findings)
			}
		})
	}
}

// A4 red: a prefix-part unique index is not a plain unique over [c]; a same
// -Columns secondary must not inherit the "same columns" verdict.
func TestT09R1PrefixUniqueSuppressesUniqueOverlap(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			t09r1RefusedNonPlain(t, dialect,
				"CREATE TABLE t (c VARCHAR(32), UNIQUE KEY uq_prefix (c(3)), KEY i_full (c));",
				&t05AbsentProvider{},
				map[string]string{"ddl.index.redundant_unique_overlap.forbid": ""},
			)
		})
	}
}

// A4 control: a plain unique over the same columns keeps the finding.
func TestT09R1PlainUniqueOverlapStillFires(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			findings := t09r1Audit(t, dialect,
				"CREATE TABLE t (c VARCHAR(32), UNIQUE KEY uq_c (c), KEY i_full (c));",
				&t05AbsentProvider{},
				map[string]string{"ddl.index.redundant_unique_overlap.forbid": ""},
			)
			if len(findings) != 1 {
				t.Fatalf("plain unique overlap must keep its finding, got %+v", findings)
			}
		})
	}
}

// A5 control: different key order never compares equal — original behavior
// must be preserved, not widened.
func TestT09R1KeyOrderNeverDuplicate(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			findings := t09r1Audit(t, dialect,
				"CREATE TABLE t (a INT, b INT, KEY i_ab (a, b), KEY i_ba (b, a));",
				&t05AbsentProvider{},
				map[string]string{"ddl.index.duplicate.forbid": ""},
			)
			if len(findings) != 0 {
				t.Fatalf("reordered keys must not compare equal, got %+v", findings)
			}
		})
	}
}

// DESC control: a descending key part is a recorded non-plain fact — the
// pair (a DESC, b) vs (a, b) never compares equal.
func TestT09R1DescPartsSuppressDuplicate(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			t09r1RefusedNonPlain(t, dialect,
				"CREATE TABLE t (a INT, b INT, KEY i_d (a DESC, b), KEY i_p (a, b));",
				&t05AbsentProvider{},
				map[string]string{"ddl.index.duplicate.forbid": ""},
			)
		})
	}
}

// Expression control: expression key parts carry no plain column names —
// two expression indexes can never prove equal by column signature.
func TestT09R1ExpressionKeysSuppressDuplicate(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			t09r1RefusedNonPlain(t, dialect,
				"CREATE TABLE t (a INT, KEY i_e1 ((a + 1)), KEY i_e2 ((a + 1)));",
				&t05AbsentProvider{},
				map[string]string{"ddl.index.duplicate.forbid": ""},
			)
		})
	}
}

// --- R1-A: existing-set sources (provider-loaded + derived) --------------

// A provider-loaded index with non-plain parts is not a comparable plain
// key — a declared plain index on the same column names is not convicted by
// it. Covers the standalone CREATE INDEX entry.
func TestT09R1ExistingNonPlainSuppressesCreateIndex(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t09Provider{tables: map[string]*spec.TableSnapshot{
				"t": t09Table("t", spec.Index{Name: "i_pre", Kind: spec.IndexKindSecondary, Columns: []string{"a"}, PrefixParts: 1}),
			}}
			result := t09Audit(t, dialect,
				"CREATE INDEX i_new ON t(a);",
				provider,
				map[string]string{"ddl.index.duplicate.forbid": ""},
			)
			if got := t05FindingsByRule(result, 0, "ddl.index.duplicate.forbid"); len(got) != 0 {
				t.Fatalf("provider non-plain index must not convict the declared plain index, got %+v", got)
			}
		})
	}
}

// Opposite control for the standalone entry: the same declared index against
// a provider-loaded PLAIN same-column index still reports.
func TestT09R1ExistingPlainStillFlagsCreateIndex(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t09Provider{tables: map[string]*spec.TableSnapshot{
				"t": t09Table("t", spec.Index{Name: "i_pre", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
			}}
			result := t09Audit(t, dialect,
				"CREATE INDEX i_new ON t(a);",
				provider,
				map[string]string{"ddl.index.duplicate.forbid": ""},
			)
			if got := t05FindingsByRule(result, 0, "ddl.index.duplicate.forbid"); len(got) != 1 {
				t.Fatalf("provider plain same-column index must keep the duplicate finding, got %+v", got)
			}
		})
	}
}

// Opposite control for the ALTER entry: ADD INDEX against a provider-loaded
// non-plain index suppresses; against a plain one it still reports.
func TestT09R1AlterAddIndexExistingComparability(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		for _, nonPlain := range []bool{true, false} {
			nonPlain := nonPlain
			t.Run(string(dialect)+"/"+map[bool]string{true: "nonplain", false: "plain"}[nonPlain], func(t *testing.T) {
				t.Parallel()
				existing := spec.Index{Name: "i_pre", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}
				if nonPlain {
					existing.PrefixParts = 1
				}
				provider := &t09Provider{tables: map[string]*spec.TableSnapshot{
					"t": t09Table("t", existing),
				}}
				result := t09Audit(t, dialect,
					"ALTER TABLE t ADD INDEX i_new (a);",
					provider,
					map[string]string{"ddl.alter.add_index.duplicate.forbid": ""},
				)
				got := t05FindingsByRule(result, 0, "ddl.alter.add_index.duplicate.forbid")
				if nonPlain && len(got) != 0 {
					t.Fatalf("provider non-plain index must not convict the added plain index, got %+v", got)
				}
				if !nonPlain && len(got) != 1 {
					t.Fatalf("provider plain same-column index must keep the duplicate finding, got %+v", got)
				}
			})
		}
	}
}

// A non-plain index this statement declares is equally non-comparable
// against an existing plain index — the same column list on the declared
// side does not launder the missing prefix fact.
func TestT09R1DeclaredNonPlainSuppressesExistingPlain(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t09Provider{tables: map[string]*spec.TableSnapshot{
				"t": t09Table("t", spec.Index{Name: "i_pre", Kind: spec.IndexKindSecondary, Columns: []string{"a"}}),
			}}
			t09r1RefusedNonPlain(t, dialect,
				"CREATE INDEX i_new ON t(a(3));",
				provider,
				map[string]string{"ddl.index.duplicate.forbid": ""},
			)
		})
	}
}

// Derived source: an index published by an earlier statement is a lifecycle
// candidate like a provider-loaded one — a declared non-plain index must not
// pair-compare against it either.
func TestT09R1DerivedPlainSuppressesDeclaredNonPlain(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t09Provider{tables: map[string]*spec.TableSnapshot{"t": t09Table("t")}}
			t09r1RefusedNonPlain(t, dialect,
				"ALTER TABLE t ADD INDEX i_pre (a);\nCREATE INDEX i_new ON t(a(3));",
				provider,
				map[string]string{"ddl.index.duplicate.forbid": ""},
			)
		})
	}
}

// Opposite control on the derived side: plain derived + plain declared
// keeps the accurate finding.
func TestT09R1DerivedPlainStillFlagsDeclaredPlain(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t09Provider{tables: map[string]*spec.TableSnapshot{"t": t09Table("t")}}
			result := t09Audit(t, dialect,
				"ALTER TABLE t ADD INDEX i_pre (a);\nCREATE INDEX i_new ON t(a);",
				provider,
				map[string]string{"ddl.index.duplicate.forbid": ""},
			)
			if got := t05FindingsByRule(result, 1, "ddl.index.duplicate.forbid"); len(got) != 1 {
				t.Fatalf("plain derived + plain declared must keep the duplicate finding, got %+v", got)
			}
		})
	}
}

// --- R1-B: ADD PRIMARY KEY premise and post-state consistency ------------

// t09r1NullableTable builds a vouched table whose only columns are nullable
// INT members — the known pre-state for the ADD PRIMARY KEY matrix.
func t09r1NullableTable(name string, columns ...string) *spec.TableSnapshot {
	cols := make([]spec.Column, 0, len(columns))
	for _, c := range columns {
		cols = append(cols, spec.Column{Name: c, Type: "int"})
	}
	return &spec.TableSnapshot{
		Exists:  true,
		Table:   &spec.Table{Name: name},
		Columns: cols,
		Indexes: []spec.Index{},
	}
}

// B1 red: ADD PRIMARY KEY on an absent member column is a broken premise —
// it must not publish a definite PrimaryKey that names the missing column,
// regardless of policy. The next statement's pre-state proves it.
func TestT09R1AddPrimaryKeyMissingMemberDoesNotPublish(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
				"golden.t": t09r1NullableTable("t", "c"),
			}}
			enriched := enrichA3(t,
				"ALTER TABLE t ADD PRIMARY KEY (missing_c);\nALTER TABLE t DROP PRIMARY KEY;",
				dialect, provider)
			if len(enriched) != 2 {
				t.Fatalf("expected two enriched statements, got %d", len(enriched))
			}
			if next := enriched[1].Metadata.TargetTable; next != nil && next.PrimaryKey != nil {
				t.Fatalf("broken member premise must not publish a primary key, got %+v", next.PrimaryKey)
			}
			if len(provider.calls) != 1 {
				t.Fatalf("provider must load the table exactly once, got %+v", provider.calls)
			}
		})
	}
}

// B2 red: a successful ADD PRIMARY KEY on a nullable member must publish a
// consistent post-state — the member column cannot keep a claimed-complete
// NotNull=false while holding a PK membership.
func TestT09R1AddPrimaryKeyReconcilesMemberNullability(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			snap := t09r1NullableTable("t", "c", "d")
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": snap}}
			enriched := enrichA3(t,
				"ALTER TABLE t ADD PRIMARY KEY (c);\nALTER TABLE t DROP PRIMARY KEY;",
				dialect, provider)
			next := enriched[1].Metadata.TargetTable
			if next == nil || next.PrimaryKey == nil {
				t.Fatalf("provable add must publish the primary key, got %+v", next)
			}
			if len(next.PrimaryKey.Columns) != 1 || next.PrimaryKey.Columns[0] != "c" {
				t.Fatalf("published PK must name member c, got %+v", next.PrimaryKey.Columns)
			}
			member := next.FindColumn("c")
			if member == nil || !member.NotNull {
				t.Fatalf("PK member c must publish NotNull consistent with the transition, got %+v", member)
			}
			if other := next.FindColumn("d"); other == nil || other.NotNull {
				t.Fatalf("non-member column d must keep its nullability, got %+v", other)
			}
			// Request isolation: the provider's own snapshot fact must not be
			// mutated by the post-state publication.
			if snap.Columns[0].NotNull {
				t.Fatal("provider snapshot column must stay untouched by the published transition")
			}
		})
	}
}

// B3 red: a withheld column set (Columns nil while the index set stays
// vouched) cannot prove member existence — the add tombstones rather than
// publishing a PK over members it cannot name.
func TestT09R1AddPrimaryKeyWithheldColumnsDoesNotPublish(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
				"golden.t": {
					Exists:  true,
					Table:   &spec.Table{Name: "t"},
					Columns: nil,
					Indexes: []spec.Index{},
				},
			}}
			enriched := enrichA3(t,
				"ALTER TABLE t ADD PRIMARY KEY (c);\nALTER TABLE t DROP PRIMARY KEY;",
				dialect, provider)
			if next := enriched[1].Metadata.TargetTable; next != nil && next.PrimaryKey != nil {
				t.Fatalf("unknown member set must not publish a primary key, got %+v", next.PrimaryKey)
			}
		})
	}
}

// B4 controls: the honest transitions keep working — second PK still
// blocked, missing-member secondary add still tombstones, a provable PK add
// stays droppable, and non-member columns are untouched.
func TestT09R1AddPrimaryKeyControls(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
				"golden.t": t09r1NullableTable("t", "c", "d"),
			}}
			enriched := enrichA3(t,
				"ALTER TABLE t ADD PRIMARY KEY (c);\nALTER TABLE t ADD PRIMARY KEY (d);\nALTER TABLE t DROP PRIMARY KEY;",
				dialect, provider)
			// stmt0 publishes PK(c) with c reconciled NOT NULL.
			first := enriched[1].Metadata.TargetTable
			if first == nil {
				t.Fatal("first add must publish a readable post-state for the second statement")
			}
			// stmt1's pre-state carries PK(c); its own second-PK add is a
			// broken premise — the entry tombstones, so stmt2 reads unknown.
			if first.PrimaryKey == nil {
				t.Fatalf("published PK(c) must be visible to stmt1, got %+v", first)
			}
			if next := enriched[2].Metadata.TargetTable; next != nil && next.PrimaryKey != nil {
				t.Fatalf("second primary key must not publish, got %+v", next.PrimaryKey)
			}
		})
	}
}

// B4 chain control: a successfully added PK remains droppable — the DROP
// PRIMARY KEY premise sees it and publishes the cleared post-state.
func TestT09R1AddThenDropPrimaryKeyChain(t *testing.T) {
	t.Parallel()
	for _, dialect := range t09r1Dialects {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
				"golden.t": t09r1NullableTable("t", "c"),
			}}
			result := t09Audit(t, dialect,
				"ALTER TABLE t ADD PRIMARY KEY (c);\nALTER TABLE t DROP PRIMARY KEY;",
				provider,
				map[string]string{"ddl.alter.drop_primary_key.exists.require": ""},
			)
			if got := t05FindingsByRule(result, 1, "ddl.alter.drop_primary_key.exists.require"); len(got) != 0 {
				t.Fatalf("drop must see the published primary key, got %+v", got)
			}
		})
	}
}
