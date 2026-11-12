package mysqlmeta

// Package mysqlmeta: T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01
// provider rows — ResolveMySQLSRSDependencies is the ordered channel's
// deps-only direct read: the Qo support gate plus the
// ST_GEOMETRY_COLUMNS witness read for one authored SRID, never the
// existence query and never through objectCache. The stub driver
// records every issued query (attempts, including failures) and
// injects query/iteration/close sentinels at the named seams.

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// opDepsFixture scripts only the two seams the direct reader may
// touch — Qo and Qdeps; an exist fragment is registered purely so the
// tests can prove it was never issued.
func opDepsFixture(version string, deps testQueryResult) map[string]testQueryResult {
	return map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow(version),
		testSRSIdentityFragment: {queryErr: errors.New("exist read must never issue on the deps-only path")},
		testSRSDepFragment:      deps,
	}
}

func opResolveDeps(t *testing.T, results map[string]testQueryResult, srid uint32) (*spec.SRSDependencyFact, error) {
	t.Helper()
	db := openTestDB(t, results)
	return NewProvider(db).ResolveMySQLSRSDependencies(context.Background(), srid)
}

func opNoExistRead(t *testing.T) {
	t.Helper()
	if got := t26Attempt(t, testSRSIdentityFragment); got != 0 {
		t.Fatalf("Qexist attempts=%d want 0 — the deps-only path never queries existence", got)
	}
}

// OP-4 direct-reader row: in-domain Qo + empty deps answer is a real
// healthy-empty result — non-nil fact, nil Columns, no Qexist.
func TestT26SRSDepsReaderHealthyEmpty(t *testing.T) {
	fact, err := opResolveDeps(t, opDepsFixture("8.0.36",
		testQueryResult{columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}}), 100001)
	if err != nil {
		t.Fatalf("ResolveMySQLSRSDependencies: %v", err)
	}
	if fact == nil || len(fact.Columns) != 0 || fact.Complete {
		t.Fatalf("fact=%+v want non-nil empty incomplete fact", fact)
	}
	if got := t26Attempt(t, testSRSVersionFragment); got != 1 {
		t.Fatalf("Qo attempts=%d want 1", got)
	}
	if got := t26Attempt(t, testSRSDepFragment); got != 1 {
		t.Fatalf("Qdeps attempts=%d want 1", got)
	}
	opNoExistRead(t)
}

// OP-4 direct-reader row: a positive answer carries the authored SRID
// on every returned column — the valid-witness shape S1 consumes.
func TestT26SRSDepsReaderPositiveWitness(t *testing.T) {
	fact, err := opResolveDeps(t, opDepsFixture("8.0.36", testQueryResult{
		columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"},
		rows: [][]driver.Value{
			{"geo", "cities", "location"},
			{"geo", "roads", "path"},
		},
	}), 100001)
	if err != nil {
		t.Fatalf("ResolveMySQLSRSDependencies: %v", err)
	}
	if fact == nil || len(fact.Columns) != 2 {
		t.Fatalf("fact=%+v want 2 columns", fact)
	}
	for i, col := range fact.Columns {
		if col.SRID != 100001 {
			t.Fatalf("columns[%d].SRID=%d want authored 100001", i, col.SRID)
		}
	}
	if got := t26Attempt(t, testSRSVersionFragment); got != 1 {
		t.Fatalf("Qo attempts=%d want 1", got)
	}
	if got := t26Attempt(t, testSRSDepFragment); got != 1 {
		t.Fatalf("Qdeps attempts=%d want 1", got)
	}
	opNoExistRead(t)
}

// OP-4 direct-reader row: a proven-unsupported Qo banner (major < 8)
// short-circuits at the support gate — nil fact, nil error, no deps
// query, no exist query.
func TestT26SRSDepsReaderProvenUnsupported(t *testing.T) {
	fact, err := opResolveDeps(t, opDepsFixture("5.7.44",
		testQueryResult{queryErr: errors.New("deps must not issue past the support gate")}), 100001)
	if err != nil {
		t.Fatalf("ResolveMySQLSRSDependencies: %v", err)
	}
	if fact != nil {
		t.Fatalf("fact=%+v want nil — the support gate declined", fact)
	}
	if got := t26Attempt(t, testSRSVersionFragment); got != 1 {
		t.Fatalf("Qo attempts=%d want 1", got)
	}
	if got := t26Attempt(t, testSRSDepFragment); got != 0 {
		t.Fatalf("Qdeps attempts=%d want 0 — the gate declined before the catalog query", got)
	}
	opNoExistRead(t)
}

