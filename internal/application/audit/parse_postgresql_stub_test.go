//go:build !postgresql

package audit

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestParseReturnsUnsupportedErrorForPostgreSQLWithoutBuildTag(t *testing.T) {
	_, err := Parse("select 1;", spec.DialectPostgreSQL)
	if err == nil {
		t.Fatal("expected unsupported postgresql error")
	}
	if !strings.Contains(err.Error(), "PG-capable DeltaScope build") {
		t.Fatalf("expected unified PG-capable build guidance, got %v", err)
	}
	if strings.Contains(err.Error(), "deltascope-pg") {
		t.Fatalf("did not expect legacy deltascope-pg guidance, got %v", err)
	}
}
