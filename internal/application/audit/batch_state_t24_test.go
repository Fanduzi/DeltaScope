package audit

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_state_t24_test.go pins the T24/#103 MySQL resource-group
// contract: recognition of the MySQL CPU family (never the TiDB RU
// declaration), dialect boundaries both directions, the version
// capability premise, reserved groups, existence adjudication, bounded
// gaps, and refusal-aware notices — all through the public AuditSQL
// path with an injected resolver.

type t24GroupFact struct {
	status     spec.MetadataStatus
	exists     bool
	definition *spec.MySQLResourceGroupDefinition
	err        error
}

type t24Provider struct {
	version string
	groups  map[string]t24GroupFact
}

func (p *t24Provider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: p.version}, nil
}

func (p *t24Provider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

func (p *t24Provider) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "mysql_resource_group" {
		return &spec.ObjectSnapshot{Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
	}
	key := strings.ToLower(strings.TrimSpace(req.Name))
	fact, ok := p.groups[key]
	if !ok {
		return &spec.ObjectSnapshot{Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
	}
	if fact.err != nil {
		return nil, fact.err
	}
	return &spec.ObjectSnapshot{
		Type:               req.Type,
		Name:               req.Name,
		Status:             fact.status,
		Exists:             fact.exists,
		MySQLResourceGroup: fact.definition,
	}, nil
}

const (
	t24ValidRule  = "ddl.mysql.resource_group.valid"
	t24ForceWarn  = "ddl.mysql.resource_group.force.warn"
	t24AffWarn    = "ddl.mysql.resource_group.affinity.warn"
	t24StateGap   = "unknown_resource_group_state"
	t24VersionGap = "unknown_resource_group_version"
	t24HostGap    = "unknown_host_vcpu_count"
	t24PrivGap    = "unknown_resource_group_privilege"
	t24CreateNote = "ddl.create_resource_group.notice"
	t24DropNote   = "ddl.drop_resource_group.notice"
)

func t24Groups(m map[string]t24GroupFact) *t24Provider {
	return &t24Provider{version: "8.0.36", groups: m}
}

// t24MustAudit mirrors the t22 helper but reports the provider's own
// version as the audit target — the observed/target consistency check
// needs a matching pair under MySQL.
func t24AuditRaw(t *testing.T, sql string, provider *t24Provider) (report.Result, error) {
	t.Helper()
	target := ""
	if provider != nil {
		target = provider.version
	}
	return AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          spec.DialectMySQL,
		TargetVersion:    target,
		MetadataProvider: provider,
	})
}

func t24MustAudit(t *testing.T, sql string, provider *t24Provider) report.Result {
	t.Helper()
	result, err := t24AuditRaw(t, sql, provider)
	if err != nil {
		t.Fatalf("AuditSQL(%q): %v", sql, err)
	}
	return result
}

// CREATE→ALTER→DROP on a batch-created identity resolves ahead of the
// provider — the final DROP stays a normal audited statement (no
// blocker, no state gap).
func TestT24BatchLifecycleResolvesAheadOfProvider(t *testing.T) {
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER VCPU=0-3 THREAD_PRIORITY=10 ENABLE;\n"+
		"ALTER RESOURCE GROUP rg VCPU=0-1;\n"+
		"DROP RESOURCE GROUP rg;",
		t24Groups(map[string]t24GroupFact{
			"rg": {status: spec.MetadataStatusNotFound, exists: false},
		}))
	if len(result.Unsupported) != 0 {
		t.Fatalf("unsupported = %+v, want none for the mysql family on mysql", result.Unsupported)
	}
	for i, stmt := range result.Statements {
		if stmt.Kind != string(spec.KindDDL) {
			t.Fatalf("statement %d kind = %q, want ddl", i, stmt.Kind)
		}
		if slices.Contains(t22GapCodes(stmt), t24StateGap) {
			t.Fatalf("statement %d gaps = %v, want no state gap", i, t22GapCodes(stmt))
		}
	}
	if got := len(t22Findings(result, 2, t24ValidRule)); got != 0 {
		t.Fatalf("drop findings = %d, want 0 on a batch-proven target", got)
	}
	// The declared affinity/priority warnings fire on authored evidence.
	if got := len(t22Findings(result, 0, t24AffWarn)); got != 1 {
		t.Fatalf("create affinity warns = %d, want 1", got)
	}
}

