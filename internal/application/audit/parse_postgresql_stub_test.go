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
	if !strings.Contains(err.Error(), "PG-capable build") {
		t.Fatalf("expected PG-capable build error message, got %v", err)
	}
}
