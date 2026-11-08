package tidbparser

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T22 R4 permanent regressions, rebuilt from the offline review
// findings (the reviewer's temporary probes were deleted; these are
// the durable red/green pins — never claimed to reuse the original
// probe output).

// B1 — the numeric Boolean arm must stay inside the upstream v8.5.0
// acceptance domain. The fixed parser (pkg/parser
// v0.0.0-20241203170126, the v8.5.0-era grammar — verified by reading
// its toInt/Boolean action verbatim) behaves as follows:
//
//	n <= MaxInt64     → intLit → NUM → Boolean → .(int64)!=0 → ACCEPT
//	MaxInt64 < n <= MaxUint64 → intLit → NUM → .(int64) assert on a
//	                    uint64 item → upstream PANIC (target-parser
//	                    anomaly — the product must fail controlled,
//	                    never rebind a synthetic truth and never panic)
//	n > MaxUint64     → toInt fails → decLit → not NUM → syntax error
//
// So every out-of-domain numeric literal must remain a parse error
// here — never an enabled declaration.
func TestT22R4BurstableNumericDomainBound(t *testing.T) {
	parser := New()
	outOfDomain := []string{
		"9223372036854775808",   // MaxInt64+1 — upstream panic anomaly
		"18446744073709551615",  // MaxUint64 — upstream panic anomaly
		"18446744073709551616",  // 2^64 — decLit upstream, not NUM
		"99999999999999999999",  // long out-of-range digit run
		"184467440737095516160", // longer still
	}
	for _, v := range outOfDomain {
		if _, err := parser.Parse(context.Background(),
			"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE="+v); err == nil {
			t.Fatalf("BURSTABLE=%s must stay a parse error — upstream rejects or anomalously panics; never a valid enabled declaration", v)
		}
	}
	// In-domain numerics stay supported (ParseUint+<=MaxInt64 domain).
	inDomain := map[string]spec.ResourceGroupBurstable{
		"0":                   spec.ResourceGroupBurstableDisabled,
		"1":                   spec.ResourceGroupBurstableEnabled,
		"42":                  spec.ResourceGroupBurstableEnabled,
		"007":                 spec.ResourceGroupBurstableEnabled, // leading zeros: toInt accepts
		"9223372036854775807": spec.ResourceGroupBurstableEnabled, // MaxInt64 — upstream ACCEPT
	}
	for v, want := range inDomain {
		result, err := parser.Parse(context.Background(),
			"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE="+v)
		if err != nil {
			t.Fatalf("BURSTABLE=%s must parse (upstream accepts): %v", v, err)
		}
		wrapped := WrapStatements(result.Statements, result.Warnings)
		st, err := wrapped[0].Extractor.Extract(spec.DialectTiDB, wrapped[0].RawSQL)
		if err != nil {
			t.Fatalf("extract BURSTABLE=%s: %v", v, err)
		}
		var got spec.ResourceGroupBurstable
		for _, o := range st.DDL.ResourceGroup.Options {
			if o.Kind == spec.ResourceGroupOptionBurstable {
				got = o.Burstable
			}
		}
		if got != want {
			t.Fatalf("BURSTABLE=%s burstable=%v want %v", v, got, want)
		}
	}
}

// B2 — supplemented statements must bind in mixed batches: an ordinary
// statement after a supplemented one must not fail the bind. The locked
// parser leaves OriginTextPosition unset on every statement type, so a
// byte-range check can never validate anything; ordered one-to-one
// consumption plus per-type shape checks carry attribution instead.
func TestT22R4MixedBatchBinds(t *testing.T) {
	parser := New()

	// Ordinary DDL → RG supplement → ordinary DDL.
	result, err := parser.Parse(context.Background(),
		`CREATE TABLE t (c INT); CREATE RESOURCE GROUP rg BURSTABLE=FALSE; CREATE TABLE t2 (c INT)`)
	if err != nil {
		t.Fatalf("mixed batch must parse: %v", err)
	}
	if len(result.Statements) != 3 {
		t.Fatalf("statements=%d want 3", len(result.Statements))
	}

	// Same-shape control through the pre-existing default-parens
	// supplement — the same bind path must recover mixed batches too.
	if _, err := parser.Parse(context.Background(),
		`CREATE TABLE t (c INT DEFAULT (1+1)); CREATE TABLE t2 (c INT)`); err != nil {
		t.Fatalf("default-parens mixed batch must parse: %v", err)
	}
	if _, err := parser.Parse(context.Background(),
		`CREATE TABLE t2 (c INT); CREATE TABLE t (c INT DEFAULT (1+1))`); err != nil {
		t.Fatalf("default-parens trailing must parse: %v", err)
	}
}

