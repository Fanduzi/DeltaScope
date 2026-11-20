//go:build postgresql

package postgresql

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGForeignKeySelection(t *testing.T) {
	s := extractPostgreSQLStatement(t, `ALTER TABLE app.t ADD FOREIGN KEY ("A,B", " X ", "A,B") REFERENCES "My DB"."Ref.Schema"." Ref " ("C,D", y, y) NOT VALID`)
	fk := spec.PGForeignKeyTarget(s)
	if fk == nil || fk.Reference.Schema != "Ref.Schema" || fk.Reference.Name != " Ref " || !fk.ReferenceCatalogQualified || fk.ReferenceQualified() || !slices.Equal(fk.Columns, []string{"A,B", " X ", "A,B"}) || !slices.Equal(fk.ReferencedColumns, []string{"C,D", "y", "y"}) {
		t.Fatalf("selection = %+v", fk)
	}
	b, err := json.Marshal(s.DDL)
	if err != nil || strings.Contains(string(b), "PGForeignKey") || strings.Contains(string(b), "ReferenceCatalogQualified") {
		t.Fatalf("private facts in JSON: %s (%v)", b, err)
	}
	omitted := spec.PGForeignKeyTarget(extractPostgreSQLStatement(t, `ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.parent`))
	if omitted == nil || !omitted.ReferenceQualified() || len(omitted.ReferencedColumns) != 0 {
		t.Fatalf("omitted reference list = %+v", omitted)
	}
}
