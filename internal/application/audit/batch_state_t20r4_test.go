package audit

// T20-R4/#99: reconstructed review-red tests for two settled placement
// behavior defects. These are permanent regression tests written from the
// review record, not copies of the deleted temporary review tests.
//
// B1: both placement rules adjudicate through firstNamedPlacementBinding —
// only the first authored named (non-DEFAULT) binding a statement carries.
// A CREATE TABLE with a proven-existing policy on an earlier member and a
// missing policy on a later member escapes the catalog check entirely, so
// the refused native 8239 outcome is published as an applied create. The
// ordered CREATE state already scans every member def; the rules do not.
//
// B2: member-level PLACEMENT POLICY inside ALTER ... ADD PARTITION /
// REORGANIZE PARTITION lands in alter.PartitionDefs — extracted by the
// parser — but statementsNeedPlacementCatalog and firstNamedPlacementBinding
// never look there. The catalog is never loaded and no rule sees the
// binding, so a named missing policy on a member add/reorg mutates state
// silently instead of refusing and preserving pre-state.
//
// Fixture shape (approved seams only): t20r4Provider wraps t10Provider and
// adds ObjectResolver — schema lookups confirm, other objects stay
// unavailable. t is a range-partitioned table with p0/p_keep both bound to
// p_old; p_ok and p_second exist unbound; p_missing is absent from the
// proven (non-nil) policy catalog — key absence under a proven map is the
// proven-absent answer, never an injected partial map.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type t20r4Provider struct{ *t10Provider }

func (p *t20r4Provider) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type == "schema" {
		return &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Exists: true, Status: spec.MetadataStatusConfirmed}, nil
	}
	return &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
}

func t20r4RangeShape() *spec.TableSnapshot {
	shape := t20PlacementShape("t")
	shape.Table.Schema = "golden"
	partitioned, method := true, "range"
	shape.Partitioned, shape.PartitionType = &partitioned, &method
	shape.PartitionNames = []string{"p0", "p_keep"}
	shape.PartitionDescriptions = []string{"100", "200"}
	shape.PartitionDefs = spec.CatalogPartitionDefs(shape.PartitionNames, shape.PartitionDescriptions, method)
	shape.PartitionPlacements = map[string]string{"p0": "p_old", "p_keep": "p_old"}
	shape.Options["table_rows"] = "9"
	return shape
}

func t20r4Fixture() *t20r4Provider {
	return &t20r4Provider{&t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{"t": t20r4RangeShape()},
		placement: map[string]*spec.PlacementPolicyFact{
			"p_ok":     {BoundTargets: []string{}},
			"p_second": {BoundTargets: []string{}},
			"p_old":    {BoundTargets: []string{"partition:golden.t.p0", "partition:golden.t.p_keep"}},
		},
	}}
}

const (
	t20r4PlacementWarn          = "ddl.tidb.placement.warn"
	t20r4PlacementExistsRequire = "ddl.tidb.placement.exists.require"
)

func t20r4Policies(t *testing.T) (isolated, warnOff string) {
	t.Helper()
	isolated = t05PolicyPathConfigured(t, map[string]t05RuleConfig{
		t20r4PlacementWarn:          {level: "warning"},
		t20r4PlacementExistsRequire: {level: "blocker"},
	})
	warnOff = t05PolicyPathConfigured(t, map[string]t05RuleConfig{
		t20r4PlacementExistsRequire: {level: "blocker"},
	})
	return isolated, warnOff
}

func t20r4Findings(result report.Result, stmt int, id string) []rule.Finding {
	var out []rule.Finding
	for _, f := range result.Statements[stmt].Findings {
		if f.RuleID == id {
			out = append(out, f)
		}
	}
	return out
}

func t20r4HasGap(result report.Result, stmt int, ruleID, fact string) bool {
	for _, g := range result.Statements[stmt].EvidenceGaps {
		if ruleID != "" && g.RuleID != ruleID {
			continue
		}
		for _, f := range g.RequiredFacts {
			if f == fact {
				return true
			}
		}
	}
	return false
}

func t20r4HasPlacementGap(result report.Result, stmt int) bool {
	for _, g := range result.Statements[stmt].EvidenceGaps {
		for _, f := range g.RequiredFacts {
			if strings.HasPrefix(f, "placement_policy.") {
				return true
			}
		}
	}
	return false
}

func t20r4Audit(t *testing.T, provider MetadataProvider, policy, sql string) (report.Result, error) {
	t.Helper()
	return AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          spec.DialectTiDB,
		Schema:           "golden",
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
}

const t20r4CreateLate = "CREATE TABLE t2 (id INT) PARTITION BY RANGE(id) " +
	"(PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=p_ok, " +
	"PARTITION p1 VALUES LESS THAN (200) PLACEMENT POLICY=p_missing);"

const t20r4CreateFirst = "CREATE TABLE t2 (id INT) PARTITION BY RANGE(id) " +
	"(PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=p_missing, " +
	"PARTITION p1 VALUES LESS THAN (200) PLACEMENT POLICY=p_ok);"

const t20r4CreateMiddle = "CREATE TABLE t2 (id INT) PARTITION BY RANGE(id) " +
	"(PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=p_ok, " +
	"PARTITION p1 VALUES LESS THAN (200) PLACEMENT POLICY=p_missing, " +
	"PARTITION p2 VALUES LESS THAN (300) PLACEMENT POLICY=p_second);"

const t20r4CreateLast = "CREATE TABLE t2 (id INT) PARTITION BY RANGE(id) " +
	"(PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=p_ok, " +
	"PARTITION p1 VALUES LESS THAN (200) PLACEMENT POLICY=p_second, " +
	"PARTITION p2 VALUES LESS THAN (300) PLACEMENT POLICY=p_missing);"

const t20r4DefaultThenNamed = "CREATE TABLE t2 (id INT) PARTITION BY RANGE(id) " +
	"(PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=DEFAULT, " +
	"PARTITION p1 VALUES LESS THAN (200) PLACEMENT POLICY=p_ok, " +
	"PARTITION p2 VALUES LESS THAN (300) PLACEMENT POLICY=p_missing);"

const t20r4TableThenPartition = "CREATE TABLE t2 (id INT) PLACEMENT POLICY=p_ok " +
	"PARTITION BY RANGE(id) " +
	"(PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=p_missing);"

const t20r4AddMissing = "ALTER TABLE t ADD PARTITION " +
	"(PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=p_missing);"

const t20r4ReorgMissing = "ALTER TABLE t REORGANIZE PARTITION p0 INTO " +
	"(PARTITION p00 VALUES LESS THAN (50) PLACEMENT POLICY=p_ok, " +
	"PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=p_missing);"

const t20r4FixtureDescription = "t20r4Provider(t10Provider{version=8.0.11-TiDB-v8.5.0, " +
	"tables={t: range partition p0<100/p_keep<200 both bound to p_old, options table_rows=9}, " +
	"placement={p_ok:[], p_second:[], p_old:[partition:golden.t.p0 partition:golden.t.p_keep]}, " +
	"p_missing proven absent by key omission}; ResolveObject schema->confirmed else->unavailable)"

// Every authored missing-policy bind — wherever it lands in the statement —
// must surface the same proven refusal: exists.require blocker, no bind
// warning, no topology gap, reject verdict. First-binding-only scans let
// later CREATE member binds and every member-level alter def slip through.
func TestT20R4PublicMissingBindings(t *testing.T) {
	isolated, warnOff := t20r4Policies(t)
	profiles := []struct {
		name   string
		path   string
		strict bool
	}{
		{"isolated", isolated, true},
		{"default", "", false},
		{"warn_off", warnOff, true},
	}
	cases := []struct {
		name string
		sql  string
	}{
		{"create_late", t20r4CreateLate},
		{"create_first", t20r4CreateFirst},
		{"create_middle", t20r4CreateMiddle},
		{"create_last", t20r4CreateLast},
		{"default_then_named", t20r4DefaultThenNamed},
		{"table_then_partition", t20r4TableThenPartition},
		{"add_missing", t20r4AddMissing},
		{"reorg_missing", t20r4ReorgMissing},
	}
	for _, tc := range cases {
		for _, profile := range profiles {
			t.Run(tc.name+"/"+profile.name, func(t *testing.T) {
				provider := t20r4Fixture()
				t.Logf("fixture: %s", t20r4FixtureDescription)
				t.Logf("case=%s profile=%s config=%q sql=%s", tc.name, profile.name, profile.path, tc.sql)
				result, err := t20r4Audit(t, provider, profile.path, tc.sql)
				if err != nil {
					t.Errorf("audit error for %q: %v", tc.sql, err)
					return
				}
				if data, marshalErr := json.Marshal(result); marshalErr == nil {
					t.Logf("public report: %s", data)
				}
				if len(result.Statements) != 1 {
					t.Errorf("statements = %d, want 1", len(result.Statements))
					return
				}
				if len(result.Unsupported) != 0 {
					t.Errorf("unsupported = %#v, want none", result.Unsupported)
				}
				if len(result.Diagnostics) != 0 {
					t.Errorf("diagnostics = %#v, want none", result.Diagnostics)
				}
				found := t20r4Findings(result, 0, t20r4PlacementExistsRequire)
				missingPolicy := false
				for _, f := range found {
					if f.Level == rule.LevelBlocker && f.Metadata["policy"] == "p_missing" {
						missingPolicy = true
					}
				}
				if !missingPolicy {
					t.Errorf("missing exists.require blocker with policy=p_missing, got findings %#v",
						result.Statements[0].Findings)
				}
				if warns := t20r4Findings(result, 0, t20r4PlacementWarn); len(warns) != 0 {
					t.Errorf("refused statement must not warn on surviving binds, got %#v", warns)
				}
				if t20r4HasGap(result, 0, "", "placement_policy.topology") {
					t.Errorf("refused statement must not carry topology gaps, got %#v",
						result.Statements[0].EvidenceGaps)
				}
				if profile.strict {
					if t20r4HasPlacementGap(result, 0) {
						t.Errorf("unexpected placement evidence gaps under %s: %#v",
							profile.name, result.Statements[0].EvidenceGaps)
					}
					if result.Statements[0].Coverage.Status != report.CoverageComplete {
						t.Errorf("coverage = %s, want complete under %s",
							result.Statements[0].Coverage.Status, profile.name)
					}
				}
				if result.Verdict != report.VerdictReject {
					t.Errorf("verdict = %s, want reject", result.Verdict)
				}
			})
		}
	}
}

