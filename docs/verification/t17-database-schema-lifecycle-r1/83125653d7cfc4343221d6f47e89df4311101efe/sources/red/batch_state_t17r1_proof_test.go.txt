package audit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// The CLI intentionally omits internal metadata. These companion assertions
// bind each new Golden SQL input to its actual ordered enrichment projections.
type t17R1ProofProvider struct {
	status      spec.MetadataStatus
	version     string
	err         error
	objectCalls int
	tableCalls  int
}

func (p *t17R1ProofProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	if p.version == "" {
		return nil, nil
	}
	return &spec.InstanceFacts{Version: p.version}, nil
}

func (p *t17R1ProofProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, name string) (*spec.TableSnapshot, error) {
	p.tableCalls++
	return &spec.TableSnapshot{Schema: schema, Table: &spec.Table{Schema: schema, Name: name}}, nil
}

func (p *t17R1ProofProvider) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.objectCalls++
	if p.err != nil {
		return nil, p.err
	}
	return &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Status: p.status}, nil
}

type t17R1GoldenInput struct {
	ID      string       `json:"id"`
	SQL     string       `json:"sql"`
	Dialect spec.Dialect `json:"dialect"`
}

func TestT17R1GoldenProjections(t *testing.T) {
	path := os.Getenv("T17R1_MANIFEST")
	if path == "" {
		path = filepath.Join("..", "..", "..", "testdata", "ddl-golden", "T17.json")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		CLI      []t17R1GoldenInput `json:"cli_cases"`
		Metadata []t17R1GoldenInput `json:"metadata_cases"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	inputs := make(map[string]t17R1GoldenInput)
	for _, input := range append(manifest.CLI, manifest.Metadata...) {
		if _, duplicate := inputs[input.ID]; duplicate {
			t.Fatalf("duplicate manifest id %s", input.ID)
		}
		inputs[input.ID] = input
	}
	cases := []struct {
		id, sql, schema, version string
		dialect                  spec.Dialect
		status                   spec.MetadataStatus
		want                     schemaKnowledge
	}{
		{"t17-cli-ine-encryption-57-mysql",
			"CREATE DATABASE IF NOT EXISTS encg DEFAULT ENCRYPTION='Y'; CREATE DATABASE encg; CREATE TABLE encg.t (id INT); CREATE TABLE encg.t (id INT);",
			"encg", "5.7.44", spec.DialectMySQL, spec.MetadataStatusUnavailable, schemaUnknown},
		{"t17-cli-ine-encryption-nilver-mysql",
			"CREATE DATABASE IF NOT EXISTS encg DEFAULT ENCRYPTION='Y'; CREATE DATABASE encg; CREATE TABLE encg.t (id INT); CREATE TABLE encg.t (id INT);",
			"encg", "", spec.DialectMySQL, spec.MetadataStatusUnavailable, schemaUnknown},
		{"t17-cli-ine-placement-tidb",
			"CREATE DATABASE IF NOT EXISTS pg PLACEMENT POLICY=p1; CREATE DATABASE pg; CREATE TABLE pg.t (id INT); CREATE TABLE pg.t (id INT);",
			"pg", "8.5.0", spec.DialectTiDB, spec.MetadataStatusUnavailable, schemaUnknown},
		{"t17-mysql80-tiflash-refused",
			"CREATE DATABASE t17tf SET TIFLASH REPLICA 1; ALTER DATABASE t17tf CHARACTER SET latin1; CREATE TABLE t17tf.t (id INT); CREATE TABLE t17tf.t (id INT);",
			"t17tf", "8.0.46", spec.DialectMySQL, spec.MetadataStatusNotFound, schemaAbsent},
		{"t17-tidb85-tiflash-unproven",
			"CREATE DATABASE t17tf SET TIFLASH REPLICA 1; CREATE DATABASE t17tf; CREATE TABLE t17tf.t (id INT); CREATE TABLE t17tf.t (id INT);",
			"t17tf", "8.5.0", spec.DialectTiDB, spec.MetadataStatusNotFound, schemaUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			input, ok := inputs[tc.id]
			if !ok || input.SQL != tc.sql || input.Dialect != tc.dialect {
				t.Fatalf("frozen Golden input differs: %+v", input)
			}
			p := &t17R1ProofProvider{status: tc.status}
			ctx := context.Background()
			parsed, err := Parse(ctx, input.SQL, input.Dialect)
			if err != nil {
				t.Fatal(err)
			}
			statements, err := Extract(ctx, parsed)
			if err != nil {
				t.Fatal(err)
			}
			state := newBatchState(input.Dialect, "request_default", p)
			if tc.version != "" {
				state.resolvedVersion, err = parseAuditTargetVersion(tc.dialect, tc.version)
				if err != nil {
					t.Fatal(err)
				}
			}
			enriched, err := state.enrichStatements(ctx,
				&MetadataRequest{Schema: "request_default", Provider: p},
				statements, parsed.failures, nil, auditLimits{orderedStatements: 4})
			if err != nil || len(enriched) != 4 {
				t.Fatalf("enrich: statements=%d err=%v", len(enriched), err)
			}
			projections := make([]*spec.Metadata, len(enriched))
			for i := range enriched {
				projections[i] = enriched[i].Metadata
			}
			postState := state.schemaEntries[tc.schema]
			var postStateEvidence any
			if postState != nil {
				postStateEvidence = map[string]any{
					"state": postState.state, "batch_created": postState.batchCreated, "attributes": postState.attrs,
				}
			}
			evidence, err := json.Marshal(map[string]any{
				"id": tc.id, "sql": input.SQL, "dialect": input.Dialect,
				"version": tc.version, "provider_status": p.status,
				"projections": projections, "schema_post_state": postStateEvidence,
				"object_calls": p.objectCalls, "table_calls": p.tableCalls,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("T17R1_PROOF %s", evidence)
			for i := 0; i < 2; i++ {
				target := enriched[i].MetadataSchemaTarget()
				wantAbsent := tc.want == schemaAbsent || i == 0 && tc.status == spec.MetadataStatusNotFound
				if wantAbsent {
					want := &spec.ObjectSnapshot{Schema: tc.schema, Type: "schema", Name: tc.schema, Status: spec.MetadataStatusNotFound}
					if !reflect.DeepEqual(target, want) {
						t.Errorf("statement %d SchemaTarget=%+v, want %+v", i, target, want)
					}
				} else if target != nil {
					t.Errorf("statement %d SchemaTarget=%+v, want nil (unproven)", i, target)
				}
			}
			for i := 2; i < 4; i++ {
				metadata := enriched[i].Metadata
				if metadata == nil || len(metadata.TargetTables) != 1 {
					t.Fatalf("statement %d missing target binding: %+v", i, metadata)
				}
				binding := metadata.TargetTables[0]
				if binding.Table.Schema != tc.schema || binding.Table.Name != "t" || binding.Ambiguous {
					t.Errorf("statement %d bound wrong identity: %+v", i, binding)
				}
				if tc.want == schemaUnknown {
					if binding.Snapshot != nil || metadata.TargetTable != nil {
						t.Errorf("statement %d fabricated table state: %+v", i, metadata)
					}
				} else if binding.Snapshot == nil || binding.Snapshot.Exists {
					t.Errorf("statement %d did not preserve 1049 absence: %+v", i, binding.Snapshot)
				}
			}
			entry := state.schemaEntries[tc.schema]
			if entry == nil || entry.state != tc.want || entry.batchCreated || len(entry.attrs) != 0 {
				t.Errorf("schema post-state=%+v, want state=%v, batchCreated=false, no attributes", entry, tc.want)
			}
			if p.objectCalls != 1 || p.tableCalls != 0 {
				t.Errorf("provider ledger: object=%d table=%d, want 1/0", p.objectCalls, p.tableCalls)
			}
		})
	}
}

func TestT17R1ProviderErrorProof(t *testing.T) {
	ctx := context.Background()
	sql := "CREATE DATABASE IF NOT EXISTS broken; CREATE TABLE broken.t (id INT);"
	sentinel := errors.New("T17-R1 injected SCHEMATA read failure")
	p := &t17R1ProofProvider{status: spec.MetadataStatusUnavailable, version: "8.0.46", err: sentinel}
	result, auditErr := AuditSQL(ctx, Request{
		SQL: sql, Dialect: spec.DialectMySQL,
		Metadata: &MetadataRequest{Schema: "request_default", Provider: p},
	})
	parsed, err := Parse(ctx, sql, spec.DialectMySQL)
	if err != nil {
		t.Fatal(err)
	}
	statements, err := Extract(ctx, parsed)
	if err != nil {
		t.Fatal(err)
	}
	state := newBatchState(spec.DialectMySQL, "request_default", p)
	enriched, enrichErr := state.enrichStatements(ctx,
		&MetadataRequest{Schema: "request_default", Provider: p},
		statements, parsed.failures, nil, auditLimits{orderedStatements: 2})
	evidence, err := json.Marshal(map[string]any{
		"sql": sql, "dialect": spec.DialectMySQL, "version": p.version,
		"provider_status": p.status, "provider_error": sentinel.Error(),
		"audit_error": errorTextT17R1(auditErr), "audit_result": result,
		"enrich_error": errorTextT17R1(enrichErr), "enriched": enriched,
		"schema_entries": len(state.schemaEntries), "object_calls": p.objectCalls,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("T17R1_PROOF %s", evidence)
	if !errors.Is(auditErr, sentinel) || !errors.Is(enrichErr, sentinel) {
		t.Errorf("errors not propagated: audit=%v enrich=%v", auditErr, enrichErr)
	}
	if len(result.Statements) != 0 || len(enriched) != 0 || len(state.schemaEntries) != 0 {
		t.Error("failed provider read fabricated statements or schema state")
	}
}

func errorTextT17R1(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