// A proven 5.7 version is a static refusal: blocker finding, no
// resolver reads, no reserved facts, every premise gap silent.
func TestT24VersionPremiseRefusesPre80(t *testing.T) {
	provider := &t24Provider{version: "5.7.44", groups: map[string]t24GroupFact{
		"usr_default": {status: spec.MetadataStatusConfirmed, exists: true},
	}}
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;\nDROP RESOURCE GROUP usr_default;",
		provider)
	for i := range result.Statements {
		findings := t22Findings(result, i, t24ValidRule)
		if len(findings) != 1 || !strings.Contains(findings[0].Message, "8.0") {
			t.Fatalf("statement %d findings = %+v, want one version refusal", i, findings)
		}
		if len(result.Statements[i].EvidenceGaps) != 0 {
			t.Fatalf("statement %d gaps = %v, want none under a proven refusal", i, result.Statements[i].EvidenceGaps)
		}
	}
}

// An unproven version owes the bounded version gap — never a guessed
// feature presence, never a fabricated refusal.
func TestT24UnprovenVersionOwesGap(t *testing.T) {
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;",
		&t24Provider{version: "", groups: map[string]t24GroupFact{}})
	gaps := t22GapCodes(result.Statements[0])
	if !slices.Contains(gaps, t24VersionGap) {
		t.Fatalf("gaps = %v, want %s", gaps, t24VersionGap)
	}
	if !slices.Contains(gaps, t24PrivGap) {
		t.Fatalf("gaps = %v, want %s", gaps, t24PrivGap)
	}
}

// The reserved builtin pair is provably present under a proven 8.0+
// version — ALTER and DROP refuse before existence adjudication.
func TestT24ReservedGroupsRefuse(t *testing.T) {
	result := t24MustAudit(t, "ALTER RESOURCE GROUP usr_default THREAD_PRIORITY=5;\nDROP RESOURCE GROUP sys_default;",
		t24Groups(map[string]t24GroupFact{}))
	for i := range result.Statements {
		findings := t22Findings(result, i, t24ValidRule)
		if len(findings) != 1 || !strings.Contains(strings.ToLower(findings[0].Message), "reserved") {
			t.Fatalf("statement %d findings = %+v, want one reserved refusal", i, findings)
		}
	}
}

// Existence adjudication: proven duplicate CREATE, proven-absent ALTER,
// proven-absent DROP — all refusals, all with the notice suppressed.
func TestT24ExistenceAdjudication(t *testing.T) {
	def := &spec.MySQLResourceGroupDefinition{GroupType: spec.MySQLResourceGroupTypeUser}
	result := t24MustAudit(t, "CREATE RESOURCE GROUP dup TYPE=USER;\nALTER RESOURCE GROUP absent ENABLE;\nDROP RESOURCE GROUP absent;",
		t24Groups(map[string]t24GroupFact{
			"dup":    {status: spec.MetadataStatusConfirmed, exists: true, definition: def},
			"absent": {status: spec.MetadataStatusNotFound, exists: false},
		}))
	for i, want := range []int{1, 1, 1} {
		if got := len(t22Findings(result, i, t24ValidRule)); got != want {
			t.Fatalf("statement %d findings = %d, want %d", i, got, want)
		}
	}
	if got := len(t22Findings(result, 0, t24CreateNote)); got != 0 {
		t.Fatalf("duplicate create notice = %d, want suppressed", got)
	}
	if got := len(t22Findings(result, 2, t24DropNote)); got != 0 {
		t.Fatalf("absent drop notice = %d, want suppressed", got)
	}
}

// A declared VCPU owes the host-capacity gap; the privilege premise
// rides every lifecycle statement.
func TestT24HostCPUAndPrivilegeGaps(t *testing.T) {
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER VCPU=0-7;",
		t24Groups(map[string]t24GroupFact{
			"rg": {status: spec.MetadataStatusNotFound, exists: false},
		}))
	gaps := t22GapCodes(result.Statements[0])
	if !slices.Contains(gaps, t24HostGap) || !slices.Contains(gaps, t24PrivGap) {
		t.Fatalf("gaps = %v, want %s and %s", gaps, t24HostGap, t24PrivGap)
	}
}

