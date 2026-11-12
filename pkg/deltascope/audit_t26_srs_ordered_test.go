package deltascope

// T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01 R3-D — the OP-7/OP-6d/e
// public/API-layer evidence. These rows run through the REAL public
// surface (Audit + Request), never the internal application exports.
//
// Capability fact at this boundary: publicMetadataProvider does not
// forward ResolveMySQLSRSDependencies — the ordered channel's direct
// deps-reader capability is structurally unreachable through the SDK,
// so a second same-SRID statement takes the per-enrich full-key cache
// path instead of the ordered consumption path. The public contract
// (findings/gaps/coverage/verdict) is mechanism-agnostic — identical
// under either. Arm/marker mechanics are proven by the internal
// realchain companions; these rows assert the public-visible contract
// plus the measured provider seam counters.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// opPublicProvider is the OP-7 custom resolver-layer fixture at the
// public surface: LoadInstanceFacts answers the observed 8.0.36
// identity, ResolveObject answers the scripted SRS catalog states,
// and LoadMySQLServerDropPrivilege serves the privilege observation.
// It deliberately implements the INTERNAL dialect signatures the
// public wrapper asserts (objectResolver + privilege provider), the
// same discovery path the production provider rides.
type opPublicProvider struct {
	version string
	srs     map[uint32]bool // srid → exists (absence answer when absent)
	account string
	grants  []string

	resolverCalls int
	loaderCalls   int
	factsCalls    int
}

func (p *opPublicProvider) LoadInstanceFacts(_ context.Context, _ Dialect, _ string) (*InstanceFacts, error) {
	p.factsCalls++
	version := p.version
	if version == "" {
		version = "8.0.36"
	}
	return &InstanceFacts{Version: version}, nil
}

func (p *opPublicProvider) LoadTableSnapshot(context.Context, Dialect, string, string) (*TableSnapshot, error) {
	return nil, nil
}

func (p *opPublicProvider) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.resolverCalls++
	if req.Type == "mysql_spatial_reference_system" {
		var srid uint32
		if _, err := parseSRIDArg(req.Name, &srid); err == nil {
			if p.srs[srid] {
				return &spec.ObjectSnapshot{
					Schema: req.Schema, Type: req.Type, Name: req.Name,
					SpatialReference: &spec.SpatialReferenceSystemState{
						SRID: srid, Read: true, Exists: true,
					},
				}, nil
			}
			return &spec.ObjectSnapshot{
				Schema: req.Schema, Type: req.Type, Name: req.Name,
				SpatialReference: &spec.SpatialReferenceSystemState{
					SRID: srid, Read: true, Absence: true,
				},
			}, nil
		}
	}
	return &spec.ObjectSnapshot{
		Schema: req.Schema, Type: req.Type, Name: req.Name,
		Status: spec.MetadataStatusUnavailable,
	}, nil
}

func (p *opPublicProvider) LoadMySQLServerDropPrivilege(_ context.Context, _ Dialect) (*MySQLServerDropPrivilegeObservation, error) {
	p.loaderCalls++
	return &MySQLServerDropPrivilegeObservation{
		Account: p.account, GrantLines: p.grants,
	}, nil
}

func parseSRIDArg(name string, out *uint32) (bool, error) {
	var v uint64
	for _, c := range name {
		if c < '0' || c > '9' {
			return false, nil
		}
		v = v*10 + uint64(c-'0')
	}
	*out = uint32(v)
	return true, nil
}

func opPublicAbsentProvider() *opPublicProvider {
	return &opPublicProvider{
		srs:     map[uint32]bool{},
		account: "app@%",
		grants:  []string{"GRANT SELECT, SUPER ON *.* TO `app`@`%`"},
	}
}

// opPublicScan marshals nothing itself — it walks the result's public
// JSON for internal ordered-provenance leakage.
func opPublicScan(t *testing.T, marshaled []byte, label string) {
	t.Helper()
	for _, needle := range []string{"OrderedAbsent", "ordered_absent", "sourceArm", "publishedIdx", "ownPrivilege", "barrier", "srsOrdered"} {
		if strings.Contains(string(marshaled), needle) {
			t.Fatalf("%s: public JSON leaks ordered provenance %q: %s", label, needle, marshaled)
		}
	}
}

