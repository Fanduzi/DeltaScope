package auditmeta

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

// T24-R13 D1 regression: mysqlClient must forward the optional
// LoadMySQLResourceGroupFacts channel to the real provider — the native
// run proved the CLI path silently skipped the RG fact preload because
// neither the auditmeta client nor the CLI wrapper forwarded it.

func TestMySQLClientForwardsResourceGroupFacts(t *testing.T) {
	db := openT22DB(t, map[string]t22Result{
		"show global variables": {
			cols: []string{"Variable_name", "Value"},
			rows: [][]driver.Value{{"read_only", "OFF"}, {"super_read_only", "OFF"}},
		},
		"select @@global.performance_schema": {
			cols: []string{"@@global.performance_schema"}, rows: [][]driver.Value{{"ON"}},
		},
		"select version()": {
			cols: []string{"version()"}, rows: [][]driver.Value{{"8.0.46"}},
		},
		"performance_schema.threads": {
			cols: []string{"resource_group"}, rows: [][]driver.Value{},
		},
		"performance_schema_thread_instances_lost": {
			cols: []string{"Variable_name", "Value"}, rows: [][]driver.Value{{"Performance_schema_thread_instances_lost", "0"}},
		},
		"resource_group_name": {
			cols: []string{"resource_group_name"}, rows: [][]driver.Value{{"usr_default"}},
		},
		"@@server_uuid": {
			cols: []string{"@@server_uuid"}, rows: [][]driver.Value{{"srv-uuid-auditmeta"}},
		},
		"Grants for u@%": {
			cols: []string{"Grants for u@%"}, rows: [][]driver.Value{{"GRANT ALL PRIVILEGES ON *.* TO 'u'@'%'"}},
		},
		"current_role()": {
			cols: []string{"current_role()"}, rows: [][]driver.Value{{"NONE"}},
		},
	})
	client := mysqlClient{db: db, provider: mysqlmeta.NewProvider(db)}

	facts, err := client.LoadMySQLResourceGroupFacts(context.Background(), spec.DialectMySQL)
	if err != nil {
		t.Fatalf("forward errored: %v", err)
	}
	if facts == nil {
		t.Fatal("forward returned nil facts — the optional channel is not reaching the provider")
	}
	if !facts.MySQLServerUUIDKnown || facts.MySQLServerUUID != "srv-uuid-auditmeta" {
		t.Fatalf("observed uuid lost in forwarding: %+v", facts)
	}
	if !facts.MySQLResourceGroupFeatureKnown || !facts.MySQLResourceGroupFeatureSupported {
		t.Fatalf("feature probe not forwarded: %+v", facts)
	}
}
