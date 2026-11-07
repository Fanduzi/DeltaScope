package audit

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t21ValidRule    = "ddl.tidb.sequence.valid"
	t21RestartRule  = "ddl.tidb.sequence.restart.warn"
	t21CycleRule    = "ddl.tidb.sequence.cycle.warn"
	t21RefsRule     = "ddl.tidb.sequence.references.warn"
	t21RuntimeGap   = "unknown_sequence_runtime_state"
	t21RuntimeFact  = "sequence.runtime_state"
	t21SequenceType = "sequence"
)

type t21SequenceFact struct {
	name              string
	status            spec.MetadataStatus
	exists            bool
	definition        *spec.SequenceDefinition
	references        []spec.SequenceRefEdge
	referencesUnknown bool
	conflictKind      string
	err               error
}

type t21Provider struct {
	version    string
	tables     map[string]*spec.TableSnapshot
	objects    map[string]t21SequenceFact
	calls      []spec.ObjectLookupRequest
	tableCalls []string
}

func (p *t21Provider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: p.version}, nil
}

func (p *t21Provider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	p.tableCalls = append(p.tableCalls, schema+"."+table)
	if snapshot, ok := p.tables[strings.ToLower(schema)+"."+strings.ToLower(table)]; ok {
		return snapshot, nil
	}
	return &spec.TableSnapshot{Exists: false, Table: &spec.Table{Schema: schema, Name: table}}, nil
}

func (p *t21Provider) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.calls = append(p.calls, req)
	if req.Type == "schema" {
		return &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Exists: true, Status: spec.MetadataStatusConfirmed}, nil
	}
	if req.Type != t21SequenceType {
		return &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
	}
	fact, ok := p.objects[strings.ToLower(req.Schema)+"."+strings.ToLower(req.Name)]
	if !ok {
		return &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
	}
	if fact.err != nil {
		return nil, fact.err
	}
	objectType := req.Type
	if fact.conflictKind != "" {
		objectType = fact.conflictKind
	}
	return &spec.ObjectSnapshot{
		Schema:                    req.Schema,
		Type:                      objectType,
		Name:                      req.Name,
		Status:                    fact.status,
		Exists:                    fact.exists,
		Sequence:                  fact.definition,
		SequenceReferences:        fact.references,
		SequenceReferencesUnknown: fact.referencesUnknown,
	}, nil
}

func t21Definition(start, increment, minValue, maxValue, cacheValue int64, cache, cycle bool) *spec.SequenceDefinition {
	return &spec.SequenceDefinition{
		Start:      &start,
		Increment:  &increment,
		MinValue:   &minValue,
		MaxValue:   &maxValue,
		CacheValue: &cacheValue,
		Cache:      &cache,
		Cycle:      &cycle,
	}
}

func t21Fixture() *t21Provider {
	return &t21Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables:  map[string]*spec.TableSnapshot{},
		objects: map[string]t21SequenceFact{
			"t21.seq": {
				name:       "seq",
				status:     spec.MetadataStatusConfirmed,
				exists:     true,
				definition: t21Definition(1, 1, 1, 9223372036854775806, 1000, true, false),
			},
			"t21.missing": {
				name:   "missing",
				status: spec.MetadataStatusNotFound,
			},
			"t21.seq_ref": {
				name:       "seq_ref",
				status:     spec.MetadataStatusConfirmed,
				exists:     true,
				definition: t21Definition(1, 1, 1, 9223372036854775806, 1000, true, false),
				references: []spec.SequenceRefEdge{{
					OwnerKind: "table", OwnerSchema: "t21", OwnerName: "t", Slot: "default:c",
					TargetSchema: "t21", TargetName: "seq_ref",
				}},
			},
			"t21.seq_unref": {
				name:       "seq_unref",
				status:     spec.MetadataStatusConfirmed,
				exists:     true,
				definition: t21Definition(1, 1, 1, 9223372036854775806, 1000, true, false),
				references: []spec.SequenceRefEdge{},
			},
			"t21.cycling": {
				name:       "cycling",
				status:     spec.MetadataStatusConfirmed,
				exists:     true,
				definition: t21Definition(1, 1, 1, 9223372036854775806, 1000, true, true),
			},
		},
	}
}

func t21Policy(t *testing.T, ids ...string) string {
	t.Helper()
	overrides := make(map[string]policy.RulePolicy, len(ids))
	for _, id := range ids {
		level := "warning"
		if id == t21ValidRule {
			level = "blocker"
		}
		overrides[id] = policy.RulePolicy{Enabled: true, Level: rule.Level(level)}
	}
	return writeIsolatedPolicy(t, overrides)
}

