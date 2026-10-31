package audit

// T17-R1 remediation tests (#96): prove the conditional CREATE SCHEMA arms
// only converge when the create arm is executable for every declared option,
// that unextracted schema options never publish a fabricated present, and
// that provider ResolveObject failures propagate through the real enrich
// call chain instead of degrading to unvouched gaps.

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t17ObjectResolver is a provider whose schema resolution answer is fully
// scriptable — Unavailable (unvouched), NotFound (proven absent),
// Confirmed+Exists (proven present), or a hard error. Every other provider
// method keeps the t05A8Provider stubs.
type t17ObjectResolver struct {
	t05A8Provider
	answer  *spec.ObjectSnapshot
	err     error
	calls   int
	version string
	cancel  context.CancelFunc
	lookups []spec.ObjectLookupRequest
}

func (p *t17ObjectResolver) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.calls++
	p.lookups = append(p.lookups, req)
	if p.cancel != nil {
		p.cancel()
	}
	if p.err != nil {
		return nil, p.err
	}
	if p.answer != nil {
		snap := *p.answer
		snap.Schema = req.Schema
		snap.Type = req.Type
		snap.Name = req.Name
		return &snap, nil
	}
	return &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
}

func (p *t17ObjectResolver) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: p.version}, nil
}

func t17Enrich(t *testing.T, dialect spec.Dialect, sql, schema string, version *spec.VersionIdentity, provider MetadataProvider) (*batchState, []spec.Statement) {
	t.Helper()
	ctx := context.Background()
	parsed, err := Parse(ctx, sql, dialect)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	extracted, err := Extract(ctx, parsed)
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	state := newBatchState(dialect, schema, provider)
	state.resolvedVersion = version
	facts, err := provider.LoadInstanceFacts(ctx, dialect, schema)
	if err != nil {
		t.Fatalf("load instance facts: %v", err)
	}
	enriched, err := state.enrichStatements(ctx, &MetadataRequest{Schema: schema, Provider: provider},
		extracted, parsed.failures, facts, auditLimits{orderedStatements: len(extracted)})
	if err != nil {
		t.Fatalf("enrich %q: %v", sql, err)
	}
	return state, enriched
}

var (
	t17v5744  = &spec.VersionIdentity{Major: 5, Minor: 7, Patch: 44}
	t17v8046  = &spec.VersionIdentity{Major: 8, Minor: 0, Patch: 46}
	t17Absent = &spec.ObjectSnapshot{Status: spec.MetadataStatusNotFound}
)

// B1: CREATE ... IF NOT EXISTS over an unvouched schema converges to present
// only when the create arm adjudicates Apply for every declared option.
func TestT17R1INEConvergesOnlyWhenCreateArmApplies(t *testing.T) {
	cases := []struct {
		name    string
		dialect spec.Dialect
		sql     string
		version *spec.VersionIdentity
		want    schemaKnowledge
	}{
		{"mysql-ine-charset-converges", spec.DialectMySQL, "CREATE DATABASE IF NOT EXISTS g CHARACTER SET utf8mb4;", t17v8046, schemaPresent},
		{"tidb-ine-charset-converges", spec.DialectTiDB, "CREATE DATABASE IF NOT EXISTS g CHARACTER SET utf8mb4;", nil, schemaPresent},
		{"mysql57-ine-encryption-refused-arm", spec.DialectMySQL, "CREATE DATABASE IF NOT EXISTS g DEFAULT ENCRYPTION='Y';", t17v5744, schemaUnknown},
		{"mysql-nilver-ine-encryption-withheld-arm", spec.DialectMySQL, "CREATE DATABASE IF NOT EXISTS g DEFAULT ENCRYPTION='Y';", nil, schemaUnknown},
		{"mysql8046-ine-encryption-converges", spec.DialectMySQL, "CREATE DATABASE IF NOT EXISTS g DEFAULT ENCRYPTION='Y';", t17v8046, schemaPresent},
		{"tidb-ine-placement-withheld-arm", spec.DialectTiDB, "CREATE DATABASE IF NOT EXISTS g PLACEMENT POLICY=p1;", nil, schemaUnknown},
		{"mysql-ine-tiflash-vendor-refused-arm", spec.DialectMySQL, "CREATE DATABASE IF NOT EXISTS g SET TIFLASH REPLICA 1;", t17v8046, schemaUnknown},
		{"tidb-ine-tiflash-unproven-arm", spec.DialectTiDB, "CREATE DATABASE IF NOT EXISTS g SET TIFLASH REPLICA 1;", nil, schemaUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state, _ := t17Enrich(t, tc.dialect, tc.sql, "req", tc.version, &t17ObjectResolver{})
			entry := state.schemaEntries["g"]
			if entry == nil {
				t.Fatalf("no schemaEntries entry for g")
			}
			if entry.state != tc.want {
				t.Errorf("state = %v, want %v", entry.state, tc.want)
			}
			if tc.want == schemaPresent && entry.batchCreated {
				t.Errorf("convergent INE must keep batchCreated=false")
			}
			if tc.want == schemaPresent && len(entry.attrs) != 0 {
				t.Errorf("convergent INE over unvouched must withhold authored attributes, got %v", entry.attrs)
			}
		})
	}
}

