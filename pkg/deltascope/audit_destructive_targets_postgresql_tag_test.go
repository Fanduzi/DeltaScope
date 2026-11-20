//go:build postgresql

package deltascope

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgDestructiveSDKProvider struct {
	fakeMetadataProvider
}

func (p *pgDestructiveSDKProvider) LoadTableSnapshot(_ context.Context, _ Dialect, schema, table string) (*TableSnapshot, error) {
	p.tableCalls = append(p.tableCalls, schema+"/"+table)
	if table == "absent" {
		return &TableSnapshot{Schema: schema, PGRelation: PGRelationState{Name: table}}, nil
	}
	if table == "partition_root" {
		return &TableSnapshot{Schema: schema, Table: &Table{Name: table}, Exists: true, PGRelation: PGRelationState{Name: table, Kind: "p"}}, nil
	}
	return &TableSnapshot{Schema: schema, Table: &Table{Name: table}, Exists: true, PGRelation: PGRelationState{Name: table, Kind: "v"}}, nil
}

func (p *pgDestructiveSDKProvider) ResolveObject(_ context.Context, _ spec.Dialect, request spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if request.Type == "truncate_references" && len(request.PGDestructiveTargets) > 0 {
		return &spec.ObjectSnapshot{PGDestructiveReferences: &spec.PGDestructiveReferences{Targets: request.PGDestructiveTargets, BlockedPosition: 1}}, nil
	}
	return nil, nil
}

func TestAuditPGDestructiveOrderedExactTargets(t *testing.T) {
	p := &pgDestructiveSDKProvider{fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"}}}
	result, err := Audit(context.Background(), Request{SQL: `DROP TABLE IF EXISTS app.absent, " Exact.Schema "." V ", " Exact.Schema "." V "`, Dialect: DialectPostgreSQL, MetadataProvider: p})
	if err != nil || result.Verdict != VerdictReject || !slices.Equal(p.tableCalls, []string{"app/absent", " Exact.Schema / V "}) {
		t.Fatalf("result=%+v calls=%v err=%v", result, p.tableCalls, err)
	}
	count := 0
	for _, statement := range result.Statements {
		for _, finding := range statement.Findings {
			if finding.RuleID == "ddl.table.drop.exists.require" {
				count++
				if finding.Metadata["table"] != " V " || finding.Metadata["reason_code"] != "relation_wrong_kind" {
					t.Fatal(finding)
				}
			}
		}
	}
	body, err := json.Marshal(result)
	if err != nil || count != 1 || strings.Contains(string(body), "PGDestructiveTableRelations") {
		t.Fatalf("result=%s count=%d err=%v", body, count, err)
	}
}

func TestAuditPGTruncateInheritedReference(t *testing.T) {
	p := &pgDestructiveSDKProvider{fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"}}}
	result, err := Audit(context.Background(), Request{SQL: "TRUNCATE app.partition_root", Dialect: DialectPostgreSQL, MetadataProvider: p})
	if err != nil || result.Verdict != VerdictReject {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "ddl.table.truncate.inbound_fk" && finding.Metadata["reason_code"] == "truncate_referenced_by_external_foreign_key" {
			return
		}
	}
	t.Fatalf("missing inbound reference finding: %+v", result.Statements[0].Findings)
}
