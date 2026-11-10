// Package audit verifies the T25 standalone MySQL tablespace family
// through the public AuditSQL seam: CREATE/ALTER/DROP TABLESPACE audit
// under their policy rules, the InnoDB/NDB/version/occupancy gates stay
// explicit, LOGFILE GROUP and NDB members hold the engine boundary, TiDB
// holds the vendor boundary, and the ordered batch publishes only what
// its premises prove — a create on an unproven pre-state owes occupancy
// to a later drop, a proven-absent create publishes unoccupied.
package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t25RuleTSCreate = "ddl.mysql.tablespace.create.warn"
	t25RuleTSAlter  = "ddl.mysql.tablespace.alter.warn"
	t25RuleTSDrop   = "ddl.mysql.tablespace.drop.forbid"
	t25RuleTSApp    = "ddl.mysql.tablespace.applicability"
)

func t25StandalonePolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPathConfigured(t, map[string]t05RuleConfig{
		t25RuleTSCreate: {level: "warning"},
		t25RuleTSAlter:  {level: "warning"},
		t25RuleTSDrop:   {level: "blocker", params: "      forbid: true\n"},
		t25RuleTSApp:    {level: "blocker"},
	})
}

func t25Standalone(t *testing.T, sql string, provider MetadataProvider) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL: sql, Dialect: spec.DialectMySQL, Schema: "golden",
		ConfigPath: t25StandalonePolicy(t), MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("%q: unexpected error %v", sql, err)
	}
	return result
}

// CREATE/DROP/ALTER TABLESPACE audit under their own rules — the DROP
// blocks, the mutations warn, and unproven catalog facts owe gaps rather
// than fabricating verdicts.
func TestT25StandaloneTablespaceLifecycle(t *testing.T) {
	t.Parallel()
	provider := t25Provider("innodb", t25Bool(false))

	result := t25Standalone(t, "CREATE TABLESPACE ts1 ADD DATAFILE 'x.ibd' ENGINE=INNODB", provider)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != t25RuleTSCreate || stmt.Findings[0].Level != "warning" {
		t.Fatalf("create findings = %+v, want one create warn", stmt.Findings)
	}
	found := false
	for _, gap := range stmt.EvidenceGaps {
		for _, f := range gap.RequiredFacts {
			if f == "tablespace.existence" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("create gaps = %+v, want tablespace.existence owed", stmt.EvidenceGaps)
	}

	result = t25Standalone(t, "DROP TABLESPACE ts1", provider)
	stmt = result.Statements[0]
	if result.Verdict != report.VerdictReject ||
		len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != t25RuleTSDrop || stmt.Findings[0].Level != "blocker" {
		t.Fatalf("drop = %s %+v, want reject with drop forbid", result.Verdict, stmt.Findings)
	}
	for _, gap := range stmt.EvidenceGaps {
		for _, f := range gap.RequiredFacts {
			if f != "tablespace.existence" && f != "tablespace.occupancy" {
				t.Fatalf("unexpected drop gap fact %q", f)
			}
		}
	}

	result = t25Standalone(t, "ALTER TABLESPACE ts1 RENAME TO ts2", provider)
	if len(result.Statements[0].Findings) != 1 || result.Statements[0].Findings[0].RuleID != t25RuleTSAlter {
		t.Fatalf("rename findings = %+v, want one alter warn", result.Statements[0].Findings)
	}
}

// The CREATE→DROP chain tells the truth about what was proven: offline,
// the create either executed or refused on an already-present object —
// existence is proven either way but occupancy is not, so the drop owes
// exactly the occupancy fact. With a provider-proven absent pre-state
// the create publishes unoccupied and the drop owes nothing.
func TestT25TablespaceBatchOccupancyChain(t *testing.T) {
	t.Parallel()

	result := t25Standalone(t,
		"CREATE TABLESPACE ts1 ADD DATAFILE 'x.ibd' ENGINE=INNODB; DROP TABLESPACE ts1",
		t25Provider("innodb", t25Bool(false)))
	drop := result.Statements[1]
	facts := map[string]bool{}
	for _, gap := range drop.EvidenceGaps {
		for _, f := range gap.RequiredFacts {
			facts[f] = true
		}
	}
	if facts["tablespace.existence"] {
		t.Fatalf("drop after create must not owe existence — both consistent worlds hold the object, gaps %+v", drop.EvidenceGaps)
	}
	if !facts["tablespace.occupancy"] {
		t.Fatalf("drop after unproven-create must owe occupancy — a refused-create world may carry bound tables, gaps %+v", drop.EvidenceGaps)
	}

	// Provider-proven absent → the create published a fresh unoccupied
	// object → the same-batch drop needs no gap at all.
	result = t25Standalone(t,
		"CREATE TABLESPACE ts1 ADD DATAFILE 'x.ibd' ENGINE=INNODB; DROP TABLESPACE ts1",
		t25TablespaceProvider(map[string]t25TablespaceFact{
			"ts1": {status: spec.MetadataStatusNotFound},
		}))
	if len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("proven-absent→create→drop gaps = %+v, want none", result.Statements[1].EvidenceGaps)
	}
}