// B1 continuation: a bare CREATE over unvouched keeps the entry unvouched;
// the refusable option only matters when the statement could converge.
func TestT17R1BareCreateOverUnvouchedStaysUnknown(t *testing.T) {
	for _, tc := range []struct {
		name    string
		dialect spec.Dialect
		sql     string
	}{
		{"mysql", spec.DialectMySQL, "CREATE DATABASE g DEFAULT ENCRYPTION='Y';"},
		{"mysql-charset", spec.DialectMySQL, "CREATE DATABASE g CHARACTER SET utf8mb4;"},
		{"tidb-placement", spec.DialectTiDB, "CREATE DATABASE g PLACEMENT POLICY=p1;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, _ := t17Enrich(t, tc.dialect, tc.sql, "req", t17v8046, &t17ObjectResolver{})
			if entry := state.schemaEntries["g"]; entry == nil || entry.state != schemaUnknown {
				t.Fatalf("bare create over unvouched must stay unknown, got %+v", entry)
			}
		})
	}
}

// B2: CREATE over a proven-absent schema must not publish present when the
// statement carries an option the model cannot adjudicate — MySQL refuses
// vendor clauses outright (pre-state kept), TiDB's outcome is unproven.
func TestT17R1AbsentCreateUnextractedOptions(t *testing.T) {
	cases := []struct {
		name        string
		dialect     spec.Dialect
		sql         string
		version     *spec.VersionIdentity
		want        schemaKnowledge
		wantCreated bool
		wantCharset string
	}{
		{"mysql-charset-publishes", spec.DialectMySQL, "CREATE DATABASE g CHARACTER SET utf8mb4;", t17v8046, schemaPresent, true, "utf8mb4"},
		{"tidb-charset-publishes", spec.DialectTiDB, "CREATE DATABASE g CHARACTER SET utf8mb4;", nil, schemaPresent, true, "utf8mb4"},
		{"mysql-tiflash-vendor-refused", spec.DialectMySQL, "CREATE DATABASE g SET TIFLASH REPLICA 1;", t17v8046, schemaAbsent, false, ""},
		{"tidb-tiflash-unproven", spec.DialectTiDB, "CREATE DATABASE g SET TIFLASH REPLICA 1;", nil, schemaUnknown, false, ""},
		{"mysql-ine-tiflash-vendor-refused", spec.DialectMySQL, "CREATE DATABASE IF NOT EXISTS g SET TIFLASH REPLICA 1;", t17v8046, schemaAbsent, false, ""},
		{"tidb-ine-tiflash-unproven", spec.DialectTiDB, "CREATE DATABASE IF NOT EXISTS g SET TIFLASH REPLICA 1;", nil, schemaUnknown, false, ""},
		{"mysql57-encryption-refused", spec.DialectMySQL, "CREATE DATABASE g DEFAULT ENCRYPTION='Y';", t17v5744, schemaAbsent, false, ""},
		{"mysql-nilver-encryption-withheld", spec.DialectMySQL, "CREATE DATABASE g DEFAULT ENCRYPTION='Y';", nil, schemaUnknown, false, ""},
		{"tidb-placement-withheld", spec.DialectTiDB, "CREATE DATABASE g PLACEMENT POLICY=p1;", nil, schemaUnknown, false, ""},
		{"mysql-placement-vendor-refused", spec.DialectMySQL, "CREATE DATABASE g PLACEMENT POLICY=p1;", t17v8046, schemaAbsent, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &t17ObjectResolver{answer: t17Absent}
			state, _ := t17Enrich(t, tc.dialect, tc.sql, "req", tc.version, p)
			entry := state.schemaEntries["g"]
			if entry == nil {
				t.Fatalf("no schemaEntries entry for g")
			}
			if entry.state != tc.want {
				t.Errorf("state = %v, want %v", entry.state, tc.want)
			}
			if entry.batchCreated != tc.wantCreated {
				t.Errorf("batchCreated = %v, want %v", entry.batchCreated, tc.wantCreated)
			}
			if tc.wantCharset == "" && len(entry.attrs) != 0 {
				t.Errorf("no attributes may publish, got %v", entry.attrs)
			}
			if tc.wantCharset != "" && entry.attrs["charset"] != tc.wantCharset {
				t.Errorf("charset = %q, want %q", entry.attrs["charset"], tc.wantCharset)
			}
			if tc.wantCharset != "" {
				collation := "utf8mb4_0900_ai_ci"
				if tc.dialect == spec.DialectTiDB {
					collation = "utf8mb4_bin"
				}
				if !reflect.DeepEqual(entry.attrs, map[string]string{"charset": tc.wantCharset, "collation": collation}) {
					t.Errorf("attributes = %#v", entry.attrs)
				}
			}
			if p.calls != 1 || !reflect.DeepEqual(t17Absent, &spec.ObjectSnapshot{Status: spec.MetadataStatusNotFound}) {
				t.Fatalf("calls = %d, provider snapshot = %#v", p.calls, t17Absent)
			}
		})
	}
}