// OP-7a: two-statement plain-drop batch through the REAL public
// surface — each statement carries exactly one absent blocker
// compared on fixed rule/level/message/metadata; per-statement
// coverage complete; aggregate complete; verdict reject.
func TestT26SRSOrderedPublicPlainAbsentPair(t *testing.T) {
	provider := opPublicAbsentProvider()
	result, err := Audit(context.Background(), Request{
		SQL:                "DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;",
		Dialect:            DialectMySQL,
		MetadataProvider:   provider,
		PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	for i, stmt := range result.Statements {
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("stmt%d gaps=%+v want zero", i, stmt.EvidenceGaps)
		}
		if len(stmt.Findings) != 1 {
			t.Fatalf("stmt%d findings=%+v want exactly 1", i, stmt.Findings)
		}
		f := stmt.Findings[0]
		if f.RuleID != "ddl.mysql.srs.drop.absent" || f.Level != LevelBlocker {
			t.Fatalf("stmt%d finding=%+v want the absent blocker", i, f)
		}
		if f.Message != "spatial reference system 100001 does not exist" {
			t.Fatalf("stmt%d message=%q want the exact absent message", i, f.Message)
		}
		if stmt.Coverage.Status != CoverageComplete {
			t.Fatalf("stmt%d coverage=%v want complete", i, stmt.Coverage.Status)
		}
	}
	if result.Coverage.Status != CoverageComplete {
		t.Fatalf("aggregate coverage=%v want complete", result.Coverage.Status)
	}
	if result.Verdict != VerdictReject {
		t.Fatalf("verdict=%v want reject — a proven absent refusal", result.Verdict)
	}
	// Provider seam: one full resolver forward — stmt2's identical
	// full-key answer rides the per-enrich cache (the public boundary
	// cannot wire the deps-reader capability); the privilege loader
	// ran once.
	if provider.resolverCalls != 1 || provider.loaderCalls != 1 || provider.factsCalls != 1 {
		t.Fatalf("seams resolver/loader/facts=%d/%d/%d want 1/1/1",
			provider.resolverCalls, provider.loaderCalls, provider.factsCalls)
	}
	raw := mustMarshalResult(t, result)
	opPublicScan(t, raw, "OP-7a")
}

// OP-7b: the OP-3b lifecycle through the REAL public surface —
// IE producer (consumable) → REVOKE unsupported → IE tail with the
// suppressed premise → absent_conditional owing [P].
func TestT26SRSOrderedPublicRevokeLifecycle(t *testing.T) {
	provider := opPublicAbsentProvider()
	result, err := Audit(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;" +
			"REVOKE SELECT ON *.* FROM 'app'@'%';" +
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;",
		Dialect:            DialectMySQL,
		MetadataProvider:   provider,
		PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err == nil || !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement (REVOKE)", err)
	}
	if len(result.Statements) != 3 {
		t.Fatalf("statements=%d want 3", len(result.Statements))
	}
	// stmt1: the absent_noop notice finding + conditional_audit marker.
	stmt1 := result.Statements[0]
	if len(stmt1.Findings) != 1 || stmt1.Findings[0].RuleID != "ddl.mysql.srs.drop.absent_noop" {
		t.Fatalf("stmt1 findings=%+v want the absent_noop notice", stmt1.Findings)
	}
	if stmt1.Findings[0].Level != LevelNotice {
		t.Fatalf("stmt1 level=%v want notice", stmt1.Findings[0].Level)
	}
	if stmt1.Findings[0].Metadata["outcome"] != "absent_noop" {
		t.Fatalf("stmt1 outcome=%v want absent_noop", stmt1.Findings[0].Metadata["outcome"])
	}
	if _, ok := stmt1.Findings[0].Metadata["conditional_audit"]; !ok {
		t.Fatalf("stmt1 metadata=%v want the conditional_audit marker", stmt1.Findings[0].Metadata)
	}
	// stmt3: the [P] gap only — no findings.
	stmt3 := result.Statements[2]
	if len(stmt3.Findings) != 0 {
		t.Fatalf("stmt3 findings=%+v want zero", stmt3.Findings)
	}
	if len(stmt3.EvidenceGaps) != 1 || len(stmt3.EvidenceGaps[0].RequiredFacts) != 1 ||
		stmt3.EvidenceGaps[0].RequiredFacts[0] != "srs.drop.privilege" {
		t.Fatalf("stmt3 gaps=%+v want exactly [srs.drop.privilege]", stmt3.EvidenceGaps)
	}
	raw := mustMarshalResult(t, result)
	opPublicScan(t, raw, "OP-7b")
}

// OP-7c: reserved SRID 4326 IF EXISTS — absent_noop notice + the
// reserved advisory co-fires on the non-refusal arm.
func TestT26SRSOrderedPublicReservedIfExists(t *testing.T) {
	provider := opPublicAbsentProvider()
	result, err := Audit(context.Background(), Request{
		SQL:                "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;",
		Dialect:            DialectMySQL,
		MetadataProvider:   provider,
		PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for i, stmt := range result.Statements {
		var noop, reserved bool
		for _, f := range stmt.Findings {
			switch f.RuleID {
			case "ddl.mysql.srs.drop.absent_noop":
				noop = f.Level == LevelNotice
			case "ddl.mysql.srs.drop.reserved":
				switch v := f.Metadata["srid"].(type) {
				case uint32:
					reserved = v == 4326
				case uint64:
					reserved = v == 4326
				case int:
					reserved = v == 4326
				}
			}
		}
		if !noop {
			t.Fatalf("stmt%d missing the absent_noop notice: %+v", i, stmt.Findings)
		}
		if !reserved {
			t.Fatalf("stmt%d missing the reserved advisory: %+v", i, stmt.Findings)
		}
	}
	raw := mustMarshalResult(t, result)
	opPublicScan(t, raw, "OP-7c")
}

// OP-7c′: reserved SRID 4326 plain — the refusal arm carries exactly
// the absent blocker; the reserved advisory NEVER co-fires.
func TestT26SRSOrderedPublicReservedPlain(t *testing.T) {
	provider := opPublicAbsentProvider()
	result, err := Audit(context.Background(), Request{
		SQL:                "DROP SPATIAL REFERENCE SYSTEM 4326; DROP SPATIAL REFERENCE SYSTEM 4326;",
		Dialect:            DialectMySQL,
		MetadataProvider:   provider,
		PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for i, stmt := range result.Statements {
		if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.absent" {
			t.Fatalf("stmt%d findings=%+v want exactly the absent blocker", i, stmt.Findings)
		}
		if stmt.Findings[0].Level != LevelBlocker {
			t.Fatalf("stmt%d level=%v want blocker", i, stmt.Findings[0].Level)
		}
	}
}

// OP-7d: policy-disabled absent rule — zero findings, zero gaps,
// complete coverage, verdict pass.
func TestT26SRSOrderedPublicPolicyDisabled(t *testing.T) {
	provider := opPublicAbsentProvider()
	policy := writeOpPolicy(t, "rules:\n  ddl.mysql.srs.drop.absent:\n    enabled: false\n")
	result, err := Audit(context.Background(), Request{
		SQL:                "DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;",
		Dialect:            DialectMySQL,
		ConfigPath:         policy,
		MetadataProvider:   provider,
		PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for i, stmt := range result.Statements {
		if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("stmt%d findings=%+v gaps=%+v want both zero", i, stmt.Findings, stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != CoverageComplete {
			t.Fatalf("stmt%d coverage=%v want complete", i, stmt.Coverage.Status)
		}
	}
	if result.Verdict != VerdictPass {
		t.Fatalf("verdict=%v want pass — the absent rule is disabled", result.Verdict)
	}
}

// OP-7e: policy demotes the absent rule to warning — the finding
// fires at warning, verdict review.
func TestT26SRSOrderedPublicPolicyWarning(t *testing.T) {
	provider := opPublicAbsentProvider()
	policy := writeOpPolicy(t, "rules:\n  ddl.mysql.srs.drop.absent:\n    enabled: true\n    level: warning\n")
	result, err := Audit(context.Background(), Request{
		SQL:                "DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;",
		Dialect:            DialectMySQL,
		ConfigPath:         policy,
		MetadataProvider:   provider,
		PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for i, stmt := range result.Statements {
		if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.absent" {
			t.Fatalf("stmt%d findings=%+v want the absent finding", i, stmt.Findings)
		}
		if stmt.Findings[0].Level != LevelWarning {
			t.Fatalf("stmt%d level=%v want warning", i, stmt.Findings[0].Level)
		}
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("stmt%d gaps=%+v want zero", i, stmt.EvidenceGaps)
		}
	}
	if result.Verdict != VerdictReview {
		t.Fatalf("verdict=%v want review — warning-level absent", result.Verdict)
	}
}

// OP-6d: providerless public row — pin supplies the version; the
// providerless S2 projection is unchanged by the ordered channel.
func TestT26SRSOrderedPublicProviderless(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: DialectMySQL,
		TargetVersion:      "8.0.36",
		PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements=%d want 1", len(result.Statements))
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("findings=%+v want zero — providerless never discharges", stmt.Findings)
	}
	want := []string{"srs.existence", "srs.column_dependencies", "srs.drop.privilege"}
	if len(stmt.EvidenceGaps) != 1 || len(stmt.EvidenceGaps[0].RequiredFacts) != 3 {
		t.Fatalf("gaps=%+v want the bounded [E,D,P] gap", stmt.EvidenceGaps)
	}
	for i := range want {
		if stmt.EvidenceGaps[0].RequiredFacts[i] != want[i] {
			t.Fatalf("gap[%d]=%q want %q", i, stmt.EvidenceGaps[0].RequiredFacts[i], want[i])
		}
	}
	if stmt.Coverage.Status != CoverageUnverified {
		t.Fatalf("coverage=%v want unverified", stmt.Coverage.Status)
	}
	raw := mustMarshalResult(t, result)
	opPublicScan(t, raw, "OP-6d")
}

// OP-6e: malformed pin — typed ErrInvalidTargetVersion preflight exit
// before any discovery: the installed provider's counters show ZERO.
func TestT26SRSOrderedPublicMalformedPin(t *testing.T) {
	provider := opPublicAbsentProvider()
	_, err := Audit(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: DialectMySQL,
		TargetVersion:      "8.0.x",
		MetadataProvider:   provider,
		PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err == nil || !errors.Is(err, ErrInvalidTargetVersion) {
		t.Fatalf("err=%v want ErrInvalidTargetVersion", err)
	}
	if provider.resolverCalls != 0 || provider.loaderCalls != 0 || provider.factsCalls != 0 {
		t.Fatalf("installed counters=%d/%d/%d want all zero — preflight exit",
			provider.resolverCalls, provider.loaderCalls, provider.factsCalls)
	}
}

func writeOpPolicy(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustMarshalResult(t *testing.T, result Result) []byte {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal public result: %v", err)
	}
	return raw
}