// The provider's occupancy answer drives the drop premise: a proven
// occupied tablespace refuses on the bounded fact and the refusal
// preserves the exact pre-state — a following CREATE on the same name
// still sees it present.
func TestT25TablespaceProviderOccupancy(t *testing.T) {
	t.Parallel()
	occupied := true
	provider := t25TablespaceProvider(map[string]t25TablespaceFact{
		"ts1": {status: spec.MetadataStatusConfirmed, exists: true, occupied: &occupied},
	})
	result := t25Standalone(t, "DROP TABLESPACE ts1; CREATE TABLESPACE ts1 ADD DATAFILE 'x.ibd'", provider)
	drop := result.Statements[0]
	found := false
	for _, f := range drop.Findings {
		if f.RuleID == t25RuleTSApp && f.Metadata["reason_code"] == "tablespace_occupied" {
			found = true
		}
	}
	if !found {
		t.Fatalf("drop findings = %+v, want tablespace_occupied refusal", drop.Findings)
	}
	create := result.Statements[1]
	found = false
	for _, f := range create.Findings {
		if f.RuleID == t25RuleTSApp && f.Metadata["reason_code"] == "tablespace_present" {
			found = true
		}
	}
	if !found {
		t.Fatalf("create-after-refused-drop findings = %+v, want tablespace_present — the refused drop preserved pre-state", create.Findings)
	}
}

// RENAME moves the proven entry to the authored identity: the source
// tombstones absent and the target inherits existence plus occupancy —
// a following DROP of the new name needs no state gap.
func TestT25TablespaceRenameMovesIdentity(t *testing.T) {
	t.Parallel()
	unoccupied := false
	provider := t25TablespaceProvider(map[string]t25TablespaceFact{
		"ts1": {status: spec.MetadataStatusConfirmed, exists: true,
			kind: spec.TablespaceKindGeneral, occupied: &unoccupied},
		"ts2": {status: spec.MetadataStatusNotFound},
	})
	provider.version = "8.0.45"
	result := t25Standalone(t,
		"ALTER TABLESPACE ts1 RENAME TO ts2; DROP TABLESPACE ts2",
		provider)
	drop := result.Statements[1]
	if len(drop.EvidenceGaps) != 0 {
		t.Fatalf("drop-after-rename gaps = %+v, want none — the renamed entry carries proven unoccupied", drop.EvidenceGaps)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("verdict = %s, want reject on the policy forbid", result.Verdict)
	}
}

// A resolver failure propagates — a failed catalog read is never cached
// as absence or degraded into a silent gap.
func TestT25TablespaceResolverErrorPropagates(t *testing.T) {
	t.Parallel()
	provider := t25TablespaceProviderErr(map[string]t25TablespaceFact{
		"ts1": {err: context.DeadlineExceeded},
	})
	_, err := AuditSQL(context.Background(), Request{
		SQL: "DROP TABLESPACE ts1", Dialect: spec.DialectMySQL, Schema: "golden",
		ConfigPath: t25StandalonePolicy(t), MetadataProvider: provider,
	})
	if err == nil {
		t.Fatalf("provider resolver failure must propagate")
	}
}

// t25TablespaceFact is the fixture shape for the mysql_tablespace object
// resolver seam.
type t25TablespaceFact struct {
	status   spec.MetadataStatus
	exists   bool
	kind     string
	occupied *bool
	err      error
}

// t25TablespaceProvider answers a proven mysql_tablespace catalog read.
type t25TablespaceProviderImpl struct {
	facts   map[string]t25TablespaceFact
	version string
	calls   []string
}

func t25TablespaceProvider(facts map[string]t25TablespaceFact) *t25TablespaceProviderImpl {
	return &t25TablespaceProviderImpl{facts: facts}
}

func t25TablespaceProviderErr(facts map[string]t25TablespaceFact) *t25TablespaceProviderImpl {
	return &t25TablespaceProviderImpl{facts: facts}
}

func (p *t25TablespaceProviderImpl) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: p.version}, nil
}

func (p *t25TablespaceProviderImpl) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