// B2 blast radius: a vendor-refused schema create must not flip the 1049 gate
// — a subsequent CREATE TABLE under it resolves the table honestly instead of
// inheriting a fabricated writable namespace.
func TestT17R1RefusedSchemaDoesNotFabricateWritableNamespace(t *testing.T) {
	state, statements := t17Enrich(t, spec.DialectMySQL,
		"CREATE DATABASE g SET TIFLASH REPLICA 1;\nCREATE TABLE g.t (id INT);",
		"req", t17v8046, &t17ObjectResolver{answer: t17Absent})
	if entry := state.schemaEntries["g"]; entry == nil || entry.state != schemaAbsent {
		t.Fatalf("refused schema create must keep g absent, got %+v", entry)
	}
	key := state.keyFor("req", spec.Table{Schema: "g", Name: "t"})
	if entry := state.entries[key]; entry == nil || entry.state != tableAbsent {
		t.Fatalf("g.t entry = %#v, want absent", entry)
	}
	if len(statements) != 2 || statements[1].Metadata == nil || statements[1].Metadata.TargetTable == nil || statements[1].Metadata.TargetTable.Exists {
		t.Fatalf("g.t projection = %#v, want absent", statements)
	}
}

func TestT17R1KnownTemporaryTableSurvivesUnknownSchema(t *testing.T) {
	p := &t17ObjectResolver{}
	sql := "CREATE TEMPORARY TABLE g.t (id INT NOT NULL, v VARCHAR(20)) COMMENT='original'; " +
		"CREATE DATABASE g; CREATE TEMPORARY TABLE IF NOT EXISTS g.t (different INT) COMMENT='replacement';"
	state, statements := t17Enrich(t, spec.DialectMySQL, sql, "req", t17v8046, p)
	key := state.keyFor("req", spec.Table{Schema: "g", Name: "t"})
	entry := state.tempEntries[key]
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		t.Fatalf("temporary table entry = %#v", entry)
	}
	if schema := state.schemaEntries["g"]; schema == nil || schema.state != schemaUnknown {
		t.Fatalf("schema = %#v, want unknown", schema)
	}
	if len(statements) != 3 || statements[2].Metadata == nil || len(statements[2].Metadata.TargetTables) != 1 {
		t.Fatalf("missing duplicate CREATE projection: %#v", statements)
	}
	before := statements[2].Metadata.TargetTables[0].Snapshot
	if before == nil || !before.Exists || !before.Temporary || before.Table.Comment != "original" ||
		before.Options["comment"] != "original" || len(before.Columns) != 2 ||
		before.Columns[0].Name != "id" || before.Columns[0].Type != "int(11)" || !before.Columns[0].NotNull ||
		before.Columns[1].Name != "v" || before.Columns[1].Type != "varchar(20)" {
		t.Fatalf("old temporary table projection = %#v", before)
	}
	projectedPost := cloneTableSnapshot(entry.shape)
	projectedPost.Temporary = true
	if !reflect.DeepEqual(projectedPost, before) {
		t.Fatalf("temporary table changed: post=%#v pre=%#v", entry.shape, before)
	}
	if p.calls != 1 {
		t.Fatalf("schema resolver calls = %d, want 1", p.calls)
	}
}

