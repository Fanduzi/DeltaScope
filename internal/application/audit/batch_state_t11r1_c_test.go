// Package audit pins the T11-R1 cross-schema referenced-parent identity:
// a foreign key's referenced table resolves once under the effective
// identity — an explicitly qualified parent keeps its authored schema, an
// unqualified reference resolves inside the child's effective schema (the
// server's own rule) — and the same identity feeds the provider read, the
// cache key, the display identity, and the dependency edge. A request
// schema must never pollute another database's parent facts.
package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t11r1SchemaProvider answers snapshots keyed by "schema.table" and records
// every (schema,table) pair the engine actually asked for.
type t11r1SchemaProvider struct {
	version string
	tables  map[string]*spec.TableSnapshot
	calls   []string
}

func (p *t11r1SchemaProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: p.version}, nil
}

func (p *t11r1SchemaProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	p.calls = append(p.calls, schema+"."+table)
	if snap, ok := p.tables[strings.ToLower(schema)+"."+strings.ToLower(table)]; ok {
		return snap, nil
	}
	return &spec.TableSnapshot{Exists: false, Table: &spec.Table{Schema: schema, Name: table}}, nil
}

// t11r1Parent returns a parent whose referenced key is a proven primary
// key — the premise an add must satisfy once the right object is read.
func t11r1Parent(schema, table string, columns ...spec.Column) *spec.TableSnapshot {
	shape := t10r2Table(table, columns)
	shape.Schema = schema
	shape.Table = &spec.Table{Schema: schema, Name: table}
	shape.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{columns[0].Name}}
	return shape
}

// t11r1Audit runs the public audit under an explicit request schema and
// returns the result plus the provider's recorded reads.
func t11r1Audit(t *testing.T, dialect spec.Dialect, schema, sql string, provider *t11r1SchemaProvider, policy string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          dialect,
		Schema:           schema,
		ConfigPath:       policy,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return result
}

func t11r1DropFKPolicy(t *testing.T) string {
	return t05PolicyPath(t, map[string]string{"ddl.alter.drop_foreign_key.exists.require": ""})
}

// An unqualified REFERENCES resolves inside the child's effective schema.
// The provider must be asked for db_b.parent — never db_a.parent — and
// the member lands only when that real parent proves the key. The
// subsequent DROP FOREIGN KEY on the child distinguishes the outcomes:
// a published member answers no finding; a refused add (request-schema
// parent read by mistake, or a genuinely missing key) reports missing.
func TestT11R1CUnqualifiedParentBindsChildSchema(t *testing.T) {
	t.Parallel()
	policy := t11r1DropFKPolicy(t)
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			version := "8.0.46"
			if dialect == spec.DialectTiDB {
				version = "8.0.11-TiDB-v8.5.0"
			}
			provider := &t11r1SchemaProvider{
				version: version,
				tables: map[string]*spec.TableSnapshot{
					"db_b.child":  t11fkShape(),
					"db_b.parent": t11r1Parent("db_b", "parent", t10r2Col("id", "int", true, 0)),
					// The request-schema parent exists but cannot serve
					// this reference — it must never be read for it.
					"db_a.parent": t11r1Parent("db_a", "parent", t10r2Col("other", "int", true, 0)),
				},
			}
			result := t11r1Audit(t, dialect, "db_a",
				"ALTER TABLE db_b.child ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES parent(id);\n"+
					"ALTER TABLE db_b.child DROP FOREIGN KEY fk_c;",
				provider, policy)
			readParent := false
			for _, call := range provider.calls {
				if call == "db_b.parent" {
					readParent = true
				}
				if call == "db_a.parent" {
					t.Fatalf("provider was asked for the request-schema parent: calls=%v", provider.calls)
				}
			}
			if !readParent {
				t.Fatalf("provider was never asked for the child-schema parent: calls=%v", provider.calls)
			}
			drop := result.Statements[1]
			if len(drop.Findings) != 0 {
				t.Fatalf("published member must answer the drop cleanly: %#v", drop.Findings)
			}
			if len(drop.EvidenceGaps) != 0 {
				t.Fatalf("proven member drop owes no gap: %#v", drop.EvidenceGaps)
			}
		})
	}
}