// r4Extract mirrors the T22 seam helpers for the attribution matrix.
func r4Extract(t *testing.T, sql string) []spec.Statement {
	t.Helper()
	result, err := New().Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	out := make([]spec.Statement, 0, len(wrapped))
	for _, w := range wrapped {
		st, err := w.Extractor.Extract(spec.DialectTiDB, w.RawSQL)
		if err != nil {
			t.Fatalf("extract: %v", err)
		}
		out = append(out, st)
	}
	return out
}

func r4Burstable(t *testing.T, st spec.Statement) spec.ResourceGroupBurstable {
	t.Helper()
	for _, o := range st.DDL.ResourceGroup.Options {
		if o.Kind == spec.ResourceGroupOptionBurstable {
			return o.Burstable
		}
	}
	t.Fatal("no burstable option")
	return ""
}

// Statement-order attribution: every supplemented fact must land on its
// own statement — never bleed across index, kind, or object.
func TestT22R4BindingAttributionMatrix(t *testing.T) {
	// Same name twice, FALSE then TRUE — truths pair by statement index,
	// the second does not overwrite the first.
	stmts := r4Extract(t, `CREATE RESOURCE GROUP rg RU_PER_SEC=1 BURSTABLE=FALSE; CREATE RESOURCE GROUP rg RU_PER_SEC=1 BURSTABLE=TRUE`)
	if len(stmts) != 2 {
		t.Fatalf("statements=%d", len(stmts))
	}
	if got := r4Burstable(t, stmts[0]); got != spec.ResourceGroupBurstableDisabled {
		t.Fatalf("stmt0=%v want disabled", got)
	}
	if got := r4Burstable(t, stmts[1]); got != spec.ResourceGroupBurstableEnabled {
		t.Fatalf("stmt1=%v want enabled", got)
	}

	// Two different groups — only the second needs the supplement.
	stmts = r4Extract(t, `CREATE RESOURCE GROUP a BURSTABLE; CREATE RESOURCE GROUP b BURSTABLE=FALSE`)
	if got := r4Burstable(t, stmts[0]); got != spec.ResourceGroupBurstableEnabled {
		t.Fatalf("a=%v want enabled (bare)", got)
	}
	if got := r4Burstable(t, stmts[1]); got != spec.ResourceGroupBurstableDisabled {
		t.Fatalf("b=%v want disabled", got)
	}

	// unsupported_form first (no supplement), supplemented TRUE after —
	// the site must not bind backward.
	stmts = r4Extract(t, `CREATE RESOURCE GROUP a BURSTABLE=MODERATED; CREATE RESOURCE GROUP b BURSTABLE=TRUE`)
	if got := r4Burstable(t, stmts[0]); got != spec.ResourceGroupBurstableUnsupportedForm {
		t.Fatalf("a=%v want unsupported_form", got)
	}
	if got := r4Burstable(t, stmts[1]); got != spec.ResourceGroupBurstableEnabled {
		t.Fatalf("b=%v want enabled", got)
	}

	// Supplemented RG framed by ordinary DDL on both sides, with object
	// identity intact.
	stmts = r4Extract(t, `CREATE TABLE t (c INT); CREATE RESOURCE GROUP rg BURSTABLE=FALSE; CREATE TABLE t2 (c INT)`)
	if len(stmts) != 3 {
		t.Fatalf("statements=%d", len(stmts))
	}
	if stmts[0].DDL == nil || stmts[0].DDL.Operation != spec.DDLOperationCreateTable ||
		stmts[2].DDL.Operation != spec.DDLOperationCreateTable {
		t.Fatalf("framing tables distorted: %v / %v", stmts[0].DDL.Operation, stmts[2].DDL.Operation)
	}
	if stmts[1].DDL.ResourceGroup.Name != "rg" {
		t.Fatalf("rg name=%q", stmts[1].DDL.ResourceGroup.Name)
	}
	if got := r4Burstable(t, stmts[1]); got != spec.ResourceGroupBurstableDisabled {
		t.Fatalf("rg=%v want disabled", got)
	}

	// default-parens supplement and RG supplement interleaved in one
	// batch — each site's fact lands on its own statement.
	stmts = r4Extract(t, `CREATE TABLE d (c INT DEFAULT (1+1)); CREATE RESOURCE GROUP rg BURSTABLE=TRUE; CREATE TABLE d2 (c INT DEFAULT (2+2))`)
	if len(stmts) != 3 {
		t.Fatalf("statements=%d", len(stmts))
	}
	if got := r4Burstable(t, stmts[1]); got != spec.ResourceGroupBurstableEnabled {
		t.Fatalf("rg=%v want enabled", got)
	}
	if len(stmts[0].DDL.Columns) == 0 || len(stmts[2].DDL.Columns) == 0 {
		t.Fatal("default-parens tables lost columns")
	}

	// BURSTABLE text inside strings, quoted identifiers, and line/block
	// comments produces no site; the real statement still binds.
	stmts = r4Extract(t, "CREATE TABLE t (c INT COMMENT 'BURSTABLE=FALSE'); /* BURSTABLE=FALSE ; */ CREATE RESOURCE GROUP `burstable` BURSTABLE=TRUE -- BURSTABLE=FALSE\n")
	if len(stmts) != 2 {
		t.Fatalf("statements=%d", len(stmts))
	}
	if stmts[1].DDL.ResourceGroup.Name != "burstable" {
		t.Fatalf("name=%q", stmts[1].DDL.ResourceGroup.Name)
	}
	if got := r4Burstable(t, stmts[1]); got != spec.ResourceGroupBurstableEnabled {
		t.Fatalf("burstable=%v want enabled", got)
	}

	// UTF-8 prefix + multi-line: identity and truth survive.
	stmts = r4Extract(t, "-- 中文注释\nCREATE RESOURCE GROUP rg BURSTABLE=FALSE")
	if got := r4Burstable(t, stmts[0]); got != spec.ResourceGroupBurstableDisabled {
		t.Fatalf("utf8 stmt=%v want disabled", got)
	}
}