// Static refusals: priority outside the authored type's range and
// over-long names — all blockers, all with the success notices
// suppressed.
func TestT24StaticRefusals(t *testing.T) {
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=SYSTEM THREAD_PRIORITY=5;\n"+
		"CREATE RESOURCE GROUP "+strings.Repeat("x", 65)+" TYPE=USER;",
		t24Groups(map[string]t24GroupFact{}))
	for i := range result.Statements {
		if got := len(t22Findings(result, i, t24ValidRule)); got == 0 {
			t.Fatalf("statement %d has no valid-rule blocker", i)
		}
	}
}

// Positional grammar: TYPE is the required first CREATE member (the
// `=` is optional, so `TYPE USER` claims), the optional members sit in
// fixed order, a bare ALTER is a valid no-op, and any member out of
// position or repeated keeps the base grammar's parse diagnostic —
// the same verdict MySQL's own parser gives.
func TestT24PositionalGrammar(t *testing.T) {
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE USER VCPU=1 DISABLE;\n"+
		"ALTER RESOURCE GROUP rg;\n"+
		"ALTER RESOURCE GROUP rg FORCE;",
		t24Groups(map[string]t24GroupFact{
			"rg": {status: spec.MetadataStatusNotFound, exists: false},
		}))
	if len(result.Statements) != 3 {
		t.Fatalf("statements = %d, want 3", len(result.Statements))
	}
	for i := range result.Statements {
		if result.Statements[i].Kind != string(spec.KindDDL) {
			t.Fatalf("statement %d kind = %q, want ddl", i, result.Statements[i].Kind)
		}
	}
	if got := len(t22Findings(result, 0, t24AffWarn)); got != 1 {
		t.Fatalf("affinity warn on `TYPE USER` create = %d, want 1", got)
	}
	for _, sql := range []string{
		"CREATE RESOURCE GROUP rg TYPE=USER TYPE=SYSTEM",
		"CREATE RESOURCE GROUP rg VCPU=1 TYPE=USER",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=0 VCPU=1",
		"ALTER RESOURCE GROUP rg FORCE VCPU=1",
		"ALTER RESOURCE GROUP rg DISABLE FORCE VCPU=1",
		"CREATE RESOURCE GROUP rg ENABLE TYPE=USER",
		"SET RESOURCE GROUP rg FOR 1,",
	} {
		if res, err := t24AuditRaw(t, sql, t24Groups(map[string]t24GroupFact{})); err == nil {
			t.Fatalf("%q audited = %+v, want parse diagnostic", sql, res.Statements)
		} else if len(res.Diagnostics) == 0 || res.Diagnostics[0].Classification != spec.DiagnosticParserError {
			t.Fatalf("%q diagnostics = %+v, want parser_error", sql, res.Diagnostics)
		}
	}
}

// Disabled-rule coverage: with the valid rule off, declared-effect
// warnings still fire on an applicable statement and a proven refusal
// still silences them — gating reads the refusal facts, not the
// blocker finding.
func TestT24WarningGatingIndependentOfValidRule(t *testing.T) {
	policyPath := t22Policy(t, map[string]policy.RulePolicy{
		t24ValidRule: {Enabled: false},
		t24AffWarn:   {Enabled: true},
		t24ForceWarn: {Enabled: true},
	})
	provider := t24Groups(map[string]t24GroupFact{
		"absent": {status: spec.MetadataStatusNotFound, exists: false},
	})
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "CREATE RESOURCE GROUP rg TYPE=USER VCPU=0 ENABLE;\nALTER RESOURCE GROUP absent FORCE;",
		Dialect:          spec.DialectMySQL,
		TargetVersion:    provider.version,
		ConfigPath:       policyPath,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	for i := range result.Statements {
		if got := len(t22Findings(result, i, t24ValidRule)); got != 0 {
			t.Fatalf("statement %d valid findings = %d, want 0 — the rule is disabled", i, got)
		}
	}
	if got := len(t22Findings(result, 0, t24AffWarn)); got != 1 {
		t.Fatalf("affinity warn on applicable create = %d, want 1", got)
	}
	if got := len(t22Findings(result, 1, t24ForceWarn)); got != 0 {
		t.Fatalf("force warn on refused alter = %d, want 0 — refusal gating does not read the blocker", got)
	}
}