func (p *t25TablespaceProviderImpl) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.calls = append(p.calls, req.Type+":"+req.Name)
	if req.Type != "mysql_tablespace" {
		return &spec.ObjectSnapshot{Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
	}
	fact, ok := p.facts[strings.TrimSpace(req.Name)]
	if !ok {
		return &spec.ObjectSnapshot{Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
	}
	if fact.err != nil {
		return nil, fact.err
	}
	snapshot := &spec.ObjectSnapshot{
		Type: req.Type, Name: req.Name, Status: fact.status, Exists: fact.exists,
	}
	if fact.occupied != nil || fact.kind != "" {
		snapshot.Tablespace = &spec.TablespaceState{Kind: fact.kind, Occupied: fact.occupied}
	}
	return snapshot, nil
}

// Engine and vendor boundaries stay explicit: NDB-declared members and
// LOGFILE GROUP hold the engine boundary; a non-tablespace engine refuses
// on the authored form; InnoDB datafile moves refuse.
func TestT25TablespaceEngineBoundaries(t *testing.T) {
	t.Parallel()
	provider := t25Provider("innodb", t25Bool(false))

	for _, sql := range []string{
		"CREATE LOGFILE GROUP lg1 ADD UNDOFILE 'u.und' ENGINE=NDB",
		"DROP LOGFILE GROUP lg1",
		"CREATE TABLESPACE ts1 ADD DATAFILE 'x.ibd' ENGINE=NDB",
		"DROP TABLESPACE ts1 ENGINE=NDB",
	} {
		result, err := AuditSQL(context.Background(), Request{
			SQL: sql, Dialect: spec.DialectMySQL, Schema: "golden",
			ConfigPath: t25StandalonePolicy(t), MetadataProvider: provider,
		})
		if err == nil {
			t.Fatalf("%q: NDB boundary must surface ErrUnsupportedStatement", sql)
		}
		if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 0 {
			t.Fatalf("%q: boundary statement must emit no governance findings, got %+v", sql, result.Statements)
		}
		if len(result.Unsupported) != 1 || result.Unsupported[0].Reason != spec.UnsupportedVendorBoundaryReason {
			t.Fatalf("%q: unsupported = %+v, want engine boundary", sql, result.Unsupported)
		}
	}

	result := t25Standalone(t, "CREATE TABLESPACE ts1 ADD DATAFILE 'x.ibd' ENGINE=MYISAM", provider)
	ruleIDs := map[string]bool{}
	for _, f := range result.Statements[0].Findings {
		ruleIDs[f.RuleID] = true
	}
	if !ruleIDs[t25RuleTSApp] {
		t.Fatalf("myisam create findings = %+v, want applicability refusal", result.Statements[0].Findings)
	}

	result = t25Standalone(t, "ALTER TABLESPACE ts1 ADD DATAFILE 'x.ibd' ENGINE=INNODB", provider)
	ruleIDs = map[string]bool{}
	for _, f := range result.Statements[0].Findings {
		ruleIDs[f.RuleID] = true
	}
	if !ruleIDs[t25RuleTSApp] {
		t.Fatalf("innodb add datafile findings = %+v, want applicability refusal", result.Statements[0].Findings)
	}
}

// TiDB refuses the whole family at the vendor boundary.
func TestT25TablespaceTiDBVendorBoundary(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE TABLESPACE ts1 ADD DATAFILE 'x.ibd' ENGINE=INNODB", Dialect: spec.DialectTiDB,
		Schema: "golden", ConfigPath: t25StandalonePolicy(t), MetadataProvider: t25Provider("innodb", t25Bool(false)),
	})
	if err == nil {
		t.Fatalf("TiDB tablespace must surface ErrUnsupportedStatement")
	}
	if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 0 {
		t.Fatalf("TiDB findings = %+v, want none", result.Statements)
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("unsupported = %+v, want vendor boundary", result.Unsupported)
	}
}

// UNDO TABLESPACE refuses on a proven pre-8.0 version — the surface does
// not exist there; on an unproven version the refusal stays quiet. The
// version fact is the observed banner only, so the provider carries it.
func TestT25UndoTablespaceVersionGate(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE UNDO TABLESPACE u1 ADD DATAFILE 'u.ibu' ENGINE=INNODB", Dialect: spec.DialectMySQL,
		Schema: "golden", ConfigPath: t25StandalonePolicy(t),
		MetadataProvider: &t10Provider{version: "5.7.44", tables: map[string]*spec.TableSnapshot{}},
	})
	if err != nil {
		t.Fatalf("5.7 undo create: %v", err)
	}
	ruleIDs := map[string]bool{}
	for _, f := range result.Statements[0].Findings {
		ruleIDs[f.RuleID] = true
		if f.RuleID == t25RuleTSApp && f.Metadata["reason_code"] != "tablespace_version_inapplicable" {
			t.Fatalf("undo version refusal metadata = %+v, want version_inapplicable", f.Metadata)
		}
	}
	if !ruleIDs[t25RuleTSApp] {
		t.Fatalf("5.7 undo findings = %+v, want applicability refusal", result.Statements[0].Findings)
	}

	result, err = AuditSQL(context.Background(), Request{
		SQL: "CREATE UNDO TABLESPACE u1 ADD DATAFILE 'u.ibu' ENGINE=INNODB", Dialect: spec.DialectMySQL,
		Schema: "golden", ConfigPath: t25StandalonePolicy(t),
		MetadataProvider: &t10Provider{version: "8.0.46", tables: map[string]*spec.TableSnapshot{}},
	})
	if err != nil {
		t.Fatalf("8.0 undo create: %v", err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t25RuleTSApp {
			t.Fatalf("8.0 undo must not refuse, got %+v", f)
		}
	}
}

// Tablespace names are case-sensitive identifiers upstream — `TS1` and
// `ts1` are distinct objects. A drop of the differently-cased name must
// not consume the created entry's proof and still owes both state facts.
func TestT25TablespaceNameCaseSensitive(t *testing.T) {
	t.Parallel()
	provider := t25TablespaceProvider(map[string]t25TablespaceFact{})
	result := t25Standalone(t,
		"CREATE TABLESPACE TS1 ADD DATAFILE 'x.ibd' ENGINE=INNODB; DROP TABLESPACE ts1",
		provider)
	drop := result.Statements[1]
	facts := map[string]bool{}
	for _, gap := range drop.EvidenceGaps {
		for _, f := range gap.RequiredFacts {
			facts[f] = true
		}
	}
	if !facts["tablespace.existence"] || !facts["tablespace.occupancy"] {
		t.Fatalf("drop of differently-cased name gaps = %+v, want existence+occupancy — TS1 and ts1 are distinct", drop.EvidenceGaps)
	}
}