func TestT17R1KnownPersistentTableSurvivesUnknownSchema(t *testing.T) {
	ctx := context.Background()
	parsed, err := Parse(ctx, "CREATE TABLE IF NOT EXISTS g.t (different INT)", spec.DialectMySQL)
	if err != nil {
		t.Fatal(err)
	}
	statements, err := Extract(ctx, parsed)
	if err != nil {
		t.Fatal(err)
	}
	state := newBatchState(spec.DialectMySQL, "req", &t17ObjectResolver{})
	key := state.keyFor("req", spec.Table{Schema: "g", Name: "t"})
	shape := &spec.TableSnapshot{Schema: "g", Exists: true, Table: &spec.Table{Schema: "g", Name: "t", Comment: "original"}, Columns: []spec.Column{{Name: "id", Type: "int(11)"}}}
	state.schemaEntries["g"] = &batchSchemaEntry{state: schemaUnknown}
	state.entries[key] = &batchTableEntry{state: tablePresent, shape: cloneTableSnapshot(shape), displaySchema: "g", displayTable: "t"}
	if err := state.apply(ctx, statements[0]); err != nil {
		t.Fatal(err)
	}
	if got := state.entries[key]; got == nil || got.state != tablePresent || !reflect.DeepEqual(got.shape, shape) {
		t.Fatalf("persistent table entry = %#v, want original %#v", got, shape)
	}
}

// B3: a provider ResolveObject error must reach the enrich caller — it is a
// live read failure, not an unvouched answer.
func TestT17R1ResolveObjectErrorPropagates(t *testing.T) {
	sentinel := errors.New("schemata read failed")
	p := &t17ObjectResolver{err: sentinel}
	ctx := context.Background()
	sql := "ALTER DATABASE g CHARACTER SET utf8mb4;"
	parsed, err := Parse(ctx, sql, spec.DialectMySQL)
	if err != nil {
		t.Fatal(err)
	}
	extracted, err := Extract(ctx, parsed)
	if err != nil {
		t.Fatal(err)
	}
	state := newBatchState(spec.DialectMySQL, "req", p)
	state.resolvedVersion = t17v8046
	facts, err := p.LoadInstanceFacts(ctx, spec.DialectMySQL, "req")
	if err != nil {
		t.Fatal(err)
	}
	enriched, err := state.enrichStatements(ctx, &MetadataRequest{Schema: "req", Provider: p},
		extracted, parsed.failures, facts, auditLimits{orderedStatements: len(extracted)})
	if !errors.Is(err, sentinel) || enriched != nil {
		t.Fatalf("provider ResolveObject failure must propagate; got enriched=%v", enriched)
	}
	if state.schemaEntries["g"] != nil {
		t.Errorf("a failed resolve must not mint a schema entry, got %+v", state.schemaEntries["g"])
	}
}

// B3 through the public audit path: Service.Audit → enrich → ResolveObject.
func TestT17R1AuditSQLPropagatesResolveError(t *testing.T) {
	sentinel := errors.New("schemata read failed")
	p := &t17ObjectResolver{err: sentinel}
	result, err := AuditSQL(context.Background(), Request{
		Dialect: spec.DialectMySQL,
		SQL:     "DROP DATABASE g;",
		Metadata: &MetadataRequest{
			Schema:   "req",
			Provider: p,
		},
	})
	if !errors.Is(err, sentinel) || len(result.Statements) != 0 {
		t.Fatal("public audit must surface the provider ResolveObject failure")
	}
}

// B3 cache behavior: a failed lookup is not cached — the entry never lands,
// so nothing downstream can read a poisoned fact.
func TestT17R1ResolveErrorIsNotCached(t *testing.T) {
	sentinel := errors.New("schemata read failed")
	p := &t17ObjectResolver{err: sentinel}
	ctx := context.Background()
	parsed, err := Parse(ctx, "DROP DATABASE IF EXISTS g;", spec.DialectMySQL)
	if err != nil {
		t.Fatal(err)
	}
	extracted, err := Extract(ctx, parsed)
	if err != nil {
		t.Fatal(err)
	}
	state := newBatchState(spec.DialectMySQL, "req", p)
	facts, err := p.LoadInstanceFacts(ctx, spec.DialectMySQL, "req")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.enrichStatements(ctx, &MetadataRequest{Schema: "req", Provider: p},
		extracted, parsed.failures, facts, auditLimits{orderedStatements: len(extracted)}); !errors.Is(err, sentinel) {
		t.Fatal("expected propagation")
	}
	if len(state.schemaEntries) != 0 {
		t.Errorf("failed resolve must leave no schema entries, got %v", state.schemaEntries)
	}
}