// FORCE on ALTER emits the declared-relocation warning; a refused
// FORCE statement stays silent.
func TestT24ForceWarns(t *testing.T) {
	def := &spec.MySQLResourceGroupDefinition{GroupType: spec.MySQLResourceGroupTypeUser}
	result := t24MustAudit(t, "ALTER RESOURCE GROUP rg DISABLE FORCE;\nALTER RESOURCE GROUP absent FORCE;",
		t24Groups(map[string]t24GroupFact{
			"rg":     {status: spec.MetadataStatusConfirmed, exists: true, definition: def},
			"absent": {status: spec.MetadataStatusNotFound, exists: false},
		}))
	if got := len(t22Findings(result, 0, t24ForceWarn)); got != 1 {
		t.Fatalf("force warn on proven target = %d, want 1", got)
	}
	if got := len(t22Findings(result, 1, t24ForceWarn)); got != 0 {
		t.Fatalf("force warn on refused alter = %d, want 0", got)
	}
}

// Boundary cells: the MySQL family on TiDB is a vendor boundary; the
// TiDB RU family on MySQL keeps its vendor boundary; SET ... FOR is
// bounded unaudited on MySQL, vendor on TiDB.
func t24Unsupported(result report.Result, feature string) *spec.UnsupportedDetail {
	for i := range result.Unsupported {
		if result.Unsupported[i].Feature == feature {
			return &result.Unsupported[i]
		}
	}
	return nil
}

func TestT24DialectBoundaries(t *testing.T) {
	mysql := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", t24Groups(map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}}))
	if len(mysql.Unsupported) != 0 {
		t.Fatalf("mysql-family create on mysql is unsupported: %+v", mysql.Unsupported)
	}
	tidb, err := t22Audit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", spec.DialectTiDB, nil, "")
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("tidb err = %v, want ErrUnsupportedStatement", err)
	}
	u := t24Unsupported(tidb, "create_resource_group")
	if u == nil || u.Metadata["boundary"] != "vendor" {
		t.Fatalf("mysql-family create on tidb = %+v, want vendor boundary", tidb.Unsupported)
	}
	mysqlRU, err := t24AuditRaw(t, "CREATE RESOURCE GROUP rg RU_PER_SEC=100;", t24Groups(map[string]t24GroupFact{}))
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("mysql-ru err = %v, want ErrUnsupportedStatement", err)
	}
	u = t24Unsupported(mysqlRU, "create_resource_group")
	if u == nil || u.Metadata["boundary"] != "vendor" {
		t.Fatalf("tidb-family create on mysql = %+v, want vendor boundary", mysqlRU.Unsupported)
	}
	setFor, err := t24AuditRaw(t, "SET RESOURCE GROUP rg FOR 1,2;", t24Groups(map[string]t24GroupFact{}))
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("set-for err = %v, want ErrUnsupportedStatement", err)
	}
	u = t24Unsupported(setFor, "set_resource_group")
	if u == nil || u.Metadata["boundary"] == "vendor" {
		t.Fatalf("SET FOR on mysql = %+v, want bounded unaudited set_resource_group", setFor.Unsupported)
	}
	// DROP ... IF EXISTS belongs to neither grammar — MySQL's resource-group
	// drop has no guard and TiDB's does not either — so the honest outcome
	// is a parse diagnostic, never a claimed boundary or audited drop.
	dropGuard, err := t24AuditRaw(t, "DROP RESOURCE GROUP rg IF EXISTS;", t24Groups(map[string]t24GroupFact{}))
	if err == nil {
		t.Fatalf("DROP IF EXISTS on mysql audited = %+v, want parse diagnostic", dropGuard.Statements)
	}
	if len(dropGuard.Diagnostics) == 0 || dropGuard.Diagnostics[0].Classification != spec.DiagnosticParserError {
		t.Fatalf("diagnostics = %+v, want parser_error", dropGuard.Diagnostics)
	}
}