func t21Audit(t *testing.T, sql string, provider MetadataProvider, policyPath string) (report.Result, error) {
	t.Helper()
	return AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          spec.DialectTiDB,
		Schema:           "t21",
		TargetVersion:    "8.5.0",
		ConfigPath:       policyPath,
		MetadataProvider: provider,
	})
}

func t21Findings(result report.Result, stmt int, id string) []rule.Finding {
	var out []rule.Finding
	for _, finding := range result.Statements[stmt].Findings {
		if finding.RuleID == id {
			out = append(out, finding)
		}
	}
	return out
}

func t21Gaps(result report.Result, stmt int, id string) []rule.EvidenceGap {
	var out []rule.EvidenceGap
	for _, gap := range result.Statements[stmt].EvidenceGaps {
		if id == "" || gap.RuleID == id {
			out = append(out, gap)
		}
	}
	return out
}

func t21AssertOnlyRuntimeGap(t *testing.T, result report.Result, stmt int, ruleID string) {
	t.Helper()
	gaps := t21Gaps(result, stmt, ruleID)
	if len(gaps) != 1 {
		t.Fatalf("%s gaps = %#v, want exactly one", ruleID, result.Statements[stmt].EvidenceGaps)
	}
	if gaps[0].ReasonCode != t21RuntimeGap || !slices.Equal(gaps[0].RequiredFacts, []string{t21RuntimeFact}) {
		t.Fatalf("gap = %#v, want %s/%s", gaps[0], t21RuntimeGap, t21RuntimeFact)
	}
}

// Frozen T21 first path: a proven sequence plus an applicable RESTART
// declaration warns that prior values can be reused and reports the missing
// runtime-issuance fact without pretending a current/next value exists.
func TestT21A1FirstPathRestartWarningAndRuntimeGap(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21RestartRule)
	result, err := t21Audit(t, "ALTER SEQUENCE seq RESTART WITH 1", t21Fixture(), policyPath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements = %d, want 1", len(result.Statements))
	}
	stmt := result.Statements[0]
	findings := t21Findings(result, 0, t21RestartRule)
	if len(findings) != 1 {
		t.Fatalf("restart findings = %#v, want exactly one", stmt.Findings)
	}
	finding := findings[0]
	if finding.Level != rule.LevelWarning {
		t.Fatalf("finding level = %s, want warning", finding.Level)
	}
	if finding.Message != "Restarting a sequence can reuse values previously issued." {
		t.Fatalf("finding message = %q", finding.Message)
	}
	if finding.Metadata["schema"] != "t21" || finding.Metadata["sequence"] != "seq" || finding.Metadata["operation"] != "restart" {
		t.Fatalf("finding metadata = %#v", finding.Metadata)
	}
	for _, key := range []string{"value", "current", "next", "comment"} {
		if _, ok := finding.Metadata[key]; ok {
			t.Fatalf("runtime value leaked into metadata: %#v", finding.Metadata)
		}
	}
	t21AssertOnlyRuntimeGap(t, result, 0, t21RestartRule)
	if stmt.Coverage.Status != report.CoverageUnverified || result.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage = %s/%s, want unverified", stmt.Coverage.Status, result.Coverage.Status)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict = %s, want review", result.Verdict)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("unsupported = %#v, want none", result.Unsupported)
	}
}

func TestT21A1RestartControlDoesNotWarn(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21RestartRule)
	for _, sql := range []string{
		"CREATE SEQUENCE safe_seq",
		"ALTER SEQUENCE seq START WITH 2",
		"ALTER SEQUENCE missing RESTART WITH 1",
	} {
		t.Run(sql, func(t *testing.T) {
			result, err := t21Audit(t, sql, t21Fixture(), policyPath)
			if err != nil && !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("statements = %d, want 1", len(result.Statements))
			}
			if findings := t21Findings(result, 0, t21RestartRule); len(findings) != 0 {
				t.Fatalf("restart findings = %#v, want none", findings)
			}
			if gaps := t21Gaps(result, 0, t21RestartRule); len(gaps) != 0 {
				t.Fatalf("restart gaps = %#v, want none", gaps)
			}
		})
	}
}

