package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type routineTargetProvider struct {
	fakeMetadataProvider
	requests []spec.ObjectLookupRequest
}

func (p *routineTargetProvider) ResolveObject(_ context.Context, _ spec.Dialect, lookup spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if lookup.Type != "mysql_routine" {
		return nil, nil
	}
	p.requests = append(p.requests, lookup)
	if lookup.Schema == "CaseSchema" && lookup.Name == "CaseProc" && lookup.Qualifiers["kind"] == "procedure" {
		return &spec.ObjectSnapshot{MySQLRoutine: &spec.MySQLRoutineTarget{
			Name: spec.Table{Schema: lookup.Schema, Name: lookup.Name}, Kind: spec.RoutineKindProcedure,
		}}, nil
	}
	return nil, nil
}

func TestMySQLRoutineTargetUsesFreshExactIdentity(t *testing.T) {
	provider := &routineTargetProvider{fakeMetadataProvider: fakeMetadataProvider{instance: &spec.InstanceFacts{Version: "8.0.46"}}}
	result, err := AuditSQL(context.Background(), Request{
		SQL:      "CREATE PROCEDURE `CaseSchema`.`CaseProc`() SQL SECURITY INVOKER SELECT 1; ALTER PROCEDURE `CaseSchema`.`CaseProc` SQL SECURITY INVOKER",
		Dialect:  spec.DialectMySQL,
		Metadata: &MetadataRequest{Schema: "CaseSchema", Provider: provider},
	})
	if err != nil || len(result.Statements) != 2 || len(provider.requests) != 1 {
		t.Fatalf("result=%+v lookups=%+v error=%v", result, provider.requests, err)
	}
	if got := t05FindingsByRule(result, 0, "ddl.routine.target.valid"); len(got) != 1 || got[0].Metadata["reason_code"] != "routine_target_exists" {
		t.Fatalf("expected a bound duplicate routine, got %+v", got)
	}
	gaps := result.Statements[1].EvidenceGaps
	if len(gaps) != 1 || gaps[0].ReasonCode != "unproven_mysql_routine_target" {
		t.Fatalf("later DDL must withhold the initial catalog fact, got %+v", gaps)
	}
}