// Control: the same-name dedupe still holds — one provider call per schema
// across the SchemaTarget projection, the Objects seam, and the transition.
func TestT17R1SchemaResolvesOncePerName(t *testing.T) {
	p := &t17ObjectResolver{answer: &spec.ObjectSnapshot{Status: spec.MetadataStatusConfirmed, Exists: true}}
	state, _ := t17Enrich(t, spec.DialectMySQL,
		"CREATE DATABASE IF NOT EXISTS g CHARACTER SET utf8mb4;\nCREATE DATABASE g;",
		"req", t17v8046, p)
	if p.calls != 1 {
		t.Errorf("ResolveObject called %d times for one schema name, want 1", p.calls)
	}
	if entry := state.schemaEntries["g"]; entry == nil || entry.state != schemaPresent {
		t.Fatalf("provider-proven schema must stay present, got %+v", entry)
	}
}

func TestT17R1CreateCharsetPrestates(t *testing.T) {
	original := &spec.ObjectSnapshot{Status: spec.MetadataStatusConfirmed, Exists: true, Attributes: map[string]string{
		"charset": "latin1", "collation": "latin1_bin", "encryption": "yes", "placement_policy": "old",
	}}
	wantOriginal := &spec.ObjectSnapshot{Status: spec.MetadataStatusConfirmed, Exists: true, Attributes: map[string]string{
		"charset": "latin1", "collation": "latin1_bin", "encryption": "yes", "placement_policy": "old",
	}}
	for _, tc := range []struct {
		name   string
		answer *spec.ObjectSnapshot
		want   batchSchemaEntry
	}{
		{"present", original, batchSchemaEntry{state: schemaPresent, attrs: wantOriginal.Attributes}},
		{"absent", t17Absent, batchSchemaEntry{state: schemaPresent, attrs: map[string]string{"charset": "utf8mb4", "collation": "utf8mb4_0900_ai_ci"}, batchCreated: true}},
		{"unavailable", nil, batchSchemaEntry{state: schemaPresent}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &t17ObjectResolver{answer: tc.answer}
			state, statements := t17Enrich(t, spec.DialectMySQL, "CREATE DATABASE IF NOT EXISTS g CHARACTER SET utf8mb4; CREATE DATABASE IF NOT EXISTS g CHARACTER SET utf8mb4;", "req", t17v8046, p)
			if got := state.schemaEntries["g"]; got == nil || !reflect.DeepEqual(*got, tc.want) {
				t.Fatalf("entry = %#v, want %#v", got, tc.want)
			}
			projection := statements[1].Metadata.SchemaTarget
			wantProjection := &spec.ObjectSnapshot{Schema: "g", Type: "schema", Name: "g", Status: spec.MetadataStatusConfirmed, Exists: true, Attributes: map[string]string{}}
			for k, v := range tc.want.attrs {
				wantProjection.Attributes[k] = v
			}
			if !reflect.DeepEqual(projection, wantProjection) {
				t.Fatalf("projection = %#v, want %#v", projection, wantProjection)
			}
			if p.calls != 1 || len(p.events) != 0 {
				t.Fatalf("provider calls = %d, table events = %v", p.calls, p.events)
			}
			if !reflect.DeepEqual(original, wantOriginal) {
				t.Fatalf("provider snapshot changed: %#v", original)
			}
		})
	}
}