// Existence guards condition the same proven answer: IF NOT EXISTS absorbs a
// duplicate create, IF EXISTS absorbs a missing alter/drop, and the unguarded
// forms block on the proven contradiction.
func TestT21A1GuardedExistenceContracts(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule)
	cases := []struct {
		name      string
		sql       string
		wantBlock bool
	}{
		{"duplicate create blocks", "CREATE SEQUENCE seq", true},
		{"if not exists absorbs duplicate", "CREATE SEQUENCE IF NOT EXISTS seq", false},
		{"missing alter blocks", "ALTER SEQUENCE missing START WITH 5", true},
		{"if exists absorbs missing alter", "ALTER SEQUENCE IF EXISTS missing START WITH 5", false},
		{"missing drop blocks", "DROP SEQUENCE missing", true},
		{"if exists absorbs missing drop", "DROP SEQUENCE IF EXISTS missing", false},
		{"create on absent lands", "CREATE SEQUENCE missing", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := t21Audit(t, tc.sql, t21Fixture(), policyPath)
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			findings := t21Findings(result, 0, t21ValidRule)
			if tc.wantBlock && len(findings) != 1 {
				t.Fatalf("valid findings = %#v, want one blocker", findings)
			}
			if !tc.wantBlock && len(findings) != 0 {
				t.Fatalf("valid findings = %#v, want none", findings)
			}
		})
	}
}

// The cycle warning fires on a declared or catalog-proven CYCLE enablement —
// never on NOCYCLE, never on unvouched cycle state.
func TestT21A1CycleWarning(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21CycleRule)
	cases := []struct {
		name string
		sql  string
		want int
	}{
		{"declared cycle on create", "CREATE SEQUENCE cyc CYCLE", 1},
		{"declared nocycle on create", "CREATE SEQUENCE ncy NOCYCLE", 0},
		{"declared cycle on live alter", "ALTER SEQUENCE seq CYCLE", 1},
		{"no cycle member on live non-cycling", "ALTER SEQUENCE seq INCREMENT BY 5", 0},
		{"proven catalog cycle kept by alter", "ALTER SEQUENCE cycling START WITH 5", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := t21Audit(t, tc.sql, t21Fixture(), policyPath)
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if findings := t21Findings(result, 0, t21CycleRule); len(findings) != tc.want {
				t.Fatalf("cycle findings = %#v, want %d", findings, tc.want)
			}
		})
	}
}

// A vouched inbound-reference set warns per proven edge on drop; a vouched
// empty set stays silent on both findings and gaps; an unvouched set emits
// the sequence.references gap — the premise is never guessed and never
// silently treated as empty (T21-R1 B2).
func TestT21A1ReferencesWarning(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21RefsRule)
	cases := []struct {
		name     string
		sql      string
		wantFind int
		wantGap  bool
	}{
		{"vouched reference warns", "DROP SEQUENCE seq_ref", 1, false},
		{"vouched empty stays silent", "DROP SEQUENCE seq_unref", 0, false},
		{"unvouched read emits references gap", "DROP SEQUENCE seq", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := t21Audit(t, tc.sql, t21Fixture(), policyPath)
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			findings := t21Findings(result, 0, t21RefsRule)
			if len(findings) != tc.wantFind {
				t.Fatalf("references findings = %#v, want %d", findings, tc.wantFind)
			}
			gaps := t21Gaps(result, 0, t21RefsRule)
			gotGap := false
			for _, gap := range gaps {
				if gap.ReasonCode == "unknown_sequence_references" {
					gotGap = true
				}
			}
			if gotGap != tc.wantGap {
				t.Fatalf("references gap = %v, want %v (gaps %#v)", gotGap, tc.wantGap, result.Statements[0].EvidenceGaps)
			}
			if tc.wantFind == 1 {
				metadata := findings[0].Metadata
				if metadata["owner_kind"] != "table" || metadata["owner_schema"] != "t21" ||
					metadata["owner_name"] != "t" || metadata["slot"] != "default:c" {
					t.Fatalf("edge metadata = %#v, want bounded owner-slot identity", metadata)
				}
			}
		})
	}
}

