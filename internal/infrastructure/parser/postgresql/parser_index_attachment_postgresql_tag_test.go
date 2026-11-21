//go:build postgresql

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGIndexAttachmentPreservesBothExactNames(t *testing.T) {
	s := extractPostgreSQLStatement(t, `ALTER INDEX " Parent Schema "." Parent Index " ATTACH PARTITION " Child Schema "." Child Index "`)
	lookup := spec.PGIndexAttachmentLookup(s)
	if lookup == nil || lookup.Schema != " Parent Schema " || lookup.Name != " Parent Index " ||
		lookup.Qualifiers["child_schema"] != " Child Schema " || lookup.Qualifiers["child"] != " Child Index " {
		t.Fatalf("lookup=%+v", lookup)
	}
	encoded, err := json.Marshal(s.DDL)
	if err != nil || strings.Contains(string(encoded), " Child Index ") || strings.Contains(string(encoded), "PGIndexAttachment") {
		t.Fatalf("public DDL=%s (%v)", encoded, err)
	}
}