func TestT17R1RefusedOrWithheldSuccessorBindings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		dialect spec.Dialect
		version *spec.VersionIdentity
		option  string
	}{
		{"mysql57-encryption", spec.DialectMySQL, t17v5744, "DEFAULT ENCRYPTION='Y'"},
		{"mysql-unknown-version", spec.DialectMySQL, nil, "DEFAULT ENCRYPTION='Y'"},
		{"tidb-placement", spec.DialectTiDB, nil, "PLACEMENT POLICY=p1"},
		{"mysql-tiflash", spec.DialectMySQL, t17v8046, "SET TIFLASH REPLICA 1"},
		{"tidb-tiflash", spec.DialectTiDB, nil, "SET TIFLASH REPLICA 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &t17ObjectResolver{}
			sql := "CREATE DATABASE IF NOT EXISTS g " + tc.option + "; ALTER DATABASE g CHARACTER SET utf8mb4; CREATE TABLE g.t (id INT); ALTER TABLE g.t ADD COLUMN x INT; CREATE INDEX idx_x ON g.t(x);"
			state, statements := t17Enrich(t, tc.dialect, sql, "req", tc.version, p)
			if entry := state.schemaEntries["g"]; entry == nil || !reflect.DeepEqual(*entry, batchSchemaEntry{state: schemaUnknown}) {
				t.Fatalf("schema = %#v, want unknown with no attributes or ownership", entry)
			}
			if len(statements) != 5 || statements[1].Metadata == nil || statements[1].Metadata.SchemaTarget != nil {
				t.Fatalf("following schema projection = %#v", statements)
			}
			for _, i := range []int{2, 3} {
				md := statements[i].Metadata
				if md == nil || len(md.TargetTables) != 1 || md.TargetTables[0].Table.Schema != "g" || md.TargetTables[0].Table.Name != "t" {
					t.Fatalf("statement %d target binding = %#v", i, md)
				}
			}
			if statements[2].Metadata.TargetTables[0].Snapshot != nil {
				t.Fatalf("create table prestate fabricated: %#v", statements[2].Metadata.TargetTables[0])
			}
			if statements[3].Metadata.TargetTables[0].Snapshot != nil {
				t.Fatalf("alter table prestate fabricated: %#v", statements[3].Metadata.TargetTables[0])
			}
			if statements[4].Metadata == nil || statements[4].Metadata.TargetTable != nil {
				t.Fatalf("following index table prestate fabricated: %#v", statements[4].Metadata)
			}
			if p.calls != 1 || len(p.events) != 0 {
				t.Fatalf("provider calls = %d, table events = %v", p.calls, p.events)
			}
		})
	}
}

func TestT17R1RefusalDominatesSyntheticUnknownOption(t *testing.T) {
	for _, tc := range []struct {
		name    string
		dialect spec.Dialect
		version *spec.VersionIdentity
		sql     string
		extra   []string
		want    schemaKnowledge
	}{
		{"encryption-refusal", spec.DialectMySQL, t17v5744, "CREATE DATABASE g CHARACTER SET utf8mb4 DEFAULT ENCRYPTION='Y'", []string{"synthetic_unknown"}, schemaAbsent},
		{"mysql-vendor-refusal", spec.DialectMySQL, t17v8046, "CREATE DATABASE g CHARACTER SET utf8mb4", []string{"synthetic_unknown", "tiflash_replica"}, schemaAbsent},
		{"mysql-unknown", spec.DialectMySQL, t17v8046, "CREATE DATABASE g CHARACTER SET utf8mb4", []string{"synthetic_unknown"}, schemaUnknown},
		{"tidb-unknown", spec.DialectTiDB, nil, "CREATE DATABASE g CHARACTER SET utf8mb4", []string{"synthetic_unknown"}, schemaUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			parsed, err := Parse(ctx, tc.sql, tc.dialect)
			if err != nil {
				t.Fatal(err)
			}
			statements, err := Extract(ctx, parsed)
			if err != nil {
				t.Fatal(err)
			}
			statements[0].DDL.UnextractedOptions = tc.extra
			p := &t17ObjectResolver{answer: t17Absent}
			state := newBatchState(tc.dialect, "req", p)
			state.resolvedVersion = tc.version
			if err := state.apply(ctx, statements[0]); err != nil {
				t.Fatal(err)
			}
			if got := state.schemaEntries["g"]; got == nil || !reflect.DeepEqual(*got, batchSchemaEntry{state: tc.want}) {
				t.Fatalf("entry = %#v, want state %v with no attributes", got, tc.want)
			}
			if p.calls != 1 {
				t.Fatalf("resolver calls = %d", p.calls)
			}
		})
	}
}