// ALTER UNDO TABLESPACE ... SET ACTIVE|INACTIVE is a recognized form but
// outside the audited surface: it surfaces as a bounded unaudited entry,
// emits no findings, and mutates nothing.
func TestT25AlterUndoSetStateBoundary(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL: "ALTER UNDO TABLESPACE u1 SET INACTIVE", Dialect: spec.DialectMySQL,
		Schema: "golden", ConfigPath: t25StandalonePolicy(t),
		MetadataProvider: t25Provider("innodb", t25Bool(false)),
	})
	if err == nil {
		t.Fatalf("alter undo set must surface ErrUnsupportedStatement")
	}
	if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 0 {
		t.Fatalf("alter undo findings = %+v, want none", result.Statements)
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Reason != spec.UnsupportedUnauditedReason {
		t.Fatalf("unsupported = %+v, want unaudited boundary", result.Unsupported)
	}
}

// Memberless general CREATE executes on InnoDB from 8.0.14 (the server
// generates the datafile) — it is never refused on the authored form.
// Below-floor versions refuse, and an unproven engine or version owes
// the fact rather than minting a verdict.
func TestT25MemberlessCreateSemantics(t *testing.T) {
	t.Parallel()

	provider := t25TablespaceProvider(map[string]t25TablespaceFact{
		"ts1": {status: spec.MetadataStatusNotFound},
	})
	provider.version = "8.0.45"
	result := t25Standalone(t, "CREATE TABLESPACE ts1 ENGINE=INNODB; DROP TABLESPACE ts1", provider)
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t25RuleTSApp {
			t.Fatalf("memberless innodb @8.0.45 emitted applicability finding %+v, want none — supported from 8.0.14", f)
		}
	}
	drop := result.Statements[1]
	if len(drop.EvidenceGaps) != 0 {
		t.Fatalf("drop after memberless create gaps = %+v, want none — proven absent create publishes general+unoccupied", drop.EvidenceGaps)
	}

	for _, version := range []string{"8.0.13", "5.7.44"} {
		p := t25TablespaceProvider(map[string]t25TablespaceFact{
			"ts1": {status: spec.MetadataStatusNotFound},
		})
		p.version = version
		res := t25Standalone(t, "CREATE TABLESPACE ts1 ENGINE=INNODB", p)
		found := false
		for _, f := range res.Statements[0].Findings {
			if f.RuleID == t25RuleTSApp && f.Metadata["reason_code"] == "tablespace_version_inapplicable" {
				found = true
			}
		}
		if !found {
			t.Fatalf("memberless @%s findings = %+v, want version_inapplicable refusal (floor 8.0.14)", version, res.Statements[0].Findings)
		}
	}

	result = t25Standalone(t, "CREATE TABLESPACE ts1; DROP TABLESPACE ts1",
		t25TablespaceProvider(map[string]t25TablespaceFact{}))
	facts := map[string]bool{}
	for _, gap := range result.Statements[0].EvidenceGaps {
		for _, f := range gap.RequiredFacts {
			facts[f] = true
		}
	}
	if !facts["tablespace.engine"] || !facts["target.version"] {
		t.Fatalf("memberless unproven-engine create gaps = %+v, want engine+version owed", result.Statements[0].EvidenceGaps)
	}
	drop = result.Statements[1]
	facts = map[string]bool{}
	for _, gap := range drop.EvidenceGaps {
		for _, f := range gap.RequiredFacts {
			facts[f] = true
		}
	}
	if !facts["tablespace.existence"] || !facts["tablespace.occupancy"] {
		t.Fatalf("drop after unproven memberless create gaps = %+v, want existence+occupancy — the create proved nothing", drop.EvidenceGaps)
	}
}

// An unproven version premise on a version-gated declaration owes
// target.version through the gap contract and must not let the ordered
// state claim existence — the sub-8.0 refusal world keeps the object
// absent, so a same-batch drop still owes both state facts.
func TestT25UndoUnknownVersionHonestState(t *testing.T) {
	t.Parallel()
	provider := t25TablespaceProvider(map[string]t25TablespaceFact{})

	result := t25Standalone(t, "CREATE UNDO TABLESPACE u1 ADD DATAFILE 'u.ibu' ENGINE=INNODB", provider)
	facts := map[string]bool{}
	for _, gap := range result.Statements[0].EvidenceGaps {
		for _, f := range gap.RequiredFacts {
			facts[f] = true
		}
	}
	if !facts["target.version"] {
		t.Fatalf("undo create gaps = %+v, want target.version owed for the unproven 8.0 gate", result.Statements[0].EvidenceGaps)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t25RuleTSApp {
			t.Fatalf("unproven version must not mint a refusal, got %+v", f)
		}
	}

	result = t25Standalone(t,
		"CREATE UNDO TABLESPACE u1 ADD DATAFILE 'u.ibu' ENGINE=INNODB; DROP TABLESPACE u1",
		provider)
	drop := result.Statements[1]
	facts = map[string]bool{}
	for _, gap := range drop.EvidenceGaps {
		for _, f := range gap.RequiredFacts {
			facts[f] = true
		}
	}
	if !facts["tablespace.existence"] || !facts["tablespace.occupancy"] {
		t.Fatalf("drop after unproven-version undo create gaps = %+v, want existence+occupancy — the create proved nothing", drop.EvidenceGaps)
	}
}