// When the child-schema parent genuinely lacks the referenced key, the
// unqualified reference is a proven refusal — no member publishes and the
// subsequent drop answers missing.
func TestT11R1CUnqualifiedParentRefusalStaysInChildSchema(t *testing.T) {
	t.Parallel()
	policy := t11r1DropFKPolicy(t)
	provider := &t11r1SchemaProvider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"db_b.child":  t11fkShape(),
			"db_b.parent": t11r1Parent("db_b", "parent", t10r2Col("other", "int", true, 0)),
			// Present for contrast only — the engine must not consult it.
			"db_a.parent": t11r1Parent("db_a", "parent", t10r2Col("id", "int", true, 0)),
		},
	}
	result := t11r1Audit(t, spec.DialectMySQL, "db_a",
		"ALTER TABLE db_b.child ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES parent(id);\n"+
			"ALTER TABLE db_b.child DROP FOREIGN KEY fk_c;",
		provider, policy)
	for _, call := range provider.calls {
		if call == "db_a.parent" {
			t.Fatalf("provider was asked for the request-schema parent: calls=%v", provider.calls)
		}
	}
	drop := result.Statements[1]
	if len(drop.Findings) != 1 || drop.Findings[0].RuleID != "ddl.alter.drop_foreign_key.exists.require" {
		t.Fatalf("refused add must leave fk_c provably missing: %#v", drop.Findings)
	}
}

// An explicitly qualified parent keeps its authored schema even when the
// child lives elsewhere — the same identity feeds the read and the member
// landing.
func TestT11R1CExplicitParentSchemaHonored(t *testing.T) {
	t.Parallel()
	policy := t11r1DropFKPolicy(t)
	provider := &t11r1SchemaProvider{
		version: "8.0.46",
		tables: map[string]*spec.TableSnapshot{
			"db_b.child":  t11fkShape(),
			"db_a.parent": t11r1Parent("db_a", "parent", t10r2Col("id", "int", true, 0)),
			"db_b.parent": t11r1Parent("db_b", "parent", t10r2Col("other", "int", true, 0)),
		},
	}
	result := t11r1Audit(t, spec.DialectMySQL, "db_a",
		"ALTER TABLE db_b.child ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES db_a.parent(id);\n"+
			"ALTER TABLE db_b.child DROP FOREIGN KEY fk_c;",
		provider, policy)
	readExplicit := false
	for _, call := range provider.calls {
		if call == "db_a.parent" {
			readExplicit = true
		}
		if call == "db_b.parent" {
			t.Fatalf("provider was asked for the child-schema parent despite the explicit qualifier: calls=%v", provider.calls)
		}
	}
	if !readExplicit {
		t.Fatalf("provider never asked for the explicitly qualified parent: calls=%v", provider.calls)
	}
	if len(result.Statements[1].Findings) != 0 {
		t.Fatalf("member under the explicit parent must publish: %#v", result.Statements[1].Findings)
	}
}

// A self-reference on a qualified child binds the child's own shape under
// its own schema — no provider read is issued against a foreign table.
func TestT11R1CSelfReferenceUsesChildShape(t *testing.T) {
	t.Parallel()
	policy := t11r1DropFKPolicy(t)
	child := t11fkShape()
	child.Schema = "db_b"
	child.Table = &spec.Table{Schema: "db_b", Name: "child"}
	child.PrimaryKey = &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"c"}}
	provider := &t11r1SchemaProvider{
		version: "8.0.46",
		tables:  map[string]*spec.TableSnapshot{"db_b.child": child},
	}
	result := t11r1Audit(t, spec.DialectMySQL, "db_a",
		"ALTER TABLE db_b.child ADD CONSTRAINT fk_c FOREIGN KEY (a) REFERENCES child(c);\n"+
			"ALTER TABLE db_b.child DROP FOREIGN KEY fk_c;",
		provider, policy)
	for _, call := range provider.calls {
		if call != "db_b.child" {
			t.Fatalf("self-reference must not read a foreign table: calls=%v", provider.calls)
		}
	}
	if len(result.Statements[1].Findings) != 0 {
		t.Fatalf("self-referenced member must publish: %#v", result.Statements[1].Findings)
	}
}