// Resolver failures propagate — the read is never degraded to unavailable
// and never cached as a silent absent.
func TestT21A1ResolverErrorPropagatesUncached(t *testing.T) {
	provider := t21Fixture()
	wantErr := errors.New("catalog read refused")
	provider.objects["t21.broken"] = t21SequenceFact{name: "broken", err: wantErr}
	policyPath := t21Policy(t, t21ValidRule, t21RestartRule)
	_, err := t21Audit(t, "ALTER SEQUENCE broken RESTART WITH 1", provider, policyPath)
	if err == nil || errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected propagated resolver error, got %v", err)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

// Ordered state runs ahead of the provider snapshot: a dropped sequence is
// refused on a later alter even though the catalog row still says present,
// and a batch-created sequence binds on a later alter without any catalog row.
func TestT21A1OrderedStateAheadOfProviderSnapshot(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21RestartRule)
	result, err := t21Audit(t, "DROP SEQUENCE seq; ALTER SEQUENCE seq RESTART WITH 1", t21Fixture(), policyPath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if findings := t21Findings(result, 1, t21ValidRule); len(findings) != 1 {
		t.Fatalf("alter on batch-dropped sequence: valid findings = %#v, want one blocker", findings)
	}
	if findings := t21Findings(result, 1, t21RestartRule); len(findings) != 0 {
		t.Fatalf("restart findings = %#v, want none on absent target", findings)
	}

	result, err = t21Audit(t, "CREATE SEQUENCE missing; ALTER SEQUENCE missing RESTART WITH 1", t21Fixture(), policyPath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if findings := t21Findings(result, 1, t21RestartRule); len(findings) != 1 {
		t.Fatalf("restart findings = %#v, want one on batch-created target", findings)
	}
	t21AssertOnlyRuntimeGap(t, result, 1, t21RestartRule)
}

// Multi-target drops adjudicate per member: the proven member drops, the
// absent member blocks, and a later statement sees the partial post-state —
// no all-or-nothing rollback is assumed.
func TestT21A1MultiTargetDropPartialEffects(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21RestartRule)
	result, err := t21Audit(t, "DROP SEQUENCE seq, missing; ALTER SEQUENCE seq RESTART WITH 1", t21Fixture(), policyPath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if findings := t21Findings(result, 0, t21ValidRule); len(findings) != 1 {
		t.Fatalf("drop valid findings = %#v, want one blocker on missing", findings)
	}
	if findings := t21Findings(result, 1, t21ValidRule); len(findings) != 1 {
		t.Fatalf("alter on partially-dropped seq: valid findings = %#v, want one blocker", findings)
	}
	if findings := t21Findings(result, 1, t21RestartRule); len(findings) != 0 {
		t.Fatalf("restart findings = %#v, want none", findings)
	}
}

// An unvouched identity reports the bounded state gap — never a fabricated
// absent and never a warning.
func TestT21A1UnknownStateGapNotAbsent(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21RestartRule)
	result, err := t21Audit(t, "ALTER SEQUENCE ghost RESTART WITH 1", t21Fixture(), policyPath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	gaps := t21Gaps(result, 0, t21ValidRule)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_sequence_state" {
		t.Fatalf("valid gaps = %#v, want one unknown_sequence_state", result.Statements[0].EvidenceGaps)
	}
	if findings := t21Findings(result, 0, t21RestartRule); len(findings) != 0 {
		t.Fatalf("restart findings = %#v, want none on unproven target", findings)
	}
	if result.Statements[0].Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage = %s, want unverified", result.Statements[0].Coverage.Status)
	}
}

// A dropped schema proves every sequence identity beneath it absent — the
// object is gone with the schema, so a later alter blocks on the proven
// contradiction rather than inheriting a stale catalog row.
func TestT21A1DroppedSchemaProvesMembersAbsent(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21RestartRule)
	result, err := t21Audit(t, "DROP SCHEMA t21; ALTER SEQUENCE seq RESTART WITH 1", t21Fixture(), policyPath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if findings := t21Findings(result, 1, t21RestartRule); len(findings) != 0 {
		t.Fatalf("restart findings = %#v, want none under dropped schema", findings)
	}
	if findings := t21Findings(result, 1, t21ValidRule); len(findings) != 1 {
		t.Fatalf("valid findings = %#v, want one blocker on schema-dead member", findings)
	}
}

// With the sequence rules disabled the statement emits no findings and no
// gaps — gaps exist only while an enabled rule's premise needs them.
func TestT21A1RulesDisabledEmitNoGaps(t *testing.T) {
	policyPath := writeAllRulesDisabledPolicy(t)
	result, err := t21Audit(t, "ALTER SEQUENCE seq RESTART WITH 1", t21Fixture(), policyPath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("findings = %#v, want none", result.Statements[0].Findings)
	}
	if len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("evidence gaps = %#v, want none", result.Statements[0].EvidenceGaps)
	}
	if result.Statements[0].Coverage.Status != report.CoverageComplete {
		t.Fatalf("coverage = %s, want complete with rules disabled", result.Statements[0].Coverage.Status)
	}
}

// A proven table at the same namespace identity refuses the sequence create —
// TiDB sequences share the table namespace, and the catalog binds the name
// to its real object kind.
func TestT21A1SharedNamespaceConflictBlocksCreate(t *testing.T) {
	provider := t21Fixture()
	provider.objects["t21.users"] = t21SequenceFact{
		name: "users", status: spec.MetadataStatusConfirmed, exists: true, conflictKind: "table",
	}
	policyPath := t21Policy(t, t21ValidRule)
	result, err := t21Audit(t, "CREATE SEQUENCE users", provider, policyPath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	findings := t21Findings(result, 0, t21ValidRule)
	if len(findings) != 1 {
		t.Fatalf("valid findings = %#v, want one namespace-conflict blocker", findings)
	}
	if !strings.Contains(findings[0].Message, "table") {
		t.Fatalf("conflict message = %q, want the proven object kind named", findings[0].Message)
	}
}