// Known version + unproven engine: the memberless gate owes only the
// engine fact — the proven 8.0.45 version must not be re-owed.
func TestT25MemberlessKnownVersionUnknownEngine(t *testing.T) {
	t.Parallel()
	provider := t25TablespaceProvider(map[string]t25TablespaceFact{
		"ts1": {status: spec.MetadataStatusNotFound},
	})
	provider.version = "8.0.45"
	result := t25Standalone(t, "CREATE TABLESPACE ts1", provider)
	facts := map[string]bool{}
	for _, gap := range result.Statements[0].EvidenceGaps {
		for _, f := range gap.RequiredFacts {
			facts[f] = true
		}
	}
	if !facts["tablespace.engine"] || facts["target.version"] {
		t.Fatalf("memberless @8.0.45 no-engine gaps = %+v, want only tablespace.engine owed", result.Statements[0].EvidenceGaps)
	}
}

// ALTER UNDO TABLESPACE ... SET is an unaudited boundary member that
// provably never touches the modeled registry facts — a following
// statement must consume the same proven pre-state (zero contamination).
func TestT25AlterUndoSetStatePreservesBatch(t *testing.T) {
	t.Parallel()
	unoccupied := false
	provider := t25TablespaceProvider(map[string]t25TablespaceFact{
		"ts1": {status: spec.MetadataStatusConfirmed, exists: true,
			kind: spec.TablespaceKindGeneral, occupied: &unoccupied},
	})
	provider.version = "8.0.45"
	result, err := AuditSQL(context.Background(), Request{
		SQL: "ALTER UNDO TABLESPACE u1 SET INACTIVE; DROP TABLESPACE ts1", Dialect: spec.DialectMySQL,
		Schema: "golden", ConfigPath: t25StandalonePolicy(t), MetadataProvider: provider,
	})
	if err == nil {
		t.Fatalf("unaudited member must surface ErrUnsupportedStatement")
	}
	if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("alter undo emitted %+v/%+v, want the unaudited boundary silent", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
	}
	drop := result.Statements[1]
	if len(drop.EvidenceGaps) != 0 {
		t.Fatalf("drop after unaudited alter-undo gaps = %+v, want none — proven pre-state must survive", drop.EvidenceGaps)
	}
}

// An undo DROP on a proven-present, proven-empty-kind undo object still
// owes the runtime eligibility fact — bound-table count never governs
// undo eligibility, and the post-state must stay unknown so a following
// statement keeps owing existence.
func TestT25UndoDropEligibilityStaysUnknown(t *testing.T) {
	t.Parallel()
	unoccupied := false
	provider := t25TablespaceProvider(map[string]t25TablespaceFact{
		"u1": {status: spec.MetadataStatusConfirmed, exists: true,
			kind: spec.TablespaceKindUndo, occupied: &unoccupied},
	})
	provider.version = "8.0.45"
	result := t25Standalone(t,
		"DROP UNDO TABLESPACE u1; CREATE UNDO TABLESPACE u1 ADD DATAFILE 'u.ibu'", provider)
	drop := result.Statements[0]
	facts := map[string]bool{}
	for _, gap := range drop.EvidenceGaps {
		for _, f := range gap.RequiredFacts {
			facts[f] = true
		}
	}
	if !facts["tablespace.drop_eligibility"] || facts["tablespace.occupancy"] {
		t.Fatalf("undo drop gaps = %+v, want only drop_eligibility owed", drop.EvidenceGaps)
	}
	for _, f := range drop.Findings {
		if f.RuleID == t25RuleTSApp {
			t.Fatalf("undo drop emitted applicability finding %+v — eligibility is a gap, never a verdict", f)
		}
	}
	create := result.Statements[1]
	facts = map[string]bool{}
	for _, gap := range create.EvidenceGaps {
		for _, f := range gap.RequiredFacts {
			facts[f] = true
		}
	}
	if !facts["tablespace.existence"] {
		t.Fatalf("create after undecidable undo drop gaps = %+v, want existence owed — the post-state stayed unknown", create.EvidenceGaps)
	}
}

// Kind mismatch is a proven refusal holding the exact pre-state: a
// general-form DROP on a proven undo object never deletes it, and the
// undo-form DROP on a proven general object never deletes it either.
func TestT25DropKindMismatchPreserves(t *testing.T) {
	t.Parallel()
	unoccupied := false
	provider := t25TablespaceProvider(map[string]t25TablespaceFact{
		"u1": {status: spec.MetadataStatusConfirmed, exists: true,
			kind: spec.TablespaceKindUndo, occupied: &unoccupied},
		"ts1": {status: spec.MetadataStatusConfirmed, exists: true,
			kind: spec.TablespaceKindGeneral, occupied: &unoccupied},
	})
	provider.version = "8.0.45"
	result := t25Standalone(t,
		"DROP TABLESPACE u1; DROP UNDO TABLESPACE ts1; DROP TABLESPACE ts1", provider)
	for i, want := range []string{"tablespace_kind_mismatch", "tablespace_kind_mismatch"} {
		found := false
		for _, f := range result.Statements[i].Findings {
			if f.Metadata["reason_code"] == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("statement %d findings = %+v, want %s refusal", i, result.Statements[i].Findings, want)
		}
	}
	if len(result.Statements[2].EvidenceGaps) != 0 {
		t.Fatalf("drop after refused wrong-form drops gaps = %+v, want none — both refusals preserved the pre-state", result.Statements[2].EvidenceGaps)
	}
}