// OP-4 direct-reader row: an unparseable Qo banner keeps the accepted
// continue direction — the deps read still issues (support is
// unproven, not declined).
func TestT26SRSDepsReaderUnparseableQoContinues(t *testing.T) {
	fact, err := opResolveDeps(t, opDepsFixture("unparseable-banner",
		testQueryResult{columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}}), 100001)
	if err != nil {
		t.Fatalf("ResolveMySQLSRSDependencies: %v", err)
	}
	if fact == nil || len(fact.Columns) != 0 {
		t.Fatalf("fact=%+v want healthy-empty", fact)
	}
	if got := t26Attempt(t, testSRSVersionFragment); got != 1 {
		t.Fatalf("Qo attempts=%d want 1", got)
	}
	if got := t26Attempt(t, testSRSDepFragment); got != 1 {
		t.Fatalf("Qdeps attempts=%d want 1 — unproven support continues the read", got)
	}
	opNoExistRead(t)
}

// OP-4 direct-reader row: every error surface propagates — Qo query,
// deps query, deps mid-iteration, deps close. Each row asserts the
// full attempt vector (Qo, Qexist, Qdeps) AND a nil returned fact —
// an error never delivers a partial witness, even when the failing
// iteration already read a positive row.
func TestT26SRSDepsReaderErrorsPropagate(t *testing.T) {
	sentinel := errors.New("op-deps sentinel")
	t.Run("qo-query-error", func(t *testing.T) {
		results := opDepsFixture("8.0.36",
			testQueryResult{columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}})
		results[testSRSVersionFragment] = testQueryResult{queryErr: sentinel}
		fact, err := opResolveDeps(t, results, 100001)
		if !errors.Is(err, sentinel) {
			t.Fatalf("err=%v want errors.Is(sentinel)", err)
		}
		if fact != nil {
			t.Fatalf("fact=%+v want nil — an error never delivers a fact", fact)
		}
		if got := t26Attempt(t, testSRSVersionFragment); got != 1 {
			t.Fatalf("Qo attempts=%d want 1", got)
		}
		if got := t26Attempt(t, testSRSDepFragment); got != 0 {
			t.Fatalf("Qdeps attempts=%d want 0 — Qo failure stops the read", got)
		}
		opNoExistRead(t)
	})
	t.Run("deps-query-error", func(t *testing.T) {
		results := opDepsFixture("8.0.36", testQueryResult{queryErr: sentinel})
		fact, err := opResolveDeps(t, results, 100001)
		if !errors.Is(err, sentinel) {
			t.Fatalf("err=%v want errors.Is(sentinel)", err)
		}
		if fact != nil {
			t.Fatalf("fact=%+v want nil — an error never delivers a fact", fact)
		}
		if got := t26Attempt(t, testSRSVersionFragment); got != 1 {
			t.Fatalf("Qo attempts=%d want 1", got)
		}
		if got := t26Attempt(t, testSRSDepFragment); got != 1 {
			t.Fatalf("Qdeps attempts=%d want 1", got)
		}
		opNoExistRead(t)
	})
	t.Run("deps-iteration-error", func(t *testing.T) {
		results := opDepsFixture("8.0.36", testQueryResult{
			columns:  []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"},
			rows:     [][]driver.Value{{"s", "t", "c"}},
			rowErrAt: 1, rowErr: sentinel,
		})
		fact, err := opResolveDeps(t, results, 100001)
		if !errors.Is(err, sentinel) {
			t.Fatalf("err=%v want errors.Is(sentinel)", err)
		}
		if fact != nil {
			t.Fatalf("fact=%+v want nil — the failing iteration must not deliver a partial fact", fact)
		}
		if got := t26Attempt(t, testSRSVersionFragment); got != 1 {
			t.Fatalf("Qo attempts=%d want 1", got)
		}
		if got := t26Attempt(t, testSRSDepFragment); got != 1 {
			t.Fatalf("Qdeps attempts=%d want 1", got)
		}
		opNoExistRead(t)
	})
	t.Run("deps-close-error", func(t *testing.T) {
		results := opDepsFixture("8.0.36", testQueryResult{
			columns:  []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"},
			closeErr: sentinel,
		})
		fact, err := opResolveDeps(t, results, 100001)
		if !errors.Is(err, sentinel) {
			t.Fatalf("err=%v want errors.Is(sentinel)", err)
		}
		if fact != nil {
			t.Fatalf("fact=%+v want nil — an error never delivers a fact", fact)
		}
		if got := t26Attempt(t, testSRSVersionFragment); got != 1 {
			t.Fatalf("Qo attempts=%d want 1", got)
		}
		if got := t26Attempt(t, testSRSDepFragment); got != 1 {
			t.Fatalf("Qdeps attempts=%d want 1", got)
		}
		opNoExistRead(t)
	})
}
