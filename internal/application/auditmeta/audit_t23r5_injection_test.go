package auditmeta

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

// T23-R5: the feature-comment scan is quote-aware — user-authored COMMENT
// text mimicking /*T![ns] */ markers can never reach the snapshot facts a
// public Audit consumes. The differential pair audits the same TTL ALTER
// against the same columns, once with the marker inside a quoted string
// (proven-absent TTL → ttl.warn fires) and once as a real feature comment
// (catalog TTL present, enable unobserved → target_table.ttl_enable gap,
// no warning).
func TestT23R5QuotedCommentCannotAlterFindings(t *testing.T) {
	columns := []t08r1Column{
		{name: "id", colType: "bigint(20)"},
		{name: "created_at", colType: "datetime"},
	}
	const alterTTL = "ALTER TABLE r1_defaults TTL = created_at + INTERVAL 1 DAY;"

	t.Run("quoted fake marker stays user text", func(t *testing.T) {
		fixture := t08r1FixtureFor(spec.DialectTiDB, columns)
		fixture.pkClustered = "NO"
		fixture.createSQL = "CREATE TABLE `r1_defaults` (\n" +
			"  `id` bigint(20) NOT NULL,\n  `created_at` datetime DEFAULT NULL\n" +
			") ENGINE=InnoDB COMMENT='note /*T![ttl] TTL=`created_at` + INTERVAL 1 DAY */ /*T![auto_id_cache] AUTO_ID_CACHE=9 */ end'"
		provider := &t08r1CountingProvider{inner: mysqlmeta.NewProvider(openT08R1DB(t, fixture))}
		result := t08r1Audit(t, alterTTL, t08r1Policy(t, "ddl.tidb.ttl.warn"), spec.DialectTiDB, provider, 1)
		stmt := result.Statements[0]
		found := false
		for _, finding := range stmt.Findings {
			if finding.RuleID == "ddl.tidb.ttl.warn" {
				found = true
			}
			if finding.Metadata["auto_id_cache"] == "9" {
				t.Fatalf("quoted marker fabricated a catalog value: %+v", finding)
			}
		}
		if !found {
			t.Fatalf("proven-absent TTL must keep the add warning: %+v", stmt.Findings)
		}
		for _, gap := range stmt.EvidenceGaps {
			for _, fact := range gap.RequiredFacts {
				if fact == "target_table.ttl_enable" {
					t.Fatalf("quoted fake TTL must not fabricate an enable premise: %+v", gap)
				}
			}
		}
	})

	t.Run("real feature comment still reaches the snapshot", func(t *testing.T) {
		fixture := t08r1FixtureFor(spec.DialectTiDB, columns)
		fixture.pkClustered = "NO"
		fixture.createSQL = "CREATE TABLE `r1_defaults` (\n" +
			"  `id` bigint(20) NOT NULL,\n  `created_at` datetime DEFAULT NULL\n" +
			") ENGINE=InnoDB /*T![ttl] TTL=`created_at` + INTERVAL 1 DAY */"
		provider := &t08r1CountingProvider{inner: mysqlmeta.NewProvider(openT08R1DB(t, fixture))}
		result := t08r1Audit(t, alterTTL, t08r1Policy(t, "ddl.tidb.ttl.warn"), spec.DialectTiDB, provider, 1)
		stmt := result.Statements[0]
		for _, finding := range stmt.Findings {
			if finding.RuleID == "ddl.tidb.ttl.warn" {
				t.Fatalf("catalog TTL must suppress the add warning: %+v", finding)
			}
		}
		enableGap := false
		for _, gap := range stmt.EvidenceGaps {
			for _, fact := range gap.RequiredFacts {
				if fact == "target_table.ttl_enable" {
					enableGap = true
				}
			}
		}
		if !enableGap {
			t.Fatalf("real TTL comment must reach the snapshot: %+v", stmt.EvidenceGaps)
		}
	})
}
