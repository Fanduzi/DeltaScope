package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T32/#111: TiDB instance-management boundary classification — every
// ALTER INSTANCE form lands an explicit class (named unaudited boundary,
// vendor boundary, or parser error), the entry preserves the action kind
// and instance target identity, and the owed version-applicability
// premise is named so promotion can never silently claim support.
func TestT32TiDBAlterInstanceBoundaryIdentity(t *testing.T) {
	for _, c := range []struct {
		sql        string
		feature    string
		noRollback bool
	}{
		{"ALTER INSTANCE RELOAD TLS", "alter_instance.reload_tls", false},
		{"ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR", "alter_instance.reload_tls", true},
	} {
		u := t26OneBoundary(t, spec.DialectTiDB, c.sql)
		if u.Feature != c.feature {
			t.Fatalf("%s: feature=%q want %s", c.sql, u.Feature, c.feature)
		}
		if u.Metadata["boundary"] != "unaudited" {
			t.Fatalf("%s: boundary=%v want unaudited", c.sql, u.Metadata["boundary"])
		}
		if u.Metadata["object"] != "instance" {
			t.Fatalf("%s: object=%v want instance — target identity lost", c.sql, u.Metadata["object"])
		}
		if u.Metadata["gap_fact"] != spec.AlterInstanceFactVersionApplicability {
			t.Fatalf("%s: gap_fact=%v want %s — promotion premise unnamed",
				c.sql, u.Metadata["gap_fact"], spec.AlterInstanceFactVersionApplicability)
		}
		if v, _ := u.Metadata["no_rollback_on_error"].(bool); v != c.noRollback {
			t.Fatalf("%s: no_rollback_on_error=%v want %v", c.sql, u.Metadata["no_rollback_on_error"], c.noRollback)
		}
	}
}

func TestT32TiDBAlterInstanceVendorBoundary(t *testing.T) {
	// ROTATE INNODB MASTER KEY is MySQL-only grammar — under TiDB it is a
	// vendor boundary, never an unaudited TiDB statement.
	u := t26OneBoundary(t, spec.DialectTiDB, "ALTER INSTANCE ROTATE INNODB MASTER KEY")
	if u.Feature != "alter_instance.rotate_innodb_master_key" {
		t.Fatalf("feature=%q want alter_instance.rotate_innodb_master_key", u.Feature)
	}
	if u.Metadata["boundary"] != "vendor" {
		t.Fatalf("boundary=%v want vendor", u.Metadata["boundary"])
	}
}

func TestT32TiDBAlterInstanceParserNegatives(t *testing.T) {
	// The vendored TiDB grammar accepts only RELOAD TLS [NO ROLLBACK ON
	// ERROR]; every other spelling — including a fabricated instance
	// target — must stay a parser error, never a boundary or silent pass.
	for _, sql := range []string{
		"ALTER INSTANCE",
		"ALTER INSTANCE RELOAD",
		"ALTER INSTANCE RELOAD TLS NO ROLLBACK",
		"ALTER INSTANCE RELOAD TLS extra",
		"ALTER INSTANCE 'tidb-1:4000' RELOAD TLS",
		"ALTER INSTANCE i1 RELOAD TLS",
	} {
		result, err := t26Audit(t, spec.DialectTiDB, sql)
		if err == nil && len(result.Unsupported) == 0 {
			t.Fatalf("%q: unexpectedly clean audit", sql)
		}
		for _, u := range result.Unsupported {
			if u.Feature == "alter_instance.reload_tls" {
				t.Fatalf("%q: malformed form claimed the named boundary", sql)
			}
		}
	}
}
