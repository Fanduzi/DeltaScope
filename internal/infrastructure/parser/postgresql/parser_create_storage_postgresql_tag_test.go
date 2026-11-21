//go:build postgresql

package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateStorageRetainsOrderedDeclarations(t *testing.T) {
	s := extractPostgreSQLStatement(t, `CREATE UNLOGGED TABLE IF NOT EXISTS app.t (id integer) USING " Exact Method " WITH (fillfactor=78, autovacuum_enabled=false) TABLESPACE " Exact Space "`)
	if s.DDL == nil || s.DDL.PGCreateStorage == nil {
		t.Fatalf("missing storage declaration: %+v", s)
	}
	storage := s.DDL.PGCreateStorage
	if storage.Target.Table != (spec.Table{Schema: "app", Name: "t"}) || !storage.IfNotExists || storage.SessionDependent ||
		storage.Method != " Exact Method " || storage.Tablespace != " Exact Space " || len(storage.Options) != 2 {
		t.Fatalf("unexpected storage declaration: %+v", storage)
	}
	if fill, vacuum := storage.Options[0], storage.Options[1]; fill.Name != "fillfactor" || fill.Integer != 78 ||
		vacuum.Name != "autovacuum_enabled" || vacuum.Value != spec.PGTableReloptionBoolean {
		t.Fatalf("unexpected storage option values: %+v", storage.Options)
	}
}
