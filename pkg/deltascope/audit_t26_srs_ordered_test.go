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

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
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

// opPublicLeakTokens are the internal ordered-provenance identifiers
// that must never appear in serialized public output — as JSON keys
// OR inside string values.
var opPublicLeakTokens = []string{
	"OrderedAbsent", "ordered_absent", "sourceArm", "source_arm",
	"publishedIdx", "published_idx", "ownPrivilege", "own_privilege",
	"barrier", "srsOrdered",
}

// opPublicScan is a REAL recursive serialization check, not a flat
// substring test: it unmarshals the marshaled public result, walks
// every nested object/array, fails on any internal provenance key, and
// asserts the missing-vs-null discriminations the contract requires —
// omitted fields must be ABSENT keys, never present-null.
func opPublicScan(t *testing.T, marshaled []byte, label string) {
	t.Helper()
	var doc any
	if err := json.Unmarshal(marshaled, &doc); err != nil {
		t.Fatalf("%s: public result does not unmarshal: %v", label, err)
	}
	opWalkJSON(t, doc, label+"$")
	for _, needle := range opPublicLeakTokens {
		if strings.Contains(string(marshaled), needle) {
			t.Fatalf("%s: public JSON leaks ordered provenance %q: %s", label, needle, marshaled)
		}
	}
}

// opWalkJSON rejects internal provenance KEYS at every depth of the
// decoded document and present-null values for fields the contract
// marks omitempty.
func opWalkJSON(t *testing.T, node any, path string) {
	t.Helper()
	obj, ok := node.(map[string]any)
	if !ok {
		if arr, isArr := node.([]any); isArr {
			for i, item := range arr {
				opWalkJSON(t, item, path+"["+itoa(i)+"]")
			}
		}
		return
	}
	for key, val := range obj {
		for _, needle := range opPublicLeakTokens {
			if key == needle {
				t.Fatalf("%s: internal provenance key %q present in public JSON", path, key)
			}
		}
		switch key {
		case "findings", "evidence_gaps", "statements", "conditional_audit":
			// Present is fine when populated; the omitempty/missing-vs-null
			// discriminations are asserted by the callers that own the
			// expected shape. A present NULL is never acceptable.
			if val == nil {
				t.Fatalf("%s.%s: present-null — omitempty fields must be absent, never null", path, key)
			}
		}
		opWalkJSON(t, val, path+"."+key)
	}
}

// opAssertOmitted proves a key is ABSENT (not present-null, not
// present-empty) inside a decoded object — the positive side of the
// missing-vs-null discrimination.
func opAssertOmitted(t *testing.T, obj map[string]any, key, label string) {
	t.Helper()
	v, present := obj[key]
	if present {
		t.Fatalf("%s: key %q must be omitted, found %v", label, key, v)
	}
}

// opDecodeResult unmarshals the marshaled public result into a generic
// document for shape checks.
func opDecodeResult(t *testing.T, marshaled []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(marshaled, &doc); err != nil {
		t.Fatalf("public result does not unmarshal: %v", err)
	}
	return doc
}

