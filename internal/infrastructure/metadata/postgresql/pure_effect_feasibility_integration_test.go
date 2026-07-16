//go:build postgresql && integration

// Package postgresqlmeta records the PG17 catalog proof feasibility ledger.
// input: live pg_catalog.pg_proc rows from the repository's PG17 integration service
// output: exact identity and negative-case evidence for the Phase 1 candidates
// pos: research evidence only; this file does not modify admission or manifests
package postgresqlmeta

import (
	"context"
	"testing"
	"time"
)

type pgProcFeasibilityRow struct {
	Name       string
	OID        uint32
	Kind       string
	ArgTypes   string
	ResultType uint32
	Volatility string
	Signature  string
}

// TestPureEffectProofFeasibility records the PG17 proof root needed before a
// later manifest expansion. Exact catalog identity, not spelling or volatility,
// is the evidence accepted by the trusted SDK path.
func TestPureEffectProofFeasibility(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PG17 pure-effect feasibility integration in short mode")
	}

	db, cleanup, err := openIntegrationDB(t)
	if err != nil {
		t.Skipf("PG17 integration unavailable (Docker/compose not running): %v", err)
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, `
		select p.proname, p.oid, p.prokind::text, p.proargtypes::text,
		       p.prorettype, p.provolatile::text,
		       pg_catalog.pg_get_function_identity_arguments(p.oid)
		from pg_catalog.pg_proc p
		where p.pronamespace = 11
		  and p.proname in ('row_number', 'rank', 'dense_rank', 'count',
		                    'sum', 'avg', 'min', 'max')
	`)
	if err != nil {
		t.Fatalf("query pg_proc feasibility ledger: %v", err)
	}
	defer rows.Close()

	byOID := make(map[uint32]pgProcFeasibilityRow)
	for rows.Next() {
		var row pgProcFeasibilityRow
		if err := rows.Scan(&row.Name, &row.OID, &row.Kind, &row.ArgTypes,
			&row.ResultType, &row.Volatility, &row.Signature); err != nil {
			t.Fatalf("scan pg_proc feasibility ledger: %v", err)
		}
		byOID[row.OID] = row
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pg_proc feasibility ledger: %v", err)
	}

	t.Run("window identities are exact immutable arity-zero rows", func(t *testing.T) {
		want := map[uint32]string{
			3100: "row_number",
			3101: "rank",
			3102: "dense_rank",
		}
		for oid, name := range want {
			row, ok := byOID[oid]
			if !ok {
				t.Fatalf("missing pg_catalog.%s OID %d", name, oid)
			}
			if row.Name != name || row.Kind != "w" || row.ArgTypes != "" || row.Volatility != "i" {
				t.Fatalf("OID %d = %+v, want %s/window/arity-0/immutable", oid, row, name)
			}
		}
	})

	t.Run("ordered-set ranking rows remain negative identities", func(t *testing.T) {
		want := map[uint32]string{3986: "rank", 3992: "dense_rank"}
		for oid, name := range want {
			row, ok := byOID[oid]
			if !ok {
				t.Fatalf("missing ordered-set pg_catalog.%s OID %d", name, oid)
			}
			if row.Name != name || row.Kind != "a" || row.ArgTypes != "2276" {
				t.Fatalf("OID %d = %+v, want %s/aggregate/any", oid, row, name)
			}
			if row.Signature != `VARIADIC "any" ORDER BY VARIADIC "any"` {
				t.Fatalf("OID %d signature = %q, want ordered-set any signature", oid, row.Signature)
			}
		}
	})

	t.Run("count identities remain exact", func(t *testing.T) {
		want := map[uint32]struct {
			name, args string
			result     uint32
		}{
			2803: {name: "count", args: "", result: 20},
			2147: {name: "count", args: "2276", result: 20},
		}
		for oid, expected := range want {
			row, ok := byOID[oid]
			if !ok {
				t.Fatalf("missing pg_catalog.count OID %d", oid)
			}
			if row.Name != expected.name || row.Kind != "a" || row.ArgTypes != expected.args || row.ResultType != expected.result || row.Volatility != "i" {
				t.Fatalf("OID %d = %+v, want count/aggregate/%q/result %d/immutable", oid, row, expected.args, expected.result)
			}
		}
	})

	t.Run("common users and orders aggregate types have catalog rows", func(t *testing.T) {
		want := map[uint32]struct {
			name, args string
			result     uint32
		}{
			2107: {name: "sum", args: "20", result: 1700},
			2108: {name: "sum", args: "23", result: 20},
			2114: {name: "sum", args: "1700", result: 1700},
			2100: {name: "avg", args: "20", result: 1700},
			2101: {name: "avg", args: "23", result: 1700},
			2103: {name: "avg", args: "1700", result: 1700},
			2131: {name: "min", args: "20", result: 20},
			2145: {name: "min", args: "25", result: 25},
			2146: {name: "min", args: "1700", result: 1700},
			2115: {name: "max", args: "20", result: 20},
			2129: {name: "max", args: "25", result: 25},
			2130: {name: "max", args: "1700", result: 1700},
		}
		for oid, expected := range want {
			row, ok := byOID[oid]
			if !ok {
				t.Fatalf("missing pg_catalog.%s OID %d for argument type %s", expected.name, oid, expected.args)
			}
			if row.Name != expected.name || row.Kind != "a" || row.ArgTypes != expected.args || row.ResultType != expected.result || row.Volatility != "i" {
				t.Fatalf("OID %d = %+v, want %s/aggregate/%s -> %d/immutable", oid, row, expected.name, expected.args, expected.result)
			}
		}
	})
}