// Case-sensitive identity: the provider returns different catalog
// answers for TS1 and ts1, and the two lookups must stay isolated —
// cache, state, and rule results never fold.
func TestT25CaseSensitiveIdentityIsolation(t *testing.T) {
	t.Parallel()
	unoccupied := false
	provider := t25TablespaceProvider(map[string]t25TablespaceFact{
		"TS1": {status: spec.MetadataStatusConfirmed, exists: true,
			kind: spec.TablespaceKindGeneral, occupied: &unoccupied},
		"ts1": {status: spec.MetadataStatusNotFound},
	})
	provider.version = "8.0.45"
	result := t25Standalone(t, "DROP TABLESPACE TS1; DROP TABLESPACE ts1", provider)
	if len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("drop TS1 gaps = %+v, want none — proven empty general object", result.Statements[0].EvidenceGaps)
	}
	found := false
	for _, f := range result.Statements[1].Findings {
		if f.Metadata["reason_code"] == "tablespace_absent" {
			found = true
		}
	}
	if !found {
		t.Fatalf("drop ts1 findings = %+v, want absent refusal — TS1's proven state must not leak", result.Statements[1].Findings)
	}
	var ts1Calls, ts1Lower int
	for _, call := range provider.calls {
		if call == "mysql_tablespace:TS1" {
			ts1Calls++
		}
		if call == "mysql_tablespace:ts1" {
			ts1Lower++
		}
	}
	if ts1Calls != 1 || ts1Lower != 1 {
		t.Fatalf("provider calls = %+v, want exactly one lookup per authored case", provider.calls)
	}
}

// NDBCLUSTER is the documented NDB alias — it routes to the same engine
// boundary, never a generic non-InnoDB/non-NDB refusal.
func TestT25NDBClusterAliasBoundary(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL: "CREATE TABLESPACE ts1 ADD DATAFILE 'a.ibd' ENGINE=NDBCLUSTER", Dialect: spec.DialectMySQL,
		Schema: "golden", ConfigPath: t25StandalonePolicy(t),
		MetadataProvider: t25TablespaceProvider(map[string]t25TablespaceFact{}),
	})
	if err == nil {
		t.Fatalf("engine boundary must surface ErrUnsupportedStatement")
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("unsupported = %+v, want the ndbcluster engine boundary", result.Unsupported)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == t25RuleTSApp {
			t.Fatalf("ndbcluster emitted applicability finding %+v — the boundary owns it", f)
		}
	}
}

// Proven refusals preserve the exact pre-state: an ENCRYPTION alter
// refused below its 8.0.13 floor leaves the object untouched, and an
// authored-engine refusal on DROP leaves the entry live for the next
// statement.
func TestT25RefusedGuardsPreservePreState(t *testing.T) {
	t.Parallel()
	unoccupied := false
	provider := t25TablespaceProvider(map[string]t25TablespaceFact{
		"ts1": {status: spec.MetadataStatusConfirmed, exists: true,
			kind: spec.TablespaceKindGeneral, occupied: &unoccupied},
	})
	provider.version = "8.0.12"
	result := t25Standalone(t, "ALTER TABLESPACE ts1 ENCRYPTION='Y'; DROP TABLESPACE ts1", provider)
	found := false
	for _, f := range result.Statements[0].Findings {
		if f.Metadata["reason_code"] == "tablespace_version_inapplicable" {
			found = true
		}
	}
	if !found {
		t.Fatalf("encryption alter @8.0.12 findings = %+v, want version_inapplicable (floor 8.0.13)", result.Statements[0].Findings)
	}
	if len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("drop after refused alter gaps = %+v, want none — the refusal must preserve pre-state", result.Statements[1].EvidenceGaps)
	}

	result = t25Standalone(t, "DROP TABLESPACE ts1 ENGINE=MYISAM; DROP TABLESPACE ts1", provider)
	found = false
	for _, f := range result.Statements[0].Findings {
		if f.Metadata["reason_code"] == "tablespace_engine_inapplicable" {
			found = true
		}
	}
	if !found {
		t.Fatalf("drop engine=myisam findings = %+v, want engine_inapplicable refusal", result.Statements[0].Findings)
	}
	if len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("drop after refused drop gaps = %+v, want none — the refused drop never tombstoned", result.Statements[1].EvidenceGaps)
	}
}