// opPublicScanControls proves the scan discriminates both ways: a
// document WITH an internal key must be rejected, and the same
// document without it must pass — the negative/positive pair.
func TestT26SRSOrderedPublicScanControls(t *testing.T) {
	good := []byte(`{"verdict":"reject","statements":[{"index":0,"findings":[{"rule_id":"r"}]}]}`)
	leaked := []byte(`{"verdict":"reject","statements":[{"index":0,"findings":[{"rule_id":"r","barrier":true}]}]}`)
	var doc any
	if err := json.Unmarshal(leaked, &doc); err != nil {
		t.Fatal(err)
	}
	// Negative: the leak must be detected — simulate via the same walk
	// logic on a throwaway testing.T would fail the test, so verify the
	// detection predicate directly instead.
	foundLeak := false
	var walk func(n any)
	walk = func(n any) {
		if obj, ok := n.(map[string]any); ok {
			for k, v := range obj {
				for _, needle := range opPublicLeakTokens {
					if k == needle {
						foundLeak = true
					}
				}
				walk(v)
			}
			return
		}
		if arr, ok := n.([]any); ok {
			for _, item := range arr {
				walk(item)
			}
		}
	}
	walk(doc)
	if !foundLeak {
		t.Fatal("the recursive walk failed to detect an internal key — the control is broken")
	}
	opPublicScan(t, good, "scan-control-positive")
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
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
		// Positional: the statement's own index is its position.
		if stmt.Index != i {
			t.Fatalf("stmt%d Index=%d want own position %d", i, stmt.Index, i)
		}
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
		if f.StatementIndex != i {
			t.Fatalf("stmt%d finding StatementIndex=%d want own index %d", i, f.StatementIndex, i)
		}
		// Canonical metadata — exactly {srid:uint32, outcome:"refused_absent"}.
		if len(f.Metadata) != 2 {
			t.Fatalf("stmt%d metadata=%v want exactly {srid, outcome}", i, f.Metadata)
		}
		srid, ok := f.Metadata["srid"].(uint32)
		if !ok || srid != 100001 {
			t.Fatalf("stmt%d metadata srid=%v(%T) want uint32(100001)", i, f.Metadata["srid"], f.Metadata["srid"])
		}
		if f.Metadata["outcome"] != "refused_absent" {
			t.Fatalf("stmt%d metadata outcome=%v want refused_absent", i, f.Metadata["outcome"])
		}
		// conditional_audit must be ABSENT on the refusal arm.
		if _, present := f.Metadata["conditional_audit"]; present {
			t.Fatalf("stmt%d refusal metadata carries conditional_audit — must be absent", i)
		}
		if stmt.Coverage.Status != CoverageComplete {
			t.Fatalf("stmt%d coverage=%v want complete", i, stmt.Coverage.Status)
		}
	}
	// Non-positional fields identical across the pair.
	a, b := result.Statements[0].Findings[0], result.Statements[1].Findings[0]
	if a.RuleID != b.RuleID || a.Level != b.Level || a.Message != b.Message ||
		len(a.Metadata) != len(b.Metadata) || a.Metadata["srid"] != b.Metadata["srid"] ||
		a.Metadata["outcome"] != b.Metadata["outcome"] {
		t.Fatalf("non-positional finding fields differ: %+v vs %+v", a, b)
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
	// Serialized shape: statements[0].index present and 0; empty
	// evidence_gaps key omitted; conditional_audit absent (not null)
	// inside both findings' metadata.
	doc := opDecodeResult(t, raw)
	stmts, ok := doc["statements"].([]any)
	if !ok || len(stmts) != 2 {
		t.Fatalf("serialized statements=%v want array of 2", doc["statements"])
	}
	for i, rawStmt := range stmts {
		stmtObj := rawStmt.(map[string]any)
		if idx, present := stmtObj["index"]; !present || idx != float64(i) {
			t.Fatalf("serialized stmt%d index=%v want present %d", i, idx, i)
		}
		opAssertOmitted(t, stmtObj, "evidence_gaps", "OP-7a gaps")
		findingObj := stmtObj["findings"].([]any)[0].(map[string]any)
		metaObj, _ := findingObj["metadata"].(map[string]any)
		opAssertOmitted(t, metaObj, "conditional_audit", "OP-7a refusal metadata")
	}
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
	// stmt1: the absent_noop notice finding — exact message, exact
	// metadata cardinality {srid:uint32, outcome, conditional_audit},
	// where conditional_audit must be the string
	// "observed_account_new_session" (never nil/wrong type/wrong token).
	stmt1 := result.Statements[0]
	if len(stmt1.Findings) != 1 {
		t.Fatalf("stmt1 findings=%+v want exactly 1", stmt1.Findings)
	}
	f1 := stmt1.Findings[0]
	if f1.RuleID != "ddl.mysql.srs.drop.absent_noop" || f1.Level != LevelNotice {
		t.Fatalf("stmt1 finding=%+v want the absent_noop notice", f1)
	}
	if f1.Message != "spatial reference system 100001 does not exist; IF EXISTS drop is a no-op" {
		t.Fatalf("stmt1 message=%q want the exact absent_noop message", f1.Message)
	}
	if len(f1.Metadata) != 3 {
		t.Fatalf("stmt1 metadata=%v want exactly {srid, outcome, conditional_audit}", f1.Metadata)
	}
	if srid, ok := f1.Metadata["srid"].(uint32); !ok || srid != 100001 {
		t.Fatalf("stmt1 srid=%v(%T) want uint32(100001)", f1.Metadata["srid"], f1.Metadata["srid"])
	}
	if f1.Metadata["outcome"] != "absent_noop" {
		t.Fatalf("stmt1 outcome=%v want absent_noop", f1.Metadata["outcome"])
	}
	ca, ok := f1.Metadata["conditional_audit"].(string)
	if !ok || ca != "observed_account_new_session" {
		t.Fatalf("stmt1 conditional_audit=%v(%T) want the string observed_account_new_session",
			f1.Metadata["conditional_audit"], f1.Metadata["conditional_audit"])
	}
	if len(stmt1.EvidenceGaps) != 0 {
		t.Fatalf("stmt1 gaps=%+v want zero", stmt1.EvidenceGaps)
	}
	if stmt1.Coverage.Status != CoverageComplete {
		t.Fatalf("stmt1 coverage=%v want complete", stmt1.Coverage.Status)
	}
	// stmt3: the [P] gap only — exact owner/reason/required_facts and
	// its own coverage, no findings.
	stmt3 := result.Statements[2]
	if len(stmt3.Findings) != 0 {
		t.Fatalf("stmt3 findings=%+v want zero", stmt3.Findings)
	}
	if len(stmt3.EvidenceGaps) != 1 {
		t.Fatalf("stmt3 gaps=%+v want exactly one", stmt3.EvidenceGaps)
	}
	gap := stmt3.EvidenceGaps[0]
	if gap.RuleID != "ddl.mysql.srs.drop.dependency" {
		t.Fatalf("stmt3 gap owner=%q want ddl.mysql.srs.drop.dependency", gap.RuleID)
	}
	if gap.ReasonCode != "unproven_privilege" {
		t.Fatalf("stmt3 gap reason=%q want unproven_privilege", gap.ReasonCode)
	}
	if len(gap.RequiredFacts) != 1 || gap.RequiredFacts[0] != "srs.drop.privilege" {
		t.Fatalf("stmt3 required_facts=%v want exactly [srs.drop.privilege]", gap.RequiredFacts)
	}
	if stmt3.Coverage.Status != CoverageUnverified {
		t.Fatalf("stmt3 coverage=%v want unverified — a gap is owed", stmt3.Coverage.Status)
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
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	for i, stmt := range result.Statements {
		// Exactly TWO findings: absent_noop + reserved, in any order —
		// cardinality is asserted, not searched-for.
		if len(stmt.Findings) != 2 {
			t.Fatalf("stmt%d findings=%+v want exactly {absent_noop, reserved}", i, stmt.Findings)
		}
		var noopF, reservedF *Finding
		for j := range stmt.Findings {
			switch stmt.Findings[j].RuleID {
			case "ddl.mysql.srs.drop.absent_noop":
				noopF = &stmt.Findings[j]
			case "ddl.mysql.srs.drop.reserved":
				reservedF = &stmt.Findings[j]
			default:
				t.Fatalf("stmt%d unexpected finding rule %q", i, stmt.Findings[j].RuleID)
			}
		}
		if noopF == nil || reservedF == nil {
			t.Fatalf("stmt%d findings=%+v want both absent_noop and reserved", i, stmt.Findings)
		}
		// absent_noop — exact shape; the marker lives ONLY on this finding.
		if noopF.Level != LevelNotice {
			t.Fatalf("stmt%d absent_noop level=%v want notice", i, noopF.Level)
		}
		if noopF.Message != "spatial reference system 4326 does not exist; IF EXISTS drop is a no-op" {
			t.Fatalf("stmt%d absent_noop message=%q want exact", i, noopF.Message)
		}
		if len(noopF.Metadata) != 3 {
			t.Fatalf("stmt%d absent_noop metadata=%v want {srid, outcome, conditional_audit}", i, noopF.Metadata)
		}
		if srid, ok := noopF.Metadata["srid"].(uint32); !ok || srid != 4326 {
			t.Fatalf("stmt%d absent_noop srid=%v(%T) want uint32(4326)", i, noopF.Metadata["srid"], noopF.Metadata["srid"])
		}
		if noopF.Metadata["outcome"] != "absent_noop" {
			t.Fatalf("stmt%d absent_noop outcome=%v want absent_noop", i, noopF.Metadata["outcome"])
		}
		if ca, ok := noopF.Metadata["conditional_audit"].(string); !ok || ca != "observed_account_new_session" {
			t.Fatalf("stmt%d conditional_audit=%v want the exact string", i, noopF.Metadata["conditional_audit"])
		}
		// reserved — exact shape {srid:uint32} only; NEVER the marker.
		if reservedF.Level != LevelNotice {
			t.Fatalf("stmt%d reserved level=%v want notice", i, reservedF.Level)
		}
		if reservedF.Message != "spatial reference system 4326 is in a reserved SRID range" {
			t.Fatalf("stmt%d reserved message=%q want exact", i, reservedF.Message)
		}
		if len(reservedF.Metadata) != 1 {
			t.Fatalf("stmt%d reserved metadata=%v want exactly {srid}", i, reservedF.Metadata)
		}
		if srid, ok := reservedF.Metadata["srid"].(uint32); !ok || srid != 4326 {
			t.Fatalf("stmt%d reserved srid=%v(%T) want uint32(4326)", i, reservedF.Metadata["srid"], reservedF.Metadata["srid"])
		}
		if _, present := reservedF.Metadata["conditional_audit"]; present {
			t.Fatalf("stmt%d reserved advisory carries the conditional marker — forbidden", i)
		}
		if _, present := reservedF.Metadata["outcome"]; present {
			t.Fatalf("stmt%d reserved advisory carries outcome — forbidden", i)
		}
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("stmt%d gaps=%+v want zero", i, stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != CoverageComplete {
			t.Fatalf("stmt%d coverage=%v want complete", i, stmt.Coverage.Status)
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
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	for i, stmt := range result.Statements {
		if len(stmt.Findings) != 1 {
			t.Fatalf("stmt%d findings=%+v want exactly the absent blocker", i, stmt.Findings)
		}
		f := stmt.Findings[0]
		if f.RuleID != "ddl.mysql.srs.drop.absent" || f.Level != LevelBlocker {
			t.Fatalf("stmt%d finding=%+v want the absent blocker", i, f)
		}
		if f.Message != "spatial reference system 4326 does not exist" {
			t.Fatalf("stmt%d message=%q want exact", i, f.Message)
		}
		if len(f.Metadata) != 2 {
			t.Fatalf("stmt%d metadata=%v want exactly {srid, outcome}", i, f.Metadata)
		}
		if srid, ok := f.Metadata["srid"].(uint32); !ok || srid != 4326 {
			t.Fatalf("stmt%d srid=%v(%T) want uint32(4326)", i, f.Metadata["srid"], f.Metadata["srid"])
		}
		if f.Metadata["outcome"] != "refused_absent" {
			t.Fatalf("stmt%d outcome=%v want refused_absent", i, f.Metadata["outcome"])
		}
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("stmt%d gaps=%+v want zero", i, stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != CoverageComplete {
			t.Fatalf("stmt%d coverage=%v want complete", i, stmt.Coverage.Status)
		}
	}
	raw := mustMarshalResult(t, result)
	opPublicScan(t, raw, "OP-7c-plain")
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
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2 — guard against an empty loop passing", len(result.Statements))
	}
	for i, stmt := range result.Statements {
		if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("stmt%d findings=%+v gaps=%+v want both zero", i, stmt.Findings, stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != CoverageComplete {
			t.Fatalf("stmt%d coverage=%v want complete", i, stmt.Coverage.Status)
		}
	}
	if result.Coverage.Status != CoverageComplete {
		t.Fatalf("aggregate coverage=%v want complete", result.Coverage.Status)
	}
	if result.Verdict != VerdictPass {
		t.Fatalf("verdict=%v want pass — the absent rule is disabled", result.Verdict)
	}
	raw := mustMarshalResult(t, result)
	opPublicScan(t, raw, "OP-7d")
	// findings/evidence_gaps keys omitted (not empty, not null) on both.
	doc := opDecodeResult(t, raw)
	for i, rawStmt := range doc["statements"].([]any) {
		stmtObj := rawStmt.(map[string]any)
		opAssertOmitted(t, stmtObj, "findings", "OP-7d findings")
		opAssertOmitted(t, stmtObj, "evidence_gaps", "OP-7d gaps")
		_ = i
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
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	for i, stmt := range result.Statements {
		if len(stmt.Findings) != 1 {
			t.Fatalf("stmt%d findings=%+v want exactly the absent finding", i, stmt.Findings)
		}
		f := stmt.Findings[0]
		if f.RuleID != "ddl.mysql.srs.drop.absent" {
			t.Fatalf("stmt%d rule=%q want ddl.mysql.srs.drop.absent", i, f.RuleID)
		}
		if f.Level != LevelWarning {
			t.Fatalf("stmt%d level=%v want warning", i, f.Level)
		}
		if f.Message != "spatial reference system 100001 does not exist" {
			t.Fatalf("stmt%d message=%q want exact", i, f.Message)
		}
		if len(f.Metadata) != 2 || f.Metadata["outcome"] != "refused_absent" {
			t.Fatalf("stmt%d metadata=%v want {srid, outcome:refused_absent}", i, f.Metadata)
		}
		if srid, ok := f.Metadata["srid"].(uint32); !ok || srid != 100001 {
			t.Fatalf("stmt%d srid=%v(%T) want uint32(100001)", i, f.Metadata["srid"], f.Metadata["srid"])
		}
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("stmt%d gaps=%+v want zero", i, stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != CoverageComplete {
			t.Fatalf("stmt%d coverage=%v want complete", i, stmt.Coverage.Status)
		}
	}
	if result.Verdict != VerdictReview {
		t.Fatalf("verdict=%v want review — warning-level absent", result.Verdict)
	}
	raw := mustMarshalResult(t, result)
	opPublicScan(t, raw, "OP-7e")
}

// OP-6d (ER1): providerless public row — the pin is conflict-check
// only; with no observation the statement takes the version boundary
// (unproven) through the public surface too.
func TestT26SRSOrderedPublicProviderless(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: DialectMySQL,
		TargetVersion:      "8.0.36",
		PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err == nil || !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("err=%v want the unsupported-statement exit", err)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("unsupported=%+v want one boundary entry", result.Unsupported)
	}
	u := result.Unsupported[0]
	if u.Metadata["object"] != "spatial_reference_system" || u.Metadata["gate"] != "version" ||
		u.Metadata["version_reason"] != "version_unproven" {
		t.Fatalf("unsupported=%+v want object=srs gate=version reason=version_unproven", u)
	}
	raw := mustMarshalResult(t, result)
	opPublicScan(t, raw, "OP-6d")
}

// OP-6e: malformed pin — typed ErrInvalidTargetVersion preflight exit
// before any discovery: the installed provider's counters show ZERO.
// The frozen literal is `8.0.x.x`; `8.0.x` is kept as an explicit
// extra companion, never passed off as the frozen row.
func TestT26SRSOrderedPublicMalformedPin(t *testing.T) {
	for _, literal := range []string{"8.0.x.x", "8.0.x"} {
		t.Run("literal_"+literal, func(t *testing.T) {
			provider := opPublicAbsentProvider()
			_, err := Audit(context.Background(), Request{
				SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: DialectMySQL,
				TargetVersion:      literal,
				MetadataProvider:   provider,
				PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
			})
			if err == nil || !errors.Is(err, ErrInvalidTargetVersion) {
				t.Fatalf("literal %s: err=%v want ErrInvalidTargetVersion", literal, err)
			}
			if provider.resolverCalls != 0 || provider.loaderCalls != 0 || provider.factsCalls != 0 {
				t.Fatalf("literal %s: installed counters=%d/%d/%d want all zero — preflight exit",
					literal, provider.resolverCalls, provider.loaderCalls, provider.factsCalls)
			}
		})
	}
}

// --- R4-A1: capability forwarding discrimination --------------------------
//
// Bound source chain (verbatim, this commit):
//   pkg/deltascope/audit.go:297-314 — Audit wraps request.MetadataProvider
//     as publicMetadataProvider{provider: …} into appRequest.MetadataProvider.
//   pkg/deltascope/audit.go:644-645 — type publicMetadataProvider struct;
//     its method set is exactly the eight methods at 652-805 — no
//     ResolveMySQLSRSDependencies.
//   internal/application/audit/metadata.go:420-428 — capability judged by
//     `state.provider.(MySQLSRSDependencyReader)` on the wired provider
//     itself; metadata.go:80-82 documents that a request-level adapter or
//     wrapper is never probed.
// Therefore a user provider that DOES implement the reader still cannot
// donate the capability through the SDK adapter — the ordered channel's
// direct deps read is structurally unreachable at this boundary.

// opCapablePublicProvider is the discriminating fixture: a public
// provider that genuinely implements the reader method (counted).
type opCapablePublicProvider struct {
	*opPublicProvider
	depsCalls int
}

func (p *opCapablePublicProvider) ResolveMySQLSRSDependencies(_ context.Context, _ uint32) (*spec.SRSDependencyFact, error) {
	p.depsCalls++
	return &spec.SRSDependencyFact{}, nil
}

// TestT26SRSOrderedPublicCapableInput records the ACTUAL forwarding
// result for a capable input: the SDK never calls the reader
// (adapter lacks the method), and the second statement's behavior is
// the same full-key cache fallback an incapable provider takes.
func TestT26SRSOrderedPublicCapableInput(t *testing.T) {
	provider := &opCapablePublicProvider{opPublicProvider: opPublicAbsentProvider()}
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
	// The capable input's reader was never forwarded to: zero calls,
	// same single full-resolver forward as the incapable fixture.
	if provider.depsCalls != 0 {
		t.Fatalf("deps-reader calls=%d want 0 — adapter must not forward the capability", provider.depsCalls)
	}
	if provider.resolverCalls != 1 {
		t.Fatalf("resolver calls=%d want 1 — stmt2 rides the full-key cache fallback, same as incapable input", provider.resolverCalls)
	}
	// Public contract identical to the incapable row: two absent
	// blockers, complete, reject.
	for i, stmt := range result.Statements {
		if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.absent" {
			t.Fatalf("stmt%d findings=%+v want the absent blocker", i, stmt.Findings)
		}
	}
	if result.Verdict != VerdictReject {
		t.Fatalf("verdict=%v want reject", result.Verdict)
	}
	opPublicScan(t, mustMarshalResult(t, result), "capable-input")
}

// --- R4-A1 second branch: application→public conversion companion -------
//
// Registered layer boundary: the ordered vector below belongs to the
// APPLICATION layer (real appaudit.AuditSQL + a genuinely capable
// provider); the SDK fallback vectors in the OP-7 rows belong to the
// SDK layer. They are never the same execution.
//
// opAppCapableProvider implements the application seam with app-level
// signatures — INCLUDING the deps reader, so the ordered channel's
// capability assertion succeeds on the wired provider itself.
type opAppCapableProvider struct {
	resolverCalls int
	depsCalls     int
	loaderCalls   int
	factsCalls    int
}

func (p *opAppCapableProvider) LoadInstanceFacts(_ context.Context, _ spec.Dialect, _ string) (*spec.InstanceFacts, error) {
	p.factsCalls++
	return &spec.InstanceFacts{Version: "8.0.36"}, nil
}

func (p *opAppCapableProvider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

func (p *opAppCapableProvider) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.resolverCalls++
	return &spec.ObjectSnapshot{
		Schema: req.Schema, Type: req.Type, Name: req.Name,
		SpatialReference: &spec.SpatialReferenceSystemState{
			SRID: 100001, Read: true, Absence: true,
		},
	}, nil
}

func (p *opAppCapableProvider) LoadMySQLServerDropPrivilege(_ context.Context, _ spec.Dialect) (*spec.MySQLServerDropPrivilegeObservation, error) {
	p.loaderCalls++
	return &spec.MySQLServerDropPrivilegeObservation{
		Account:    "app@%",
		GrantLines: []string{"GRANT SELECT, SUPER ON *.* TO `app`@`%`"},
	}, nil
}

func (p *opAppCapableProvider) ResolveMySQLSRSDependencies(_ context.Context, srid uint32) (*spec.SRSDependencyFact, error) {
	p.depsCalls++
	if srid != 100001 {
		return nil, nil
	}
	return &spec.SRSDependencyFact{}, nil
}

// TestT26SRSOrderedConversionLayer generates the ordered-hit result at
// the REAL application entry (AuditSQL + capable provider), then
// converts through the REAL fromDomainResult and serializes — the
// registered application→public conversion companion proving the
// ordered path's public projection is clean. The measured app-seam
// vector (resolver=1 full, deps=1 direct — the ordered hit never
// re-resolves) is attributed to THIS layer.
func TestT26SRSOrderedConversionLayer(t *testing.T) {
	provider := &opAppCapableProvider{}
	domainResult, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL:                "DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;",
		Dialect:            spec.DialectMySQL,
		MetadataProvider:   provider,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
		TargetVersion:      "8.0.36",
	})
	if err != nil {
		t.Fatalf("appaudit.AuditSQL: %v", err)
	}
	// Ordered hit measured at the app seam: one full ResolveObject
	// (stmt1), one direct deps-reader forward (stmt2 — the ordered
	// consumption, never a second full resolve), one loader.
	if provider.resolverCalls != 1 || provider.depsCalls != 1 {
		t.Fatalf("app-seam vector resolver/deps=%d/%d want 1/1 — stmt2 must consume via the direct reader",
			provider.resolverCalls, provider.depsCalls)
	}
	public := fromDomainResult(domainResult)
	if len(public.Statements) != 2 {
		t.Fatalf("public statements=%d want 2", len(public.Statements))
	}
	for i, stmt := range public.Statements {
		if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.absent" ||
			stmt.Findings[0].Level != LevelBlocker {
			t.Fatalf("converted stmt%d findings=%+v want the absent blocker", i, stmt.Findings)
		}
		if stmt.Findings[0].Message != "spatial reference system 100001 does not exist" {
			t.Fatalf("converted stmt%d message=%q want exact", i, stmt.Findings[0].Message)
		}
	}
	if public.Verdict != VerdictReject {
		t.Fatalf("converted verdict=%v want reject", public.Verdict)
	}
	raw := mustMarshalResult(t, public)
	opPublicScan(t, raw, "conversion-layer")
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