// Rejections and fail-closed behavior stay authored-exact.
func TestT22R4RejectionsStayClosed(t *testing.T) {
	parser := New()
	for _, sql := range []string{
		// Duplicate BURSTABLE — upstream CheckAppend also rejects.
		`CREATE RESOURCE GROUP rg BURSTABLE=TRUE BURSTABLE=FALSE`,
		// Trailing garbage after a blanked site — no half-published DDL.
		`CREATE RESOURCE GROUP rg BURSTABLE=TRUE @@bogus`,
		// Non-Boolean operands keep the base grammar's verdict.
		`CREATE RESOURCE GROUP rg BURSTABLE=-1`,
		`CREATE RESOURCE GROUP rg BURSTABLE=1.5`,
		`CREATE RESOURCE GROUP rg BURSTABLE=1e3`,
		`CREATE RESOURCE GROUP rg BURSTABLE='x'`,
		`CREATE RESOURCE GROUP rg BURSTABLE=ON`,
		`CREATE RESOURCE GROUP rg BURSTABLE=abc`,
	} {
		if _, err := parser.Parse(context.Background(), sql); err == nil {
			t.Fatalf("%q must stay a parse error", sql)
		}
	}

	// A supplemented statement carries its own statement identity — the
	// rewrite must not leak: RawSQL is restored verbatim and the
	// statement's authored text keeps its original bytes.
	result, err := parser.Parse(context.Background(),
		"CREATE TABLE x (c INT);\nCREATE RESOURCE GROUP rg BURSTABLE=TRUE")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	if got := strings.TrimSpace(wrapped[1].RawSQL); got != "CREATE RESOURCE GROUP rg BURSTABLE=TRUE" {
		t.Fatalf("RawSQL rewritten or lost: %q", got)
	}
}