// R4-N01: the shared rename outcome adjudicates every premise — target
// existence, source kind, and the 8.0.3 version floor — so unproven
// worlds can never be promoted to a determined migration.
func TestT25RenameJointAdjudication(t *testing.T) {
	t.Parallel()
	unoccupied := false
	base := func() *t25TablespaceProviderImpl {
		p := t25TablespaceProvider(map[string]t25TablespaceFact{
			"a": {status: spec.MetadataStatusConfirmed, exists: true,
				kind: spec.TablespaceKindGeneral, occupied: &unoccupied},
		})
		p.version = "8.0.45"
		return p
	}
	gapFacts := func(result report.Result, idx int) map[string]bool {
		facts := map[string]bool{}
		for _, gap := range result.Statements[idx].EvidenceGaps {
			for _, f := range gap.RequiredFacts {
				facts[f] = true
			}
		}
		return facts
	}

	// A: source proven general + version proven, target unknown → the
	// rename owes the target fact and must not publish a fabricated
	// source-absent for the dependent drop.
	t.Run("A target unknown stays undecidable", func(t *testing.T) {
		result := t25Standalone(t,
			"ALTER TABLESPACE a RENAME TO b; DROP TABLESPACE a", base())
		if !gapFacts(result, 0)["tablespace.rename_target"] {
			t.Fatalf("rename gaps = %+v, want tablespace.rename_target owed", result.Statements[0].EvidenceGaps)
		}
		drop := gapFacts(result, 1)
		if !drop["tablespace.existence"] {
			t.Fatalf("drop-after-undecidable-rename gaps = %+v, want existence owed — the source tombstone was never proven", result.Statements[1].EvidenceGaps)
		}
	})

	// B: both states proven, version unproven → no determined migration.
	t.Run("B unproven version stays undecidable", func(t *testing.T) {
		p := base()
		p.version = ""
		p.facts["b"] = t25TablespaceFact{status: spec.MetadataStatusNotFound}
		result := t25Standalone(t,
			"ALTER TABLESPACE a RENAME TO b; DROP TABLESPACE a", p)
		if !gapFacts(result, 0)["target.version"] {
			t.Fatalf("rename gaps = %+v, want target.version owed", result.Statements[0].EvidenceGaps)
		}
		if !gapFacts(result, 1)["tablespace.existence"] {
			t.Fatalf("drop gaps = %+v, want existence owed — migration was never proven", result.Statements[1].EvidenceGaps)
		}
	})

	// C: source present with unknown kind, target proven absent,
	// version proven → the undo-kind world keeps kind owed.
	t.Run("C unknown kind stays owed", func(t *testing.T) {
		p := base()
		p.facts["a"] = t25TablespaceFact{status: spec.MetadataStatusConfirmed, exists: true, occupied: &unoccupied}
		p.facts["b"] = t25TablespaceFact{status: spec.MetadataStatusNotFound}
		result := t25Standalone(t, "ALTER TABLESPACE a RENAME TO b; DROP TABLESPACE a", p)
		if !gapFacts(result, 0)["tablespace.kind"] {
			t.Fatalf("rename gaps = %+v, want tablespace.kind owed", result.Statements[0].EvidenceGaps)
		}
		if !gapFacts(result, 1)["tablespace.existence"] {
			t.Fatalf("drop gaps = %+v, want existence owed — kind-undecidable migration never published", result.Statements[1].EvidenceGaps)
		}
	})

	// D: proven-present target → collision refusal preserving pre-state;
	// the source survives for the dependent drop with zero gaps.
	t.Run("D collision refuses and preserves", func(t *testing.T) {
		p := base()
		p.facts["b"] = t25TablespaceFact{status: spec.MetadataStatusConfirmed, exists: true,
			kind: spec.TablespaceKindGeneral, occupied: &unoccupied}
		result := t25Standalone(t, "ALTER TABLESPACE a RENAME TO b; DROP TABLESPACE a", p)
		found := false
		for _, f := range result.Statements[0].Findings {
			if f.Metadata["reason_code"] == "tablespace_rename_target_present" {
				found = true
			}
		}
		if !found {
			t.Fatalf("collision findings = %+v, want rename_target_present refusal", result.Statements[0].Findings)
		}
		if len(result.Statements[1].EvidenceGaps) != 0 {
			t.Fatalf("drop-after-refused-rename gaps = %+v, want none — the source pre-state held", result.Statements[1].EvidenceGaps)
		}
	})

	// E: fully proven premises still migrate (positive arm preserved —
	// covered by TestT25TablespaceRenameMovesIdentity).

	// R6-N06: source existence unproven must not mask a proven-present
	// target — the shared refusal surfaces as the applicability blocker
	// and both pre-states survive for the dependent drops.
	t.Run("F unproven source still emits collision refusal", func(t *testing.T) {
		p := t25TablespaceProvider(map[string]t25TablespaceFact{
			"b": {status: spec.MetadataStatusConfirmed, exists: true,
				kind: spec.TablespaceKindGeneral, occupied: &unoccupied},
		})
		p.version = "8.0.45"
		result := t25Standalone(t,
			"ALTER TABLESPACE a RENAME TO b; DROP TABLESPACE b; DROP TABLESPACE a", p)
		var collision *rule.Finding
		for i, f := range result.Statements[0].Findings {
			if f.RuleID == t25RuleTSApp && f.Metadata["reason_code"] == "tablespace_rename_target_present" {
				collision = &result.Statements[0].Findings[i]
			}
		}
		if collision == nil {
			t.Fatalf("masked-collision findings = %+v, want the shared rename_target_present blocker", result.Statements[0].Findings)
		}
		if collision.Level != "blocker" {
			t.Fatalf("collision level = %q, want blocker", collision.Level)
		}
		// The refused rename holds both pre-states: b's proven
		// unoccupied drop adjudicates complete — no gaps, no migration
		// or contamination of its identity.
		if len(result.Statements[1].EvidenceGaps) != 0 {
			t.Fatalf("drop-of-b gaps = %+v, want none — b's proven pre-state survived the refusal", result.Statements[1].EvidenceGaps)
		}
		for _, f := range result.Statements[1].Findings {
			if f.RuleID == t25RuleTSApp {
				t.Fatalf("drop-of-b findings = %+v, want no applicability refusal on the proven unoccupied target", result.Statements[1].Findings)
			}
		}
		// a was never proven: the trailing drop still owes existence
		// and occupancy — the undecidable source was neither
		// tombstoned nor promoted, so no absent finding may appear.
		facts := gapFacts(result, 2)
		if !facts["tablespace.existence"] || !facts["tablespace.occupancy"] {
			t.Fatalf("drop-of-a gaps = %+v, want existence and occupancy owed — a stayed unproven", result.Statements[2].EvidenceGaps)
		}
		for _, f := range result.Statements[2].Findings {
			if f.Metadata["reason_code"] == "tablespace_absent" {
				t.Fatalf("drop-of-a findings = %+v, want no absent finding — a's absence was never proven", result.Statements[2].Findings)
			}
		}
	})
}

