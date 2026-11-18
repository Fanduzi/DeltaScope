//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func TestPGColumnFormatASTFacts(t *testing.T) {
	for _, tc := range []struct{ sql, method, storage string }{
		{`ALTER TABLE t ALTER COLUMN v SET COMPRESSION pglz`, "pglz", ""},
		{`ALTER TABLE t ALTER COLUMN v SET COMPRESSION lz4`, "lz4", ""},
		{`ALTER TABLE t ALTER COLUMN v SET COMPRESSION default`, "default", ""},
		{`ALTER TABLE t ALTER COLUMN v SET COMPRESSION "PGLZ"`, "PGLZ", ""},
		{`ALTER TABLE t ALTER COLUMN v SET COMPRESSION "private_method_payload"`, "private_method_payload", ""},
		{`ALTER TABLE t ALTER COLUMN v SET STORAGE "ExTeRnAl"`, "", "ExTeRnAl"},
		{`ALTER TABLE t ALTER COLUMN v SET STORAGE DEFAULT`, "", "default"},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			s := extractPostgreSQLStatement(t, tc.sql)
			a := s.DDL.Alter[0]
			if tc.method != "" {
				if a.PGCompressionMethod == nil || *a.PGCompressionMethod != tc.method || a.Action != "set_compression" || a.Options["has_compression"] != "true" {
					t.Fatalf("lost compression fact: %+v", a)
				}
				encoded, err := json.Marshal(a)
				if err != nil || strings.Contains(string(encoded), tc.method) || strings.Contains(string(encoded), "PGCompressionMethod") {
					t.Fatalf("private method leaked: %s %v", encoded, err)
				}
			} else if a.PGCompressionMethod != nil || a.Options["storage_kind"] != tc.storage {
				t.Fatalf("lost storage case: %+v", a)
			}
		})
	}
	for _, def := range []*pg_query.Node{nil, {Node: &pg_query.Node_Integer{Integer: &pg_query.Integer{Ival: 1}}}} {
		a, ok, unsupported := alterFromCmd(&pg_query.AlterTableCmd{Subtype: pg_query.AlterTableType_AT_SetCompression, Name: "v", Def: def})
		if !ok || unsupported != nil || a.PGCompressionMethod != nil {
			t.Fatal("invented method for unmodeled AST")
		}
	}
}

func TestPGColumnCompressionFactPrivacy(t *testing.T) {
	value := "private_compression_value"
	b, err := json.Marshal(spec.Alter{Action: "set_compression", PGCompressionMethod: &value})
	if err != nil || strings.Contains(string(b), value) {
		t.Fatalf("%s %v", b, err)
	}
}