// A proven refusal preserves exact pre-state: the member mutation must not
// publish and the proven catalog projections must retain the old references.
// The companion batch seam (Parse -> Extract -> enrich) exposes the ordered
// pre-state the second statement reads.
func TestT20R4RefusedMemberState(t *testing.T) {
	cases := []struct {
		name   string
		sql    string
		target string
	}{
		{"create_late", t20r4CreateLate, "t2"},
		{"add_missing", t20r4AddMissing, "t"},
		{"reorg_missing", t20r4ReorgMissing, "t"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := t20r4Fixture()
			original := cloneTableSnapshot(provider.tables["t"])
			batchSQL := tc.sql + "ALTER TABLE " + tc.target + " TRUNCATE PARTITION p0;"
			t.Logf("fixture: %s", t20r4FixtureDescription)
			t.Logf("case=%s batch=%s", tc.name, batchSQL)
			_, statements := t17Enrich(t, spec.DialectTiDB, batchSQL, "golden", t20v805, provider)
			if len(statements) != 2 {
				t.Errorf("statements = %d, want 2", len(statements))
				return
			}
			metadata := statements[1].Metadata
			var snapshot *spec.TableSnapshot
			if metadata == nil || len(metadata.TargetTables) != 1 {
				t.Errorf("second target bindings = %#v, want exactly one", metadata)
			} else {
				snapshot = metadata.TargetTables[0].Snapshot
			}
			policies := map[string]*spec.PlacementPolicyFact(nil)
			if metadata != nil {
				policies = metadata.PlacementPolicies
			}
			for i, statement := range statements {
				if statement.Metadata != nil {
					for name, fact := range statement.Metadata.PlacementPolicies {
						bounds := []string(nil)
						if fact != nil {
							bounds = fact.BoundTargets
						}
						t.Logf("stmt %d policy %s bound_targets=%v", i, name, bounds)
					}
				}
			}
			if snapshot != nil {
				t.Logf("second-statement snapshot: exists=%v partition_names=%v partition_defs=%+v "+
					"partition_placements=%v options=%v",
					snapshot.Exists, snapshot.PartitionNames, snapshot.PartitionDefs,
					snapshot.PartitionPlacements, snapshot.Options)
			}
			if tc.name == "create_late" {
				if snapshot == nil || snapshot.Exists {
					t.Errorf("refused create must leave t2 proven absent, got %#v", snapshot)
				}
			} else if snapshot == nil {
				t.Errorf("second statement must bind t's snapshot, got nil")
			} else if !reflect.DeepEqual(snapshot, original) {
				t.Errorf("refused %s must preserve exact pre-state snapshot", tc.name)
			}
			if !reflect.DeepEqual(provider.tables["t"], original) {
				t.Errorf("provider snapshot mutated in place")
			}
			if policies == nil {
				t.Errorf("second statement must carry the placement policy projection")
			} else {
				assertBounds := func(policy string, want []string) {
					fact := policies[policy]
					if fact == nil {
						t.Errorf("policy %s missing from projection, got %v", policy, policies)
						return
					}
					if fact.BoundTargets == nil {
						t.Errorf("policy %s bound_targets = nil, want proven non-nil set %v", policy, want)
						return
					}
					if !reflect.DeepEqual(asSet(fact.BoundTargets), asSet(want)) {
						t.Errorf("policy %s bound_targets = %v, want %v", policy, fact.BoundTargets, want)
					}
				}
				assertBounds("p_old", []string{"partition:golden.t.p0", "partition:golden.t.p_keep"})
				assertBounds("p_ok", nil)
				assertBounds("p_second", nil)
				for name, fact := range policies {
					if fact == nil {
						continue
					}
					for _, ref := range fact.BoundTargets {
						for _, member := range []string{".p5", ".p00", ".p01"} {
							if strings.HasSuffix(ref, member) {
								t.Errorf("refused member mutation leaked ref %q into policy %s", ref, name)
							}
						}
					}
				}
			}
		})
	}
}

func asSet(in []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, v := range in {
		out[v] = struct{}{}
	}
	return out
}

const t20r4CreateAllPresent = "CREATE TABLE t2 (id INT) PARTITION BY RANGE(id) " +
	"(PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=p_ok, " +
	"PARTITION p1 VALUES LESS THAN (200) PLACEMENT POLICY=p_second);"

const t20r4CreateSamePolicy = "CREATE TABLE t2 (id INT) PARTITION BY RANGE(id) " +
	"(PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=p_ok, " +
	"PARTITION p1 VALUES LESS THAN (200) PLACEMENT POLICY=p_ok);"

const t20r4AddPresent = "ALTER TABLE t ADD PARTITION " +
	"(PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=p_ok);"

const t20r4ReorgPresent = "ALTER TABLE t REORGANIZE PARTITION p0 INTO " +
	"(PARTITION p00 VALUES LESS THAN (50) PLACEMENT POLICY=p_ok, " +
	"PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=p_ok);"

const t20r4AddDefault = "ALTER TABLE t ADD PARTITION " +
	"(PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=DEFAULT);"

const t20r4ReorgDefault = "ALTER TABLE t REORGANIZE PARTITION p0 INTO " +
	"(PARTITION p00 VALUES LESS THAN (50) PLACEMENT POLICY=DEFAULT, " +
	"PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=DEFAULT);"