func TestT17R1WithholdClearsOwnershipRefusalKeepsPrestate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version *spec.VersionIdentity
		want    batchSchemaEntry
	}{
		{"withhold", nil, batchSchemaEntry{state: schemaUnknown}},
		{"refused", t17v5744, batchSchemaEntry{state: schemaAbsent, attrs: map[string]string{"charset": "old"}, batchCreated: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			parsed, err := Parse(ctx, "CREATE DATABASE g DEFAULT ENCRYPTION='Y'", spec.DialectMySQL)
			if err != nil {
				t.Fatal(err)
			}
			statements, err := Extract(ctx, parsed)
			if err != nil {
				t.Fatal(err)
			}
			p := &t17ObjectResolver{}
			state := newBatchState(spec.DialectMySQL, "req", p)
			state.resolvedVersion = tc.version
			state.schemaEntries["g"] = &batchSchemaEntry{state: schemaAbsent, attrs: map[string]string{"charset": "old"}, batchCreated: true}
			if err := state.apply(ctx, statements[0]); err != nil {
				t.Fatal(err)
			}
			if got := state.schemaEntries["g"]; got == nil || !reflect.DeepEqual(*got, tc.want) || p.calls != 0 {
				t.Fatalf("entry=%#v calls=%d, want %#v and no read", got, p.calls, tc.want)
			}
		})
	}
}

func TestT17R1NonSchemaFirstObjectCacheMiss(t *testing.T) {
	answer := &spec.ObjectSnapshot{Status: spec.MetadataStatusConfirmed, Exists: true, Attributes: map[string]string{"sentinel": "first"}}
	p := &t17ObjectResolver{answer: answer}
	_, statements := t17Enrich(t, spec.DialectTiDB, "CREATE SEQUENCE s; DROP SEQUENCE s;", "req", nil, p)
	want := spec.ObjectSnapshot{Schema: "req", Type: "sequence", Name: "s", Status: spec.MetadataStatusConfirmed, Exists: true, Attributes: map[string]string{"sentinel": "first"}}
	if p.calls != 1 || len(statements) != 2 {
		t.Fatalf("resolver calls %d, statements %d", p.calls, len(statements))
	}
	for i, stmt := range statements {
		if stmt.Metadata == nil || !reflect.DeepEqual(stmt.Metadata.Objects, []spec.ObjectSnapshot{want}) {
			t.Fatalf("statement %d objects = %#v, want %#v", i, stmt.Metadata, want)
		}
	}
	if !reflect.DeepEqual(answer, &spec.ObjectSnapshot{Status: spec.MetadataStatusConfirmed, Exists: true, Attributes: map[string]string{"sentinel": "first"}}) {
		t.Fatalf("provider answer changed: %#v", answer)
	}
}

func TestT17R1ResolverFailurePublicAndEnrich(t *testing.T) {
	sentinel := errors.New("resolver sentinel")
	for _, tc := range []struct {
		name    string
		dialect spec.Dialect
		sql     string
		err     error
		cancel  bool
	}{
		{"sentinel", spec.DialectMySQL, "DROP DATABASE g", sentinel, false},
		{"canceled", spec.DialectMySQL, "DROP DATABASE g", context.Canceled, false},
		{"deadline", spec.DialectMySQL, "DROP DATABASE g", context.DeadlineExceeded, false},
		{"cancel-during-call", spec.DialectMySQL, "DROP DATABASE g", nil, true},
		{"tidb-placement", spec.DialectTiDB, "CREATE DATABASE IF NOT EXISTS g PLACEMENT POLICY=p1", sentinel, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			p := &t17ObjectResolver{err: tc.err}
			want := tc.err
			if tc.cancel {
				p.cancel = cancel
				want = context.Canceled
			}
			result, err := AuditSQL(ctx, Request{Dialect: tc.dialect, SQL: tc.sql, Metadata: &MetadataRequest{Schema: "req", Provider: p}})
			if !errors.Is(err, want) || len(result.Statements) != 0 || p.calls != 1 {
				t.Fatalf("audit result=%#v error=%v calls=%d, want %v and empty result", result, err, p.calls, want)
			}
			ctx2, cancel2 := context.WithCancel(context.Background())
			defer cancel2()
			p2 := &t17ObjectResolver{err: tc.err}
			if tc.cancel {
				p2.cancel = cancel2
			}
			parsed, err := Parse(ctx2, tc.sql, tc.dialect)
			if err != nil {
				t.Fatal(err)
			}
			statements, err := Extract(ctx2, parsed)
			if err != nil {
				t.Fatal(err)
			}
			state := newBatchState(tc.dialect, "req", p2)
			enriched, err := state.enrichStatements(ctx2, &MetadataRequest{Schema: "req", Provider: p2}, statements, parsed.failures, nil, auditLimits{orderedStatements: len(statements)})
			if !errors.Is(err, want) || enriched != nil || p2.calls != 1 {
				t.Fatalf("enrich=%#v error=%v calls=%d, want %v and no projections", enriched, err, p2.calls, want)
			}
		})
	}
}

