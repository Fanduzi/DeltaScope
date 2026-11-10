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
		"ts1": {status: spec.MetadataStatusConfirmed, exists: true, occupied: &unoccupied},
		"ts2": {status: spec.MetadataStatusNotFound},
	})
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
	facts map[string]t25TablespaceFact
	calls []string
}

func t25TablespaceProvider(facts map[string]t25TablespaceFact) *t25TablespaceProviderImpl {
	return &t25TablespaceProviderImpl{facts: facts}
}

func t25TablespaceProviderErr(facts map[string]t25TablespaceFact) *t25TablespaceProviderImpl {
	return &t25TablespaceProviderImpl{facts: facts}
}

func (p *t25TablespaceProviderImpl) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}

func (p *t25TablespaceProviderImpl) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

func (p *t25TablespaceProviderImpl) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.calls = append(p.calls, req.Type+":"+req.Name)
	if req.Type != "mysql_tablespace" {
		return &spec.ObjectSnapshot{Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
	}
	fact, ok := p.facts[strings.ToLower(strings.TrimSpace(req.Name))]
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
