package audit

// T26-SRS-CONFLICT-IDENTITY-01 EXECUTION-01 — the frozen I1/I6/I7/I12
// contract: a NAME/ORGANIZATION spelling is a conflict-query identity
// only while decode-complete (single-quoted, no backslash byte,
// ASCII-only); ineligible spellings fire no query and leave their
// premise owed — never a silent "no conflict".

import (
	"context"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// parse the declaration directly for flag assertions.
func parseSRSDecl(t *testing.T, sql string) *spec.SpatialReferenceDeclaration {
	t.Helper()
	parsed, err := parseSQL(context.Background(), sql, spec.DialectMySQL)
	if err != nil || len(parsed.Statements) == 0 {
		t.Fatalf("parse %q: err=%v", sql, err)
	}
	stmts, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	decl := stmts[0].DDL.SpatialReference
	if decl == nil {
		t.Fatalf("no SRS declaration for %q", sql)
	}
	return decl
}

const srsCreateBase = "CREATE SPATIAL REFERENCE SYSTEM 912001 " +
	"DEFINITION 'GEOGCS[\"WGS 84\"]' "

// I1 eligibility flag matrix — quote kind, backslash, ASCII, doubled
// quote, trailing space; NAME and ORGANIZATION independently.
func TestT26SRSConflictDecodeCompleteFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		attr string
		want bool
	}{
		{"single-quoted ASCII", "NAME 'plain'", true},
		{"doubled quote", "NAME 'it''s'", true},
		{"backslash escape", `NAME 'a\nb'`, false},
		{"escaped quote backslash", `NAME 'a\'b'`, false},
		{"double-quoted", `NAME "dq"`, false},
		{"non-ASCII", "NAME 'sé'", false},
		{"trailing space", "NAME 'x '", true},
		{"leading space", "NAME ' x'", true},
		{"empty", "NAME ''", true},
		{"digits", "NAME '4326'", true},
	} {
		decl := parseSRSDecl(t, srsCreateBase+tc.attr)
		if got := decl.Attrs.NameDecodeComplete; got != tc.want {
			t.Fatalf("%s: NameDecodeComplete=%v want %v (decl %+v)", tc.name, got, tc.want, decl.Attrs)
		}
	}
}

// ORGANIZATION eligibility adjudicates independently of the
// parser-proven OrgID member.
func TestT26SRSConflictOrgDecodeComplete(t *testing.T) {
	for _, tc := range []struct {
		name string
		attr string
		want bool
	}{
		{"org eligible pair", "ORGANIZATION 'EPSG' IDENTIFIED BY 4326", true},
		{"org backslash", `ORGANIZATION 'EP\SG' IDENTIFIED BY 4326`, false},
		{"org double-quoted", `ORGANIZATION "EPSG" IDENTIFIED BY 4326`, false},
		{"org non-ASCII", "ORGANIZATION 'EPéG' IDENTIFIED BY 4326", false},
	} {
		decl := parseSRSDecl(t, srsCreateBase+tc.attr)
		if got := decl.Attrs.OrgDecodeComplete; got != tc.want {
			t.Fatalf("%s: OrgDecodeComplete=%v want %v", tc.name, got, tc.want)
		}
		if !decl.Attrs.HasIdentifiedBy || decl.Attrs.OrgID != 4326 {
			t.Fatalf("%s: OrgID=%v has=%v, want proven u32 4326", tc.name, decl.Attrs.OrgID, decl.Attrs.HasIdentifiedBy)
		}
	}
}

// I6: the qualifier is populated exactly when decode-complete — the
// last call's qualifier map is the observable seam.
func TestT26SRSConflictNameQualifierGate(t *testing.T) {
	for _, tc := range []struct {
		name string
		attr string
		want bool
	}{
		{"eligible name qualifies", "NAME 'plain'", true},
		{"backslash name withheld", `NAME 'a\nb'`, false},
		{"double-quoted withheld", `NAME "dq"`, false},
		{"non-ASCII withheld", "NAME 'sé'", false},
		{"trailing space qualifies", "NAME 'x '", true},
	} {
		provider := &t26Provider{srs: map[uint32]bool{}, version: "8.0.36"}
		enriched := enrichT26(t, srsCreateBase+tc.attr, spec.DialectMySQL, provider)
		if len(provider.calls) != 1 {
			t.Fatalf("%s: resolver calls %+v, want exactly the one SRS read", tc.name, provider.calls)
		}
		_, ok := provider.calls[0].qualifiers[spec.ObjectLookupQualifierSRSName]
		if ok != tc.want {
			t.Fatalf("%s: srs_name qualifier present=%v want %v (quals %+v)", tc.name, ok, tc.want, provider.calls[0].qualifiers)
		}
		state := enriched[0].Metadata.SpatialReference
		if tc.want {
			if state == nil || state.NameConflict == nil {
				t.Fatalf("%s: eligible name must produce a read conflict fact", tc.name)
			}
		} else if state != nil && state.NameConflict != nil {
			t.Fatalf("%s: ineligible name must never produce a conflict fact", tc.name)
		}
	}
}