func TestT17R1FailedSchemaLookupNeverCached(t *testing.T) {
	ctx := context.Background()
	sentinel := errors.New("resolver sentinel")
	p := &t17ObjectResolver{err: sentinel}
	parsed, err := Parse(ctx, "DROP DATABASE g", spec.DialectMySQL)
	if err != nil {
		t.Fatal(err)
	}
	statements, err := Extract(ctx, parsed)
	if err != nil {
		t.Fatal(err)
	}
	state := newBatchState(spec.DialectMySQL, "req", p)
	got, err := state.enrichStatements(ctx, &MetadataRequest{Schema: "req", Provider: p}, statements, parsed.failures, nil, auditLimits{orderedStatements: 1})
	if !errors.Is(err, sentinel) || got != nil || state.schemaEntries["g"] != nil {
		t.Fatalf("enrich: %v %#v %#v", err, got, state.schemaEntries)
	}
	if _, err = state.resolveSchema(ctx, "g"); !errors.Is(err, sentinel) || state.schemaEntries["g"] != nil {
		t.Fatalf("second lookup: %v %#v", err, state.schemaEntries)
	}
	p.err = nil
	p.answer = &spec.ObjectSnapshot{Status: spec.MetadataStatusConfirmed, Exists: true}
	entry, err := state.resolveSchema(ctx, "g")
	if err != nil || entry == nil || entry.state != schemaPresent || p.calls != 3 {
		t.Fatalf("success: entry=%#v err=%v calls=%d", entry, err, p.calls)
	}
}

func TestT17R1ResolverAnswersAreNotErrors(t *testing.T) {
	for _, answer := range []*spec.ObjectSnapshot{
		{Status: spec.MetadataStatusConfirmed, Exists: true},
		{Status: spec.MetadataStatusNotFound},
		{Status: spec.MetadataStatusUnavailable},
	} {
		t.Run(string(answer.Status), func(t *testing.T) {
			p := &t17ObjectResolver{answer: answer}
			result, err := AuditSQL(context.Background(), Request{Dialect: spec.DialectMySQL, SQL: "DROP DATABASE g", Metadata: &MetadataRequest{Schema: "req", Provider: p}})
			if err != nil || len(result.Statements) != 1 || p.calls != 1 {
				t.Fatalf("result=%#v err=%v calls=%d", result, err, p.calls)
			}
		})
	}
}

func TestT17R1PolicyDoesNotChangeUnknownSchema(t *testing.T) {
	const alterRule = "ddl.database.alter.exists.require"
	profiles := []struct {
		name, path string
		gap        bool
	}{
		{"default", "", true},
		{"disabled", t05PolicyPath(t, map[string]string{}), false},
		{"warning", t05PolicyPathConfigured(t, map[string]t05RuleConfig{alterRule: {level: "warning"}}), true},
	}
	for _, tc := range profiles {
		t.Run(tc.name, func(t *testing.T) {
			p := &t17ObjectResolver{}
			result, err := AuditSQL(context.Background(), Request{
				Dialect: spec.DialectMySQL, SQL: "CREATE DATABASE IF NOT EXISTS g DEFAULT ENCRYPTION='Y'; ALTER DATABASE g CHARACTER SET utf8mb4",
				ConfigPath: tc.path, Metadata: &MetadataRequest{Schema: "req", Provider: p},
			})
			if err != nil || len(result.Statements) != 2 || p.calls != 1 {
				t.Fatalf("result=%#v err=%v calls=%d", result, err, p.calls)
			}
			found := false
			for _, gap := range result.Statements[1].EvidenceGaps {
				if gap.RuleID == alterRule && gap.ReasonCode == "unknown_schema_state" && reflect.DeepEqual(gap.RequiredFacts, []string{"schema.existence"}) {
					found = true
				}
			}
			for _, finding := range result.Statements[1].Findings {
				if finding.RuleID == alterRule {
					t.Fatalf("false existence finding: %#v", finding)
				}
			}
			if found != tc.gap {
				t.Fatalf("alter gap = %v, want %v; gaps %#v", found, tc.gap, result.Statements[1].EvidenceGaps)
			}
		})
	}
}