// Controls pin the behavior boundary around the defects: proven binds warn
// with exactly one topology gap, unvouched catalogs owe the catalog fact on
// both rules without findings, and DEFAULT binds stay silent. Member-level
// alter binds must reach the same seam — silently skipping them is the B2
// defect this test reconstructs.
func TestT20R4PublicBindingControls(t *testing.T) {
	bothRules, _ := t20r4Policies(t)
	presentCases := []struct {
		name string
		sql  string
	}{
		{"create_all_present", t20r4CreateAllPresent},
		{"create_same_policy", t20r4CreateSamePolicy},
		{"add_ok", t20r4AddPresent},
		{"reorg_ok", t20r4ReorgPresent},
	}
	for _, tc := range presentCases {
		t.Run("present/"+tc.name, func(t *testing.T) {
			provider := t20r4Fixture()
			t.Logf("fixture: %s", t20r4FixtureDescription)
			t.Logf("case=%s policy=isolated sql=%s", tc.name, tc.sql)
			result, err := t20r4Audit(t, provider, bothRules, tc.sql)
			if err != nil {
				t.Errorf("audit error for %q: %v", tc.sql, err)
				return
			}
			if data, marshalErr := json.Marshal(result); marshalErr == nil {
				t.Logf("public report: %s", data)
			}
			if len(result.Statements) != 1 {
				t.Errorf("statements = %d, want 1", len(result.Statements))
				return
			}
			if warns := t20r4Findings(result, 0, t20r4PlacementWarn); len(warns) == 0 {
				t.Errorf("proven bind must warn, got findings %#v", result.Statements[0].Findings)
			}
			if found := t20r4Findings(result, 0, t20r4PlacementExistsRequire); len(found) != 0 {
				t.Errorf("proven policies must not draw existence findings, got %#v", found)
			}
			topology := 0
			for _, g := range result.Statements[0].EvidenceGaps {
				if g.RuleID != t20r4PlacementWarn || g.ReasonCode != "unknown_table_state" {
					continue
				}
				for _, f := range g.RequiredFacts {
					if f == "placement_policy.topology" {
						topology++
					}
				}
			}
			if topology != 1 {
				t.Errorf("topology gaps = %d, want exactly one, got %#v",
					topology, result.Statements[0].EvidenceGaps)
			}
			if result.Statements[0].Coverage.Status != report.CoverageUnverified {
				t.Errorf("coverage = %s, want unverified", result.Statements[0].Coverage.Status)
			}
			if result.Verdict != report.VerdictReview {
				t.Errorf("verdict = %s, want review", result.Verdict)
			}
		})
	}
	for _, tc := range presentCases {
		t.Run("unknown/"+tc.name, func(t *testing.T) {
			provider := t20r4Fixture()
			provider.placement = nil
			t.Logf("fixture: %s with placement=nil (unvouched catalog)", t20r4FixtureDescription)
			t.Logf("case=%s policy=isolated sql=%s", tc.name, tc.sql)
			result, err := t20r4Audit(t, provider, bothRules, tc.sql)
			if err != nil {
				t.Errorf("audit error for %q: %v", tc.sql, err)
				return
			}
			if len(result.Statements) != 1 {
				t.Errorf("statements = %d, want 1", len(result.Statements))
				return
			}
			if found := t20r4Findings(result, 0, t20r4PlacementExistsRequire); len(found) != 0 {
				t.Errorf("unvouched catalog must not fabricate existence blockers, got %#v", found)
			}
			if warns := t20r4Findings(result, 0, t20r4PlacementWarn); len(warns) != 0 {
				t.Errorf("unproven binds must not warn, got %#v", warns)
			}
			if t20r4HasGap(result, 0, "", "placement_policy.topology") {
				t.Errorf("unproven binds must not fabricate topology gaps, got %#v",
					result.Statements[0].EvidenceGaps)
			}
			if !t20r4HasGap(result, 0, t20r4PlacementWarn, "placement_policy.catalog") {
				t.Errorf("unvouched catalog must owe the warn catalog gap, got %#v",
					result.Statements[0].EvidenceGaps)
			}
			if !t20r4HasGap(result, 0, t20r4PlacementExistsRequire, "placement_policy.catalog") {
				t.Errorf("unvouched catalog must owe the exists.require catalog gap, got %#v",
					result.Statements[0].EvidenceGaps)
			}
		})
	}
	defaultCases := []struct {
		name string
		sql  string
	}{
		{"add_default", t20r4AddDefault},
		{"reorg_default", t20r4ReorgDefault},
	}
	for _, tc := range defaultCases {
		t.Run("default_bind/"+tc.name, func(t *testing.T) {
			provider := t20r4Fixture()
			t.Logf("fixture: %s", t20r4FixtureDescription)
			t.Logf("case=%s policy=isolated sql=%s", tc.name, tc.sql)
			result, err := t20r4Audit(t, provider, bothRules, tc.sql)
			if err != nil {
				t.Errorf("audit error for %q: %v", tc.sql, err)
				return
			}
			if len(result.Statements) != 1 {
				t.Errorf("statements = %d, want 1", len(result.Statements))
				return
			}
			for _, id := range []string{t20r4PlacementWarn, t20r4PlacementExistsRequire} {
				if found := t20r4Findings(result, 0, id); len(found) != 0 {
					t.Errorf("DEFAULT binds stay silent, got %s findings %#v", id, found)
				}
			}
			if t20r4HasPlacementGap(result, 0) {
				t.Errorf("DEFAULT binds with all bounds known owe no placement gaps, got %#v",
					result.Statements[0].EvidenceGaps)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// R1 discriminating tests — lead review findings A–F. Each exercises a
// settled defect the consolidated rework must close; written red-first.
// ---------------------------------------------------------------------------

// R1-B2/A: an unproven member directory or unproven binding premise must not
// publish member refs or an applied warning. REORG p_ok + p_unknown leaves
// p_ok's proven-empty set unvouched — a subsequent DROP POLICY p_ok owes a
// bound-target gap, never a silent success or a refused-by-real-ref finding.
func TestT20R4R1UncertainReorgThenDropOK(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	sql := "ALTER TABLE t REORGANIZE PARTITION p0 INTO " +
		"(PARTITION p00 VALUES LESS THAN (50) PLACEMENT POLICY=p_ok, " +
		"PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=p_unknown); " +
		"DROP PLACEMENT POLICY p_ok;"
	fixture := t20r4Fixture()
	// p_unknown makes the member binding premise unvouched (registry nil).
	fixture.placement = nil
	result, err := t20r4Audit(t, fixture, isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements = %d, want 2", len(result.Statements))
	}
	// Statement 0: unproven binding owes the catalog gap, no warn, no exists blocker.
	if finds := t20r4Findings(result, 0, t20r4PlacementExistsRequire); len(finds) != 0 {
		t.Errorf("stmt0 exists findings = %v, want none for unproven premise", finds)
	}
	if finds := t20r4Findings(result, 0, t20r4PlacementWarn); len(finds) != 0 {
		t.Errorf("stmt0 warn findings = %v, want none while premise unproven", finds)
	}
	if !t20r4HasPlacementGap(result, 0) {
		t.Errorf("stmt0 owes a placement gap for the unproven premise")
	}
	// Statement 1: DROP p_ok — the member set may have gained a p_ok ref, so
	// the bound-target premise is unvouched. Gap required; silent success or a
	// false bound-blocker are both wrong.
	if finds := t20r4Findings(result, 1, t20r4PlacementExistsRequire); len(finds) != 0 {
		t.Errorf("stmt1 drop blocker = %v, want gap instead of fabricated ref refusal", finds)
	}
	if !t20r4HasGap(result, 1, t20r4PlacementExistsRequire, "placement_policy.bound_targets") {
		t.Errorf("stmt1 DROP p_ok must owe placement_policy.bound_targets gap")
	}
}

// R1-C: REORG retires only the source member's override+ref; the p_keep
// survivor still refuses DROP p_old. After the surviving override is
// released with DEFAULT, the final old ref removal lets DROP succeed.
func TestT20R4R1ReorgSurvivorRefAndRelease(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	sql := "ALTER TABLE t REORGANIZE PARTITION p0 INTO " +
		"(PARTITION p00 VALUES LESS THAN (50) PLACEMENT POLICY=p_ok, " +
		"PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=p_second); " +
		"DROP PLACEMENT POLICY p_old; " +
		"ALTER TABLE t PARTITION p_keep PLACEMENT POLICY=DEFAULT; " +
		"DROP PLACEMENT POLICY p_old;"
	result, err := t20r4Audit(t, t20r4Fixture(), isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	finds := t20r4Findings(result, 1, t20r4PlacementExistsRequire)
	if len(finds) == 0 {
		t.Fatalf("DROP p_old must stay refused — p_keep still bound")
	}
	meta := finds[0].Metadata
	refs, _ := meta["bound_targets"].([]string)
	found := false
	for _, ref := range refs {
		if strings.Contains(ref, "p_keep") {
			found = true
		}
		if strings.Contains(ref, ".p0") || strings.Contains(ref, "p00") || strings.Contains(ref, "p01") {
			t.Errorf("bound_targets %v must not carry retired or new members", refs)
		}
	}
	if !found {
		t.Errorf("bound_targets %v must retain partition:golden.t.p_keep", refs)
	}
	t20r4AssertAppliedPolicyDrop(t, result, 3)
	_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, t20r4Fixture())
	bound := t20r4PolicyAt(t, statements, 3).BoundTargets
	if bound == nil || len(*bound) != 0 {
		t.Fatalf("released old policy needs proven-empty refs, got %v", bound)
	}
}

// R1-A/D: an unqualified member finding must carry the effective schema
// identity, and each missing member keeps its own finding.
func TestT20R4R1FindingIdentities(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	result, err := t20r4Audit(t, t20r4Fixture(), isolated, t20r4CreateLate)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	finds := t20r4Findings(result, 0, t20r4PlacementExistsRequire)
	if len(finds) == 0 {
		t.Fatalf("missing finding expected for p_missing")
	}
	meta := finds[0].Metadata
	if meta["schema"] != "golden" {
		t.Errorf("finding schema = %v, want golden", meta["schema"])
	}
	if meta["table"] != "t2" {
		t.Errorf("finding table = %v, want t2", meta["table"])
	}
	if meta["partition"] != "p1" {
		t.Errorf("finding partition = %v, want p1", meta["partition"])
	}
	if meta["policy"] != "p_missing" {
		t.Errorf("finding policy = %v, want p_missing", meta["policy"])
	}
}

// R1-B: CREATE with a refused member binding marks MutationOutcome refused
// — the warn rule stays silent even for the earlier applied binding.
func TestT20R4R1RefusedCreateSilentWarn(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	result, err := t20r4Audit(t, t20r4Fixture(), isolated, t20r4CreateMiddle)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if finds := t20r4Findings(result, 0, t20r4PlacementWarn); len(finds) != 0 {
		t.Errorf("warn findings = %v, want none — aggregate refusal silences", finds)
	}
	finds := t20r4Findings(result, 0, t20r4PlacementExistsRequire)
	if len(finds) != 1 {
		t.Fatalf("exists findings = %d, want exactly one (p_missing member)", len(finds))
	}
}

// R1-E: TiDB 8.5 subpartition axis is a bounded ASPECT boundary — the
// statement is not suppressed from rule evaluation, the parent RANGE and a
// table-level binding still produce findings, coverage reports the
// subpartition aspect, and no child member becomes a finding.
func TestT20R4R1SubpartitionAspectBoundary(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	sql := "CREATE TABLE t_sub (id INT) PLACEMENT POLICY=p_ok " +
		"PARTITION BY RANGE(id) SUBPARTITION BY HASH(id) (" +
		"PARTITION p0 VALUES LESS THAN (100) (" +
		"SUBPARTITION s0 PLACEMENT POLICY=t20r4_missing, " +
		"SUBPARTITION s1 PLACEMENT POLICY=DEFAULT));"
	result, err := t20r4Audit(t, t20r4Fixture(), isolated, sql)
	// The bounded aspect surfaces through ErrUnsupportedStatement — the
	// partial report stays usable: sibling rules still evaluate (T20-R4).
	if err != nil && !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements = %d, want 1", len(result.Statements))
	}
	stmt := result.Statements[0]
	// Whole-statement unsupported suppression must NOT appear — rules still
	// run against the primary form, and coverage reports the aspect.
	if stmt.Coverage.Status == report.CoverageComplete {
		t.Errorf("coverage = complete, want incomplete under the subpartition aspect")
	}
	// Table-level proven binding still warns.
	if finds := t20r4Findings(result, 0, t20r4PlacementWarn); len(finds) == 0 {
		t.Errorf("parent table binding must still warn under the aspect boundary")
	}
	// Coverage: the tidb_subpartition aspect gap appears in the report's
	// unsupported surface as a bounded vendor boundary.
	foundAspect := false
	for _, u := range result.Unsupported {
		if u.Feature == "tidb_subpartition" {
			foundAspect = true
			if u.Reason != spec.UnsupportedVendorBoundaryReason {
				t.Errorf("tidb_subpartition reason = %v, want vendor boundary", u.Reason)
			}
		}
	}
	if !foundAspect {
		t.Errorf("tidb_subpartition aspect missing from report.Unsupported: %v", result.Unsupported)
	}
	// The ignored child policy never becomes an exists blocker.
	for _, f := range t20r4Findings(result, 0, t20r4PlacementExistsRequire) {
		if f.Metadata["policy"] == "t20r4_missing" {
			t.Errorf("ignored subpartition policy must not produce a blocker: %v", f)
		}
	}
}

// R1-C: incomplete positive sets sort before returning, never exposing
// insertion order.
func TestT20R4R1BoundSetSortedOutput(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	sql := "ALTER TABLE t ADD PARTITION (PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=p_ok); " +
		"ALTER TABLE t REORGANIZE PARTITION p0 INTO " +
		"(PARTITION p00 VALUES LESS THAN (50) PLACEMENT POLICY=p_ok, " +
		"PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=p_ok); " +
		"DROP PLACEMENT POLICY p_ok;"
	result, err := t20r4Audit(t, t20r4Fixture(), isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	finds := t20r4Findings(result, 2, t20r4PlacementExistsRequire)
	if len(finds) == 0 {
		t.Fatalf("DROP p_ok must refuse — proven refs exist")
	}
	refs, _ := finds[0].Metadata["bound_targets"].([]string)
	if len(refs) < 2 {
		t.Fatalf("bound_targets = %v, want >=2 proven refs", refs)
	}
	for i := 1; i < len(refs); i++ {
		if refs[i-1] >= refs[i] {
			t.Errorf("bound_targets not sorted: %v", refs)
		}
	}
}

// R1-B/C: a proven-absent table refuses without warn; an unknown target
// marks the named policy's ref completeness unknown and owes
// target_table.existence — no member publication.
func TestT20R4R1AbsentAndUnknownTargets(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	// t2 is proven absent under this fixture.
	result, err := t20r4Audit(t, t20r4Fixture(), isolated,
		"ALTER TABLE t2 ADD PARTITION (PARTITION p0 VALUES LESS THAN (1) PLACEMENT POLICY=p_ok);")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if finds := t20r4Findings(result, 0, t20r4PlacementWarn); len(finds) != 0 {
		t.Errorf("absent table must not warn: %v", finds)
	}
}

// R1-F: provider catalog load errors fail loudly — never a silent gap.
func TestT20R4R1ProviderLoaderError(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	fixture := t20r4Fixture()
	fixture.placementErr = errors.New("simulated catalog failure")
	_, err := t20r4Audit(t, fixture, isolated, t20r4AddMissing)
	if err == nil {
		t.Fatalf("provider error must propagate, not degrade to a gap")
	}
}

// R1-F: same member name in two schemas keeps distinct ref identities.
func TestT20R4R1QualifiedMemberRefs(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	// Qualified other.t resolves through the same name-keyed provider but
	// lands refs under its own schema identity.
	sql := "ALTER TABLE other.t ADD PARTITION (PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=p_ok); " +
		"ALTER TABLE golden.t ADD PARTITION (PARTITION p6 VALUES LESS THAN (600) PLACEMENT POLICY=p_ok); " +
		"DROP PLACEMENT POLICY p_ok;"
	fixture := t20r4Fixture()
	fixture.tables["t"].PartitionPlacements = map[string]string{"p0": "", "p_keep": ""}
	fixture.placement["p_old"].BoundTargets = []string{}
	result, err := t20r4Audit(t, &t20r4QualifiedProvider{fixture}, isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	finds := t20r4Findings(result, 2, t20r4PlacementExistsRequire)
	if len(finds) == 0 {
		t.Fatalf("DROP p_ok must refuse — proven refs under both schemas")
	}
	refs, _ := finds[0].Metadata["bound_targets"].([]string)
	var goldenRef, otherRef bool
	for _, ref := range refs {
		if ref == "partition:golden.t.p6" {
			goldenRef = true
		}
		if ref == "partition:other.t.p5" {
			otherRef = true
		}
	}
	if !goldenRef || !otherRef {
		t.Errorf("bound_targets = %v, want distinct refs under both schemas", refs)
	}
}

// R1-F: collector positional matrix — real parser, enrichment-attached
// records keep exact source/action/definition indices and identities.
func TestT20R4R1CollectorPositionalMatrix(t *testing.T) {
	sql := "CREATE TABLE t2 (id INT) PLACEMENT POLICY=p_ok PARTITION BY RANGE(id) " +
		"(PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=p_second, " +
		"PARTITION p1 VALUES LESS THAN (200) PLACEMENT POLICY=DEFAULT, " +
		"PARTITION p2 VALUES LESS THAN (300));"
	_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, t20r4Fixture())
	if len(statements) != 1 {
		t.Fatalf("statements = %d, want 1", len(statements))
	}
	stmt := statements[0]
	if stmt.Metadata == nil || stmt.Metadata.Placement == nil {
		t.Fatalf("no placement facts attached")
	}
	bindings := stmt.Metadata.Placement.Bindings
	if len(bindings) != 3 {
		t.Fatalf("bindings = %d, want 3 (table + p0 + p1; omitted p2 unbound), got %v", len(bindings), bindings)
	}
	// Table-level direct bind.
	if bindings[0].Target.Source != "direct" || bindings[0].Target.TargetKind != "table" ||
		bindings[0].Target.Binding.Policy != "p_ok" || bindings[0].Target.ActionIndex != -1 {
		t.Errorf("binding[0] = %+v, want direct table p_ok", bindings[0].Target)
	}
	// Member defs: definition index preserved, partition identity exact.
	if bindings[1].Target.Source != "member_definition" || bindings[1].Target.DefinitionIndex != 0 ||
		bindings[1].Target.Partition != "p0" || bindings[1].Target.Binding.Policy != "p_second" {
		t.Errorf("binding[1] = %+v, want member_definition idx0 p0 p_second", bindings[1].Target)
	}
	if bindings[2].Target.Source != "member_definition" || bindings[2].Target.DefinitionIndex != 1 ||
		bindings[2].Target.Partition != "p1" || !bindings[2].Target.Binding.Default {
		t.Errorf("binding[2] = %+v, want member_definition idx1 p1 DEFAULT", bindings[2].Target)
	}
	// Schema filled from effective request schema on unqualified targets.
	if bindings[0].Target.Schema != "golden" || bindings[1].Target.Schema != "golden" {
		t.Errorf("schema not filled: %v / %v", bindings[0].Target.Schema, bindings[1].Target.Schema)
	}
	// p2 declares no binding — no record. Aggregate stays decided.
	if stmt.Metadata.Placement.Outcome != spec.PlacementApplied {
		t.Errorf("aggregate = %v, want applied", stmt.Metadata.Placement.Outcome)
	}
}

// R1-F: provider originals stay immutable — success, refusal and
// undecided paths never mutate the loaded shape or catalog.
func TestT20R4R1ProviderImmutability(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	for name, sql := range map[string]string{
		"applied":   "ALTER TABLE t ADD PARTITION (PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=p_ok);",
		"refused":   t20r4AddMissing,
		"undecided": "ALTER TABLE t REORGANIZE PARTITION p0 INTO (PARTITION p00 VALUES LESS THAN (50), PARTITION p01 VALUES LESS THAN (100));",
	} {
		fixture := t20r4Fixture()
		if name == "undecided" {
			// Genuinely undecidable: withhold the member directory facts
			// so T14 cannot prove the reorganize outcome.
			fixture.tables["t"].PartitionDefs = nil
			fixture.tables["t"].PartitionDescriptions = nil
		}
		orig := cloneTableSnapshot(fixture.tables["t"])
		origPlacement := append([]string(nil), fixture.placement["p_old"].BoundTargets...)
		if _, err := t20r4Audit(t, fixture, isolated, sql); err != nil {
			t.Fatalf("%s audit: %v", name, err)
		}
		if !reflect.DeepEqual(fixture.tables["t"], orig) {
			t.Errorf("%s mutated provider shape", name)
		}
		// Independent literal expectation: using the production clone
		// alone could hide a shallow-copy bug in this very map.
		if !reflect.DeepEqual(fixture.tables["t"].PartitionPlacements,
			map[string]string{"p0": "p_old", "p_keep": "p_old"}) {
			t.Errorf("%s mutated the provider's member binding map: %v", name, fixture.tables["t"].PartitionPlacements)
		}
		if !reflect.DeepEqual(fixture.placement["p_old"].BoundTargets, origPlacement) {
			t.Errorf("%s mutated provider bound set", name)
		}
		if name == "undecided" {
			_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, fixture)
			outcome := statements[0].Metadata.Placement.MutationOutcome
			if outcome == nil || *outcome != spec.PlacementUnproven {
				t.Fatal("immutability control did not execute an undecided transition")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// FINAL matrix — rework-final §6 A-L distinguishing cases.
// ---------------------------------------------------------------------------

// t20r4UnknownCatalogFixture returns the complete-seed + unknown-entry
// catalog: p_unknown is present with UNREAD bound set; a preceding
// DROP p_unknown leaves it unproven while p_ok stays present-proven-empty.
func t20r4UnknownCatalogFixture() *t20r4Provider {
	fixture := t20r4Fixture()
	fixture.placement["p_unknown"] = &spec.PlacementPolicyFact{BoundTargets: nil}
	return fixture
}

const t20r4DropUnknownPrefix = "DROP PLACEMENT POLICY p_unknown; "

// A: unknown aggregate cannot authorize a later DROP — matrix over
// CREATE/ADD/REORG/PARTITION BY (exact per-brief assertions).
func TestT20R4R2UnknownAggregateNoDropAuthorization(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	operations := map[string]string{
		"create": "CREATE TABLE t2 (id INT) PARTITION BY RANGE(id) " +
			"(PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=p_ok, " +
			"PARTITION p1 VALUES LESS THAN (200) PLACEMENT POLICY=p_unknown);",
		"add": "ALTER TABLE t ADD PARTITION " +
			"(PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=p_ok, " +
			"PARTITION p6 VALUES LESS THAN (600) PLACEMENT POLICY=p_unknown);",
		"reorg": "ALTER TABLE t REORGANIZE PARTITION p0 INTO " +
			"(PARTITION p00 VALUES LESS THAN (50) PLACEMENT POLICY=p_ok, " +
			"PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=p_unknown);",
		"partition_by": "ALTER TABLE t PARTITION BY RANGE(id) " +
			"(PARTITION q0 VALUES LESS THAN (100) PLACEMENT POLICY=p_ok, " +
			"PARTITION q1 VALUES LESS THAN (200) PLACEMENT POLICY=p_unknown);",
	}
	for name, op := range operations {
		t.Run(name, func(t *testing.T) {
			sql := t20r4DropUnknownPrefix + op +
				" DROP PLACEMENT POLICY p_ok; ALTER PLACEMENT POLICY p_ok FOLLOWERS=1;"
			result, err := t20r4Audit(t, t20r4UnknownCatalogFixture(), isolated, sql)
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 4 {
				t.Fatalf("statements = %d, want 4", len(result.Statements))
			}
			// stmt1: unknown member bind owes catalog gap, no warn/blocker.
			if finds := t20r4Findings(result, 1, t20r4PlacementWarn); len(finds) != 0 {
				t.Errorf("stmt1 warn = %v, want none while aggregate unknown", finds)
			}
			if finds := t20r4Findings(result, 1, t20r4PlacementExistsRequire); len(finds) != 0 {
				t.Errorf("stmt1 exists = %v, want none while aggregate unknown", finds)
			}
			if !t20r4HasGap(result, 1, t20r4PlacementWarn, "placement_policy.catalog") {
				t.Errorf("stmt1 warn must owe placement_policy.catalog")
			}
			if !t20r4HasGap(result, 1, t20r4PlacementExistsRequire, "placement_policy.catalog") {
				t.Errorf("stmt1 exists must owe placement_policy.catalog")
			}
			// stmt2: DROP p_ok — p_ok may have acquired member refs under the
			// undecided operation; bound set stays unvouched, so bound_targets
			// gap only, never a silent apply or a fabricated ref blocker.
			if finds := t20r4Findings(result, 2, t20r4PlacementWarn); len(finds) != 0 {
				t.Errorf("stmt2 drop warn = %v, want none while bound set unvouched", finds)
			}
			if finds := t20r4Findings(result, 2, t20r4PlacementExistsRequire); len(finds) != 0 {
				t.Errorf("stmt2 drop blocker = %v, want gap not fabricated refusal", finds)
			}
			if !t20r4HasGap(result, 2, t20r4PlacementExistsRequire, "placement_policy.bound_targets") {
				t.Errorf("stmt2 must owe placement_policy.bound_targets")
			}
			// The undecided DROP cannot prove either presence or absence.
			for _, id := range []string{t20r4PlacementExistsRequire, t20r4PlacementWarn} {
				if len(t20r4Findings(result, 3, id)) != 0 ||
					!t20r4HasGap(result, 3, id, "placement_policy.catalog") {
					t.Errorf("stmt3 must keep p_ok unknown under %s", id)
				}
			}
			fixture := t20r4UnknownCatalogFixture()
			original := cloneTableSnapshot(fixture.tables["t"])
			_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, fixture)
			beforeDrop := t20r4PolicyAt(t, statements, 2)
			if beforeDrop.Exists == nil || !*beforeDrop.Exists || beforeDrop.BoundTargets != nil {
				t.Fatalf("before DROP, p_ok exists but refs are unread: %+v", beforeDrop)
			}
			if t20r4PolicyAt(t, statements, 3).Exists != nil {
				t.Fatal("undecided DROP fabricated a definite existence answer")
			}
			if !reflect.DeepEqual(fixture.tables["t"], original) ||
				fixture.placement["p_ok"].BoundTargets == nil ||
				len(fixture.placement["p_ok"].BoundTargets) != 0 {
				t.Fatal("uncertainty mutated provider originals")
			}
		})
	}
}

// B: unknown then missing — refused binding publishes no destination.
func TestT20R4R2UnknownThenMissing(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	sql := t20r4DropUnknownPrefix +
		"CREATE TABLE t2 (id INT) PARTITION BY RANGE(id) " +
		"(PARTITION p0 VALUES LESS THAN (100) PLACEMENT POLICY=p_unknown, " +
		"PARTITION p1 VALUES LESS THAN (200) PLACEMENT POLICY=p_missing); " +
		"ALTER TABLE t2 ADD COLUMN observed INT;"
	result, err := t20r4Audit(t, t20r4UnknownCatalogFixture(), isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	finds := t20r4Findings(result, 1, t20r4PlacementExistsRequire)
	if len(finds) != 1 {
		t.Fatalf("exists findings = %d, want exactly one (p_missing)", len(finds))
	}
	if finds[0].Metadata["partition"] != "p1" || finds[0].Metadata["policy"] != "p_missing" {
		t.Errorf("finding identity = %v, want p_missing/p1", finds[0].Metadata)
	}
	if result.Verdict != report.VerdictReject {
		t.Errorf("verdict = %v, want reject", result.Verdict)
	}
	if len(t20r4Findings(result, 1, t20r4PlacementWarn)) != 0 ||
		t20r4HasGap(result, 1, "", "placement_policy.topology") {
		t.Fatal("proven refusal must suppress binding warnings and topology gaps")
	}
	_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, t20r4UnknownCatalogFixture())
	if snapshot := t20r4SnapshotAt(t, statements, 2); snapshot == nil || snapshot.Exists {
		t.Fatalf("refused CREATE must preserve proven absence: %+v", snapshot)
	}
}

// C: unknown target retirement — nil snapshot target still revokes the
// possibly-retired p0 ref so DROP p_old gaps rather than stale-blocking.
func TestT20R4R2UnknownTargetRetirement(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	fixture := t20r4Fixture()
	fixture.tables["t"] = nil // unknown target, not proven absent
	fixture.placement["p_old"].BoundTargets = []string{"partition:golden.t.p0"}
	sql := "ALTER TABLE t REORGANIZE PARTITION p0 INTO " +
		"(PARTITION p00 VALUES LESS THAN (50) PLACEMENT POLICY=p_ok, " +
		"PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=DEFAULT); " +
		"DROP PLACEMENT POLICY p_old;"
	result, err := t20r4Audit(t, fixture, isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if finds := t20r4Findings(result, 0, t20r4PlacementWarn); len(finds) != 0 {
		t.Errorf("stmt0 warn = %v, want none on unknown target", finds)
	}
	if finds := t20r4Findings(result, 1, t20r4PlacementExistsRequire); len(finds) != 0 {
		t.Errorf("stmt1 drop blocker = %v, want bound_targets gap", finds)
	}
	if !t20r4HasGap(result, 1, t20r4PlacementExistsRequire, "placement_policy.bound_targets") {
		t.Errorf("stmt1 DROP p_old must owe bound_targets gap")
	}
	if !t20r4HasGap(result, 0, t20r4PlacementWarn, "target_table.existence") {
		t.Fatal("unknown target must carry its own missing fact")
	}
	_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, fixture)
	if t20r4PolicyAt(t, statements, 1).BoundTargets != nil {
		t.Fatal("possibly retired source must not remain positive DROP evidence")
	}
}

// D: known directory + unread binding map — DROP PARTITION p0 retires the
// p0 ref via directory names (not the nil binding map), so DROP p_old
// succeeds prospectively.
func TestT20R4R2UnreadBindingMapDrop(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	fixture := t20r4Fixture()
	fixture.tables["t"].PartitionPlacements = nil
	fixture.placement["p_old"].BoundTargets = []string{"partition:golden.t.p0"}
	sql := "ALTER TABLE t DROP PARTITION p0; DROP PLACEMENT POLICY p_old; ALTER TABLE t ADD COLUMN observed INT;"
	result, err := t20r4Audit(t, fixture, isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if finds := t20r4Findings(result, 1, t20r4PlacementExistsRequire); len(finds) != 0 {
		t.Errorf("DROP p_old blocker = %v, want applied — p0 ref retired by directory", finds)
	}
	t20r4AssertAppliedPolicyDrop(t, result, 1)
	_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, fixture)
	bound := t20r4PolicyAt(t, statements, 1).BoundTargets
	if bound == nil || len(*bound) != 0 {
		t.Fatalf("directory retirement must prove empty refs: %v", bound)
	}
	if snapshot := t20r4SnapshotAt(t, statements, 2); snapshot == nil ||
		!reflect.DeepEqual(snapshot.PartitionNames, []string{"p_keep"}) {
		t.Fatalf("surviving directory is wrong: %+v", snapshot)
	}
}

// E: T14 unknown static vs literal new defs — only target_table.partition_defs
// is owed; statement defs were provided and comparable.
func TestT20R4R2T14UnknownStaticFacts(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	fixture := t20r4Fixture()
	fixture.tables["t"].PartitionDefs = nil
	fixture.tables["t"].PartitionDescriptions = nil
	sql := "ALTER TABLE t ADD PARTITION (PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=p_ok); " +
		"DROP PLACEMENT POLICY p_ok; ALTER TABLE t ADD COLUMN observed INT;"
	result, err := t20r4Audit(t, fixture, isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if finds := t20r4Findings(result, 0, t20r4PlacementWarn); len(finds) != 0 {
		t.Errorf("stmt0 warn = %v, want none while member facts unproven", finds)
	}
	got := map[string]bool{}
	for _, g := range result.Statements[0].EvidenceGaps {
		if g.RuleID != t20r4PlacementWarn {
			continue
		}
		for _, f := range g.RequiredFacts {
			got[f] = true
		}
	}
	if len(got) != 1 || !got["target_table.partition_defs"] {
		t.Errorf("warn RequiredFacts = %v, want exactly [target_table.partition_defs]", got)
	}
	if !t20r4HasGap(result, 1, t20r4PlacementExistsRequire, "placement_policy.bound_targets") ||
		len(t20r4Findings(result, 1, t20r4PlacementExistsRequire)) != 0 {
		t.Fatal("undecided ADD must not authorize or falsely refuse policy DROP")
	}
	_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, fixture)
	snapshot := t20r4SnapshotAt(t, statements, 2)
	if snapshot == nil || snapshot.PartitionNames != nil ||
		!reflect.DeepEqual(snapshot.PartitionPlacements, map[string]string{"p0": "p_old", "p_keep": "p_old"}) {
		t.Fatalf("undecided ADD must keep old overrides without publishing p5: %+v", snapshot)
	}
	defaultSQL := strings.ReplaceAll(sql, "PLACEMENT POLICY=p_ok", "PLACEMENT POLICY=DEFAULT")
	defaultReport, err := t20r4Audit(t, fixture, isolated, defaultSQL)
	if err != nil {
		t.Fatal(err)
	}
	if t20r4HasPlacementGap(defaultReport, 0) || len(t20r4Findings(defaultReport, 0, t20r4PlacementWarn)) != 0 {
		t.Fatal("DEFAULT-only ADD owes no placement premise")
	}
	_, defaults := t17Enrich(t, spec.DialectTiDB, defaultSQL, "golden", t20v805, fixture)
	if next := t20r4SnapshotAt(t, defaults, 2); next == nil || next.PartitionNames != nil ||
		!reflect.DeepEqual(next.PartitionPlacements, snapshot.PartitionPlacements) {
		t.Fatalf("DEFAULT must not fabricate an undecided directory: %+v", next)
	}
}

// F: lifecycle unknown under warn-off isolated profile — CREATE and ALTER
// POLICY each owe exists.require catalog gap only.
func TestT20R4R2LifecycleUnknownWarnOff(t *testing.T) {
	_, warnOff := t20r4Policies(t)
	fixture := t20r4Fixture()
	fixture.placement = nil
	for name, sql := range map[string]string{
		"create": "CREATE PLACEMENT POLICY p_new FOLLOWERS=1;",
		"alter":  "ALTER PLACEMENT POLICY p_new FOLLOWERS=1;",
	} {
		result, err := t20r4Audit(t, fixture, warnOff, sql)
		if err != nil {
			t.Fatalf("%s audit: %v", name, err)
		}
		if finds := t20r4Findings(result, 0, t20r4PlacementExistsRequire); len(finds) != 0 {
			t.Errorf("%s exists finding = %v, want none under unvouched catalog", name, finds)
		}
		if !t20r4HasGap(result, 0, t20r4PlacementExistsRequire, "placement_policy.catalog") {
			t.Errorf("%s must owe exists.require catalog gap", name)
		}
		if result.Statements[0].Coverage.Status != report.CoverageUnverified {
			t.Errorf("%s must remain unverified", name)
		}
	}
}

// H: RANGE positive-subset — DEFAULT revokes the stale global ref so the
// old-policy DROP lands without a stale bound blocker.
func TestT20R4R2RangeDefaultRevokesRef(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	fixture := t20r4Fixture()
	fixture.placement["p_old"].BoundTargets = []string{"range:global"}
	result, err := t20r4Audit(t, fixture, isolated,
		"ALTER RANGE global PLACEMENT POLICY=DEFAULT; DROP PLACEMENT POLICY p_old;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if finds := t20r4Findings(result, 1, t20r4PlacementExistsRequire); len(finds) != 0 {
		t.Errorf("DROP p_old blocker = %v, want applied — global ref revoked", finds)
	}
	t20r4AssertAppliedPolicyDrop(t, result, 1)
}

// J-repair: QualifiedMemberRefs uses the SAME member name p5 under both
// schemas — distinct ref identities.
func TestT20R4R2QualifiedMemberSameName(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	sql := "ALTER TABLE other.t ADD PARTITION (PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=p_ok); " +
		"ALTER TABLE golden.t ADD PARTITION (PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=p_ok); " +
		"DROP PLACEMENT POLICY p_ok;"
	fixture := t20r4Fixture()
	fixture.tables["t"].PartitionPlacements = map[string]string{"p0": "", "p_keep": ""}
	fixture.placement["p_old"].BoundTargets = []string{}
	result, err := t20r4Audit(t, &t20r4QualifiedProvider{fixture}, isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	finds := t20r4Findings(result, 2, t20r4PlacementExistsRequire)
	if len(finds) == 0 {
		t.Fatalf("DROP p_ok must refuse — proven refs under both schemas")
	}
	refs, _ := finds[0].Metadata["bound_targets"].([]string)
	var goldenRef, otherRef bool
	for _, ref := range refs {
		if ref == "partition:golden.t.p5" {
			goldenRef = true
		}
		if ref == "partition:other.t.p5" {
			otherRef = true
		}
	}
	if !goldenRef || !otherRef {
		t.Errorf("bound_targets = %v, want same member name under both schemas", refs)
	}
}

// G: native-validated PARTITION BY member bindings — scheme replaces the
// member set atomically; old refs retire, new binds land.
func TestT20R4R2PartitionBySchemeBindings(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	sql := "ALTER TABLE t PARTITION BY RANGE(id) " +
		"(PARTITION q0 VALUES LESS THAN (50) PLACEMENT POLICY=p_ok, " +
		"PARTITION q1 VALUES LESS THAN (200) PLACEMENT POLICY=DEFAULT); " +
		"DROP PLACEMENT POLICY p_old; DROP PLACEMENT POLICY p_ok; ALTER TABLE t ADD COLUMN observed INT;"
	result, err := t20r4Audit(t, t20r4Fixture(), isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	// One warn for the applied q0 bind (DEFAULT q1 stays silent).
	warns := t20r4Findings(result, 0, t20r4PlacementWarn)
	if len(warns) != 1 || warns[0].Metadata["partition"] != "q0" {
		t.Errorf("warn findings = %v, want exactly one for q0", warns)
	}
	if !t20r4HasGap(result, 0, t20r4PlacementWarn, "placement_policy.topology") {
		t.Errorf("applied bind must owe the topology gap")
	}
	// p_old's refs retired with the scheme — its DROP lands clean.
	if finds := t20r4Findings(result, 1, t20r4PlacementExistsRequire); len(finds) != 0 {
		t.Errorf("DROP p_old blocker = %v, want applied — scheme retired old refs", finds)
	}
	// p_ok refuses with exactly the new q0 ref.
	finds := t20r4Findings(result, 2, t20r4PlacementExistsRequire)
	if len(finds) != 1 {
		t.Fatalf("DROP p_ok findings = %d, want 1", len(finds))
	}
	refs, _ := finds[0].Metadata["bound_targets"].([]string)
	if len(refs) != 1 || refs[0] != "partition:golden.t.q0" {
		t.Errorf("bound_targets = %v, want exactly [partition:golden.t.q0]", refs)
	}
	t20r4AssertAppliedPolicyDrop(t, result, 1)
	_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, t20r4Fixture())
	snapshot := t20r4SnapshotAt(t, statements, 3)
	if snapshot == nil || !reflect.DeepEqual(snapshot.PartitionNames, []string{"q0", "q1"}) ||
		!reflect.DeepEqual(snapshot.PartitionPlacements, map[string]string{"q0": "p_ok", "q1": ""}) {
		t.Fatalf("scheme replacement must replace member facts: %+v", snapshot)
	}
	refusedSQL := "ALTER TABLE t PARTITION BY RANGE(id) " +
		"(PARTITION q0 VALUES LESS THAN (50) PLACEMENT POLICY=p_ok, " +
		"PARTITION q1 VALUES LESS THAN (200) PLACEMENT POLICY=p_missing); " +
		"ALTER TABLE t ADD COLUMN observed INT;"
	refused, err := t20r4Audit(t, t20r4Fixture(), isolated, refusedSQL)
	if err != nil {
		t.Fatal(err)
	}
	if len(t20r4Findings(refused, 0, t20r4PlacementExistsRequire)) != 1 ||
		len(t20r4Findings(refused, 0, t20r4PlacementWarn)) != 0 {
		t.Fatal("missing scheme-member policy must refuse atomically")
	}
	provider := t20r4Fixture()
	original := cloneTableSnapshot(provider.tables["t"])
	_, before := t17Enrich(t, spec.DialectTiDB, refusedSQL, "golden", t20v805, provider)
	if !reflect.DeepEqual(t20r4SnapshotAt(t, before, 1), original) {
		t.Fatal("refused scheme replacement changed the prior snapshot")
	}
}

// H2: range rebind through an unknown intermediate policy keeps the
// unvouched bound set honest — no stale blocker on the final DROP.
func TestT20R4R2RangeRebindUnknown(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	fixture := t20r4Fixture()
	fixture.placement["p_old"].BoundTargets = []string{"range:global"}
	fixture.placement["p_unknown"] = &spec.PlacementPolicyFact{BoundTargets: nil}
	sql := "ALTER RANGE global PLACEMENT POLICY=p_old; " +
		"DROP PLACEMENT POLICY p_unknown; " +
		"ALTER RANGE global PLACEMENT POLICY=p_unknown; " +
		"DROP PLACEMENT POLICY p_old;"
	result, err := t20r4Audit(t, fixture, isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	// Final DROP p_old: range may still bind it or may have moved to
	// p_unknown — the bound set stays unvouched, a gap not a stale
	// blocker listing the old proven ref.
	finds := t20r4Findings(result, 3, t20r4PlacementExistsRequire)
	if len(finds) != 0 {
		t.Errorf("final DROP blocker = %v, want bound_targets gap", finds)
	}
	if !t20r4HasGap(result, 3, t20r4PlacementExistsRequire, "placement_policy.bound_targets") {
		t.Errorf("final DROP must owe bound_targets gap")
	}
	for _, finding := range t20r4Findings(result, 0, t20r4PlacementWarn) {
		for _, key := range []string{"schema", "table", "partition"} {
			if _, exists := finding.Metadata[key]; exists {
				t.Fatalf("RANGE acquired a schema-owned identity: %v", finding.Metadata)
			}
		}
	}
}

// I: positive-subset rename chain — proven ref survives rename under a
// non-exhaustive set; first DROP refuses with the renamed identity.
func TestT20R4R2PositiveSubsetRenameChain(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	fixture := t20r4Fixture()
	fixture.placement = nil
	sql := "CREATE OR REPLACE PLACEMENT POLICY p_ok FOLLOWERS=1; " +
		"ALTER TABLE t ADD PARTITION (PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=p_ok); " +
		"RENAME TABLE t TO r; " +
		"DROP PLACEMENT POLICY p_ok; " +
		"ALTER TABLE r PARTITION p5 PLACEMENT POLICY=DEFAULT; " +
		"DROP PLACEMENT POLICY p_ok;"
	result, err := t20r4Audit(t, fixture, isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	finds := t20r4Findings(result, 3, t20r4PlacementExistsRequire)
	if len(finds) == 0 {
		t.Fatalf("first DROP must refuse — proven p5 ref survives rename")
	}
	refs, _ := finds[0].Metadata["bound_targets"].([]string)
	if len(refs) != 1 || refs[0] != "partition:golden.r.p5" {
		t.Errorf("bound_targets = %v, want [partition:golden.r.p5]", refs)
	}
	// Other original refs were never enumerated: DEFAULT cannot prove emptiness.
	if finds := t20r4Findings(result, 5, t20r4PlacementExistsRequire); len(finds) != 0 {
		t.Errorf("final DROP blocker = %v, want bound_targets gap", finds)
	}
	if !t20r4HasGap(result, 5, t20r4PlacementExistsRequire, "placement_policy.bound_targets") ||
		len(t20r4Findings(result, 5, t20r4PlacementWarn)) != 0 {
		t.Fatal("DEFAULT laundered an incomplete set into proven-empty")
	}
	_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, fixture)
	source := statements[2].Metadata.TargetTables[0].Snapshot
	if source == nil || source.Table == nil || source.Table.Name != "t" ||
		source.PartitionPlacements["p5"] != "p_ok" {
		t.Fatalf("later rename/unbind mutated the preceding projection: %+v", source)
	}
}

// J: statement source location and finding identity on a multiline input.
func TestT20R4R2FindingLocationAndIdentity(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	// Leading newline puts the CREATE on line 2, column 3.
	sql := "\n  " + t20r4CreateLate
	result, err := t20r4Audit(t, t20r4Fixture(), isolated, sql)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	finds := t20r4Findings(result, 0, t20r4PlacementExistsRequire)
	if len(finds) == 0 {
		t.Fatalf("missing finding expected")
	}
	loc := finds[0].Location
	if loc == nil {
		t.Fatalf("finding must carry source location")
	}
	if loc.Line != 2 || loc.Column != 3 {
		t.Errorf("location = line %d col %d, want 2/3", loc.Line, loc.Column)
	}
	if finds[0].Metadata["schema"] != "golden" || finds[0].Metadata["table"] != "t2" ||
		finds[0].Metadata["partition"] != "p1" {
		t.Fatalf("statement location lost member identity: %v", finds[0].Metadata)
	}
}

// J: collector matrix over every structured source — ALTER DATABASE,
// direct table bind, named partition bind, ADD defs, REORG defs,
// PARTITION BY scheme, RANGE — including DEFAULT retention.
func TestT20R4R2CollectorSourceMatrix(t *testing.T) {
	sql := "ALTER DATABASE golden PLACEMENT POLICY=p_ok; " +
		"ALTER TABLE t PLACEMENT POLICY=p_second; " +
		"ALTER TABLE t PARTITION p0 PLACEMENT POLICY=p_ok; " +
		"ALTER TABLE t ADD PARTITION (PARTITION p5 VALUES LESS THAN (500) PLACEMENT POLICY=p_second, PARTITION p6 VALUES LESS THAN (600) PLACEMENT POLICY=DEFAULT); " +
		"ALTER TABLE t REORGANIZE PARTITION p0 INTO (PARTITION p00 VALUES LESS THAN (50) PLACEMENT POLICY=p_ok, PARTITION p01 VALUES LESS THAN (100) PLACEMENT POLICY=p_second); " +
		"ALTER TABLE t PARTITION BY RANGE(id) (PARTITION q0 VALUES LESS THAN (100) PLACEMENT POLICY=p_ok); " +
		"ALTER RANGE global PLACEMENT POLICY=p_second;"
	_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, t20r4Fixture())
	if len(statements) != 7 {
		t.Fatalf("statements = %d, want 7", len(statements))
	}
	check := func(stmt int, want []struct{ source, kind, partition, policy string }) {
		if statements[stmt].Metadata == nil || statements[stmt].Metadata.Placement == nil {
			t.Fatalf("stmt %d has no placement facts", stmt)
		}
		got := statements[stmt].Metadata.Placement.Bindings
		if len(got) != len(want) {
			t.Fatalf("stmt %d bindings = %d, want %d: %+v", stmt, len(got), len(want), got)
		}
		for i, w := range want {
			tg := got[i].Target
			if tg.Source != w.source || tg.TargetKind != w.kind ||
				tg.Partition != w.partition || tg.Binding.Policy != w.policy {
				t.Errorf("stmt %d binding[%d] = %+v, want %s/%s/%s/%s", stmt, i, tg, w.source, w.kind, w.partition, w.policy)
			}
			actionIndex, definitionIndex := -1, -1
			if stmt >= 1 && stmt <= 5 {
				actionIndex = 0
			}
			if w.source != "direct" {
				definitionIndex = i
			}
			if tg.ActionIndex != actionIndex || tg.DefinitionIndex != definitionIndex {
				t.Errorf("stmt %d binding[%d] has wrong indices: %+v", stmt, i, tg)
			}
			if tg.Binding.Default != (stmt == 3 && i == 1) {
				t.Errorf("stmt %d binding[%d] lost DEFAULT identity", stmt, i)
			}
			if stmt == 6 && tg.Schema != "" {
				t.Fatal("global range must not acquire a schema")
			}
		}
	}
	check(0, []struct{ source, kind, partition, policy string }{{"direct", "schema", "", "p_ok"}})
	check(1, []struct{ source, kind, partition, policy string }{{"direct", "table", "", "p_second"}})
	check(2, []struct{ source, kind, partition, policy string }{{"direct", "partition", "p0", "p_ok"}})
	check(3, []struct{ source, kind, partition, policy string }{
		{"member_definition", "partition", "p5", "p_second"},
		{"member_definition", "partition", "p6", ""}, // DEFAULT retained
	})
	check(4, []struct{ source, kind, partition, policy string }{
		{"member_definition", "partition", "p00", "p_ok"},
		{"member_definition", "partition", "p01", "p_second"},
	})
	check(5, []struct{ source, kind, partition, policy string }{{"scheme_definition", "partition", "q0", "p_ok"}})
	check(6, []struct{ source, kind, partition, policy string }{{"direct", "range", "", "p_second"}})
}

// K: subpartition companion — normalized copy restores primary member
// names and table binding while the original markers stay.
func TestT20R4R2SubpartitionCompanion(t *testing.T) {
	sql := "CREATE TABLE t_sub (id INT) PLACEMENT POLICY=p_ok " +
		"PARTITION BY RANGE(id) SUBPARTITION BY HASH(id) (" +
		"PARTITION p0 VALUES LESS THAN (100) (" +
		"SUBPARTITION s0 PLACEMENT POLICY=p_ok, " +
		"SUBPARTITION s1 PLACEMENT POLICY=DEFAULT)); " +
		"ALTER TABLE t_sub ADD COLUMN observed INT;"
	for _, patch := range []int{0, 1} {
		t.Run(fmt.Sprintf("8.5.%d", patch), func(t *testing.T) {
			version := *t20v805
			version.Product = spec.VersionProductTiDB
			version.Patch = patch
			fixture := t20r4Fixture()
			fixture.version = fmt.Sprintf("8.0.11-TiDB-v8.5.%d", patch)
			_, statements := t17Enrich(t, spec.DialectTiDB, sql, "golden", &version, fixture)
			if !statements[0].DDL.Subpartitioned || statements[0].DDL.PartitionNames != nil {
				t.Fatal("normalization mutated the authored declaration")
			}
			snapshot := t20r4SnapshotAt(t, statements, 1)
			if patch == 0 {
				if snapshot == nil || !reflect.DeepEqual(snapshot.PartitionNames, []string{"p0"}) ||
					snapshot.PlacementPolicy != "p_ok" || !snapshot.PlacementPolicyKnown ||
					!reflect.DeepEqual(snapshot.PartitionPlacements, map[string]string{"p0": ""}) {
					t.Fatalf("fixed native anchor must publish only the primary axis: %+v", snapshot)
				}
				policy := statements[1].Metadata.PlacementPolicies["p_ok"]
				if policy == nil || !reflect.DeepEqual(policy.BoundTargets, []string{"table:golden.t_sub"}) {
					t.Fatalf("ignored children must create no refs: %+v", policy)
				}
			} else if snapshot != nil && snapshot.PartitionNames != nil {
				t.Fatalf("unverified patch version inherited 8.5.0 normalization: %+v", snapshot)
			}
		})
	}
}

// K: MySQL control — same subpartition SQL produces no TiDB aspect,
// no TiDB rule finding, no normalization.
func TestT20R4R2SubpartitionMySQLControl(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	sql := "CREATE TABLE t_sub (id INT) " +
		"PARTITION BY RANGE(id) SUBPARTITION BY HASH(id) (" +
		"PARTITION p0 VALUES LESS THAN (100) (SUBPARTITION s0, SUBPARTITION s1));"
	mysqlFixture := &t20r4Provider{&t10Provider{version: "8.4.10",
		tables: map[string]*spec.TableSnapshot{"t": t20r4RangeShape()}}}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          spec.DialectMySQL,
		Schema:           "golden",
		ConfigPath:       isolated,
		MetadataProvider: mysqlFixture,
	})
	if err != nil && !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("audit: %v", err)
	}
	for _, u := range result.Unsupported {
		if u.Feature == "tidb_subpartition" {
			t.Errorf("MySQL must not emit tidb_subpartition aspect")
		}
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t20r4PlacementWarn || f.RuleID == t20r4PlacementExistsRequire {
			t.Errorf("MySQL must not produce TiDB placement findings: %v", f)
		}
	}
	version := &spec.VersionIdentity{Product: spec.VersionProductMySQL, Major: 8, Minor: 4, Patch: 10}
	_, statements := t17Enrich(t, spec.DialectMySQL,
		sql+" ALTER TABLE t_sub ADD COLUMN observed INT;", "golden", version, mysqlFixture)
	if !statements[0].DDL.Subpartitioned {
		t.Fatal("MySQL declaration marker was cleared")
	}
	if snapshot := t20r4SnapshotAt(t, statements, 1); snapshot != nil && snapshot.PartitionNames != nil {
		t.Fatalf("MySQL received TiDB's ignored-axis normalization: %+v", snapshot)
	}
}

// L: provider catalog errors propagate under every profile including
// all-rules-off — state loading is never disarmed by policy switches.
func TestT20R4R2ProviderErrorSentinels(t *testing.T) {
	isolated, warnOff := t20r4Policies(t)
	allOff := t05PolicyPathConfigured(t, map[string]t05RuleConfig{})
	sentinels := map[string]error{
		"errors":   errors.New("simulated catalog failure"),
		"canceled": context.Canceled,
		"deadline": context.DeadlineExceeded,
	}
	for name, sentinel := range sentinels {
		for _, sql := range []string{t20r4AddMissing, t20r4ReorgMissing} {
			for profileName, profile := range map[string]string{"both": isolated, "warn_off": warnOff, "all_off": allOff} {
				fixture := t20r4Fixture()
				fixture.placementErr = sentinel
				_, err := t20r4Audit(t, fixture, profile, sql)
				if !errors.Is(err, sentinel) {
					t.Errorf("%s %s err = %v, want errors.Is(%v)", name, profileName, err, sentinel)
				}
			}
		}
	}
}

func t20r4SnapshotAt(t *testing.T, statements []spec.Statement, index int) *spec.TableSnapshot {
	t.Helper()
	if index >= len(statements) || statements[index].Metadata == nil ||
		len(statements[index].Metadata.TargetTables) != 1 {
		t.Fatalf("statement %d needs one bound table", index)
	}
	return statements[index].Metadata.TargetTables[0].Snapshot
}

// Severity affects judgment, not the preceding policy-independent enrichment.
// The all-off public loader test above also ensures enrichment is not skipped.
func TestT20R4R2ConfiguredLevelsKeepRefusedState(t *testing.T) {
	profiles := map[string]string{
		"warning": t05PolicyPathConfigured(t, map[string]t05RuleConfig{
			t20r4PlacementWarn:          {level: "blocker"},
			t20r4PlacementExistsRequire: {level: "warning"},
		}),
		"all_off": t05PolicyPathConfigured(t, map[string]t05RuleConfig{}),
	}
	for profileName, profile := range profiles {
		for operation, sql := range map[string]string{"add": t20r4AddMissing, "reorg": t20r4ReorgMissing} {
			t.Run(profileName+"/"+operation, func(t *testing.T) {
				fixture := t20r4Fixture()
				original := cloneTableSnapshot(fixture.tables["t"])
				result, err := t20r4Audit(t, fixture, profile, sql)
				if err != nil {
					t.Fatal(err)
				}
				if len(t20r4Findings(result, 0, t20r4PlacementWarn)) != 0 {
					t.Fatal("severity override authorized a refused binding")
				}
				findings := t20r4Findings(result, 0, t20r4PlacementExistsRequire)
				if profileName == "warning" {
					if len(findings) != 1 || findings[0].Level != "warning" {
						t.Fatalf("configured severity not honored: %v", findings)
					}
				} else if len(result.Statements[0].Findings) != 0 ||
					len(result.Statements[0].EvidenceGaps) != 0 {
					t.Fatal("disabled rules emitted a judgment or gap")
				}
				// The same real enrichment seam used by AuditSQL has no
				// policy input. Inspect its successor separately.
				_, statements := t17Enrich(t, spec.DialectTiDB,
					sql+" ALTER TABLE t ADD COLUMN observed INT;", "golden", t20v805, fixture)
				if !reflect.DeepEqual(t20r4SnapshotAt(t, statements, 1), original) {
					t.Fatal("refused member operation changed its successor")
				}
			})
		}
	}
}

// Each one-element bound list is modeled, but T14 cannot compare the old
// and new string-valued DATE bounds. The missing comparison must remain
// visible under an isolated placement profile.
func TestT20R4R2CrossBoundComparisonGap(t *testing.T) {
	isolated, _ := t20r4Policies(t)
	fixture := t20r4Fixture()
	shape := fixture.tables["t"]
	method := "range columns"
	shape.PartitionType = &method
	shape.Columns = []spec.Column{{Name: "id", Type: "date"}}
	shape.PartitionNames = []string{"p0"}
	shape.PartitionDescriptions = []string{"'2023-01-01'"}
	shape.PartitionDefs = spec.CatalogPartitionDefs(shape.PartitionNames, shape.PartitionDescriptions, method)
	shape.PartitionPlacements = map[string]string{"p0": "p_old"}
	fixture.placement["p_old"].BoundTargets = []string{"partition:golden.t.p0"}
	sql := "ALTER TABLE t ADD PARTITION " +
		"(PARTITION p1 VALUES LESS THAN ('2024-01-01') PLACEMENT POLICY=p_ok);"
	result, err := t20r4Audit(t, fixture, isolated, sql)
	if err != nil {
		t.Fatal(err)
	}
	if len(t20r4Findings(result, 0, t20r4PlacementWarn)) != 0 {
		t.Fatal("undecidable cross-bound comparison published an applied warning")
	}
	for _, fact := range []string{"target_table.partition_defs", "statement.partition_defs"} {
		if !t20r4HasGap(result, 0, t20r4PlacementWarn, fact) {
			t.Errorf("missing cross-bound comparison fact %s", fact)
		}
	}
	if result.Statements[0].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("undecidable cross-bound comparison claimed %s coverage", result.Statements[0].Coverage.Status)
	}
}

func t20r4PolicyAt(t *testing.T, statements []spec.Statement, index int) *spec.PlacementPolicyPremise {
	t.Helper()
	if index >= len(statements) || statements[index].Metadata == nil ||
		statements[index].Metadata.Placement == nil || statements[index].Metadata.Placement.Policy == nil {
		t.Fatalf("statement %d needs a policy premise", index)
	}
	return statements[index].Metadata.Placement.Policy
}

func t20r4AssertAppliedPolicyDrop(t *testing.T, result report.Result, index int) {
	t.Helper()
	if len(t20r4Findings(result, index, t20r4PlacementExistsRequire)) != 0 ||
		len(t20r4Findings(result, index, t20r4PlacementWarn)) != 1 ||
		t20r4HasPlacementGap(result, index) {
		t.Fatalf("statement %d must be a proven unbound DROP: %+v", index, result.Statements[index])
	}
}

// Same table shape in two real schema identities, without aliasing provider data.
type t20r4QualifiedProvider struct{ *t20r4Provider }

func (p *t20r4QualifiedProvider) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	snapshot, err := p.t20r4Provider.LoadTableSnapshot(ctx, dialect, schema, table)
	if err != nil || snapshot == nil {
		return snapshot, err
	}
	owned := cloneTableSnapshot(snapshot)
	owned.Schema = schema
	if owned.Table != nil {
		owned.Table.Schema = schema
	}
	return owned, nil
}