// I7: the composite pair fires only when BOTH members are trusted —
// the proven OrgID cannot vouch for an unproven org spelling.
func TestT26SRSConflictOrgQualifierGate(t *testing.T) {
	for _, tc := range []struct {
		name string
		attr string
		want bool
	}{
		{"eligible pair qualifies", "ORGANIZATION 'EPSG' IDENTIFIED BY 4326", true},
		{"backslash org withheld", `ORGANIZATION 'EP\SG' IDENTIFIED BY 4326`, false},
		{"double-quoted org withheld", `ORGANIZATION "EPSG" IDENTIFIED BY 4326`, false},
	} {
		provider := &t26Provider{srs: map[uint32]bool{}, version: "8.0.36"}
		enrichT26(t, srsCreateBase+tc.attr, spec.DialectMySQL, provider)
		if len(provider.calls) != 1 {
			t.Fatalf("%s: resolver calls %+v", tc.name, provider.calls)
		}
		q := provider.calls[0].qualifiers
		_, orgOK := q[spec.ObjectLookupQualifierSRSOrg]
		_, idOK := q[spec.ObjectLookupQualifierSRSOrgID]
		if orgOK != tc.want || idOK != tc.want {
			t.Fatalf("%s: org qualifiers org=%v id=%v want %v/%v", tc.name, orgOK, idOK, tc.want, tc.want)
		}
	}
}

// I12: an ineligible spelling leaves the premise owed — no query, no
// "no conflict" inference. An eligible spelling with an empty witness
// set stays owed identically (empty != proven no-conflict).
func TestT26SRSConflictOwedFallback(t *testing.T) {
	for _, tc := range []struct {
		name string
		sql  string
	}{
		{"backslash name", srsCreateBase + `NAME 'a\nb'`},
		{"double-quoted name", srsCreateBase + `NAME "dq"`},
		{"non-ASCII name", srsCreateBase + "NAME 'sé'"},
		{"eligible name, empty witness", srsCreateBase + "NAME 'plain'"},
	} {
		provider := &t26Provider{srs: map[uint32]bool{}, version: "8.0.36"}
		enriched := enrichT26(t, tc.sql, spec.DialectMySQL, provider)
		adj := enriched[0].Metadata.SRSCreate
		if adj == nil {
			t.Fatalf("%s: create adjudication missing", tc.name)
		}
		found := false
		for _, f := range adj.Owed {
			if f == spec.SRSFactNameConflict {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: owed=%v, want srs.name_conflict owed — never a silent no-conflict", tc.name, adj.Owed)
		}
	}
}

// Positive witness: an eligible name matching another SRID lands the
// refusal arm — the refusal is the adjudication's, driven by the
// returned witnesses, not by the identity itself.
func TestT26SRSConflictPositiveWitness(t *testing.T) {
	provider := &t26Provider{
		srs:             map[uint32]bool{912002: true},
		version:         "8.0.36",
		srsNameConflict: map[string][]uint32{"plain": {912002}},
	}
	enriched := enrichT26(t, srsCreateBase+"NAME 'plain'", spec.DialectMySQL, provider)
	adj := enriched[0].Metadata.SRSCreate
	if adj == nil || adj.Arm != spec.SRSCreateArmRefusedNameConflict {
		t.Fatalf("arm=%+v, want refused_name_conflict on the positive witness", adj)
	}
}

// Org witness fires the same way through the composite.
func TestT26SRSConflictOrgWitness(t *testing.T) {
	provider := &t26Provider{
		srs:            map[uint32]bool{912002: true},
		version:        "8.0.36",
		srsOrgConflict: map[string][]uint32{"EPSG|4326": {912002}},
	}
	enriched := enrichT26(t, srsCreateBase+"NAME 'plain' ORGANIZATION 'EPSG' IDENTIFIED BY 4326", spec.DialectMySQL, provider)
	adj := enriched[0].Metadata.SRSCreate
	if adj == nil || adj.Arm != spec.SRSCreateArmRefusedOrgConflict {
		t.Fatalf("arm=%+v, want refused_org_conflict on the positive witness", adj)
	}
}

// Provider error propagates — never laundered into an empty witness
// set or a resolved premise.
func TestT26SRSConflictProviderError(t *testing.T) {
	markerErr := errors.New("t26-ci-provider-marker")
	provider := &t26Provider{srs: map[uint32]bool{}, version: "8.0.36", err: markerErr}
	parsed, parseErr := parseSQL(context.Background(), srsCreateBase+"NAME 'plain'", spec.DialectMySQL)
	if parseErr != nil {
		t.Fatalf("parse: %v", parseErr)
	}
	stmts, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	_, err = enrichStatementsWithMetadata(context.Background(), spec.DialectMySQL,
		&MetadataRequest{Schema: "golden", Provider: provider}, stmts, parsed.failures)
	if err == nil || !errors.Is(err, markerErr) {
		t.Fatalf("resolver failure must propagate with the cause reachable: %v", err)
	}
}