// R4-N02: capability floors are exact — the 5.7 accepted-and-ignored
// CREATE AUTOEXTEND_SIZE is never a version refusal, ENGINE_ATTRIBUTE
// bounds at 8.0.21, ALTER AUTOEXTEND_SIZE at 8.0.23.
func TestT25CapabilityFloorCalibration(t *testing.T) {
	t.Parallel()
	providerFor := func(version string) *t25TablespaceProviderImpl {
		p := t25TablespaceProvider(map[string]t25TablespaceFact{
			"ts1": {status: spec.MetadataStatusNotFound},
		})
		p.version = version
		return p
	}
	applicability := func(result report.Result) string {
		for _, f := range result.Statements[0].Findings {
			if f.RuleID == t25RuleTSApp {
				if rc, _ := f.Metadata["reason_code"].(string); rc != "" {
					return rc
				}
			}
		}
		return ""
	}

	// 5.7 CREATE with AUTOEXTEND_SIZE: accepted and ignored — the only
	// adjudication left is the unproven object state (no version refusal).
	result := t25Standalone(t,
		"CREATE TABLESPACE ts1 ADD DATAFILE 'f.ibd' AUTOEXTEND_SIZE=4194304 ENGINE=InnoDB",
		providerFor("5.7.44"))
	if rc := applicability(result); rc == "tablespace_version_inapplicable" {
		t.Fatalf("create autoextend @5.7 emitted %s — 5.7 accepts and ignores the option", rc)
	}

	// ENGINE_ATTRIBUTE floor boundary: 8.0.20 refuses, 8.0.21 supports.
	for _, tc := range []struct {
		version string
		want    string
	}{
		{"8.0.20", "tablespace_version_inapplicable"},
		{"8.0.21", ""},
	} {
		res := t25Standalone(t,
			"CREATE TABLESPACE ts1 ADD DATAFILE 'f.ibd' ENGINE_ATTRIBUTE='{\"k\":1}' ENGINE=InnoDB",
			providerFor(tc.version))
		if rc := applicability(res); (rc == "tablespace_version_inapplicable") != (tc.want != "") {
			t.Fatalf("engine_attribute @%s reason=%q, want %q", tc.version, rc, tc.want)
		}
	}

	// ALTER AUTOEXTEND_SIZE bounds at 8.0.23 (8.0.22 refuses).
	res := t25Standalone(t,
		"ALTER TABLESPACE ts1 ENCRYPTION='Y' AUTOEXTEND_SIZE=8M",
		providerFor("8.0.22"))
	if rc := applicability(res); rc != "tablespace_version_inapplicable" {
		t.Fatalf("alter autoextend @8.0.22 reason=%q, want version_inapplicable (floor 8.0.23)", rc)
	}
	res = t25Standalone(t,
		"ALTER TABLESPACE ts1 ENCRYPTION='Y' AUTOEXTEND_SIZE=8M",
		providerFor("8.0.23"))
	if rc := applicability(res); rc == "tablespace_version_inapplicable" {
		t.Fatalf("alter autoextend @8.0.23 emitted %s — the alter capability arrives at 8.0.23", rc)
	}

	// RENAME's real floor is 8.0.3: a proven 8.0.2 refuses, 8.0.3 passes
	// the version arm (remaining premises adjudicate on their own).
	res = t25Standalone(t, "ALTER TABLESPACE ts1 RENAME TO ts2", providerFor("8.0.2"))
	if rc := applicability(res); rc != "tablespace_version_inapplicable" {
		t.Fatalf("rename @8.0.2 reason=%q, want version_inapplicable (floor 8.0.3)", rc)
	}

	// R7-N03: the standalone scalar members lead the member list and
	// still land on the same capability floors — not just as trailing
	// options after a recognised leading member.
	for _, tc := range []struct {
		version string
		sql     string
		refused bool
	}{
		{"8.0.20", "ALTER TABLESPACE ts1 ENGINE_ATTRIBUTE='{\"k\":1}'", true},
		{"8.0.21", "ALTER TABLESPACE ts1 ENGINE_ATTRIBUTE='{\"k\":1}'", false},
		{"8.0.22", "ALTER TABLESPACE ts1 AUTOEXTEND_SIZE=8M", true},
		{"8.0.23", "ALTER TABLESPACE ts1 AUTOEXTEND_SIZE=8M", false},
	} {
		res = t25Standalone(t, tc.sql, providerFor(tc.version))
		if rc := applicability(res); (rc == "tablespace_version_inapplicable") != tc.refused {
			t.Fatalf("standalone alter %q @%s reason=%q, want refused=%t", tc.sql, tc.version, rc, tc.refused)
		}
	}
}
