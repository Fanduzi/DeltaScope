//go:build postgresql

// Package postgresql benchmarks PostgreSQL parser throughput for representative DDL/DCL SQL shapes.
// input: deterministic PostgreSQL SQL strings and parser benchmark loops
// output: benchmark timing and allocation measurements for parser evaluation
// pos: PostgreSQL-tagged parser benchmark coverage for cache evaluation
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	"context"
	"testing"
)

// benchPGCreateExtension is a representative CREATE EXTENSION statement.
const benchPGCreateExtension = "CREATE EXTENSION pg_trgm"

// benchPGGrant is a representative GRANT statement.
const benchPGGrant = "GRANT SELECT ON TABLE users TO analyst"

// benchPGAlterTable is a representative ALTER TABLE statement.
const benchPGAlterTable = "ALTER TABLE users ADD COLUMN age integer"

// benchPGCreateTable is a typical CREATE TABLE with constraints.
const benchPGCreateTable = `CREATE TABLE orders (
	id SERIAL PRIMARY KEY,
	user_id integer NOT NULL REFERENCES users(id),
	amount numeric(10,2),
	created_at timestamptz DEFAULT now()
)`

// benchPGMultiStatement is 5 mixed DDL and DCL statements.
const benchPGMultiStatement = `CREATE TABLE users (id SERIAL PRIMARY KEY, name varchar(100), email varchar(255) UNIQUE);
ALTER TABLE users ADD COLUMN age integer DEFAULT 0;
INSERT INTO users (id, name, email) VALUES (1, 'test', 'test@example.com');
DELETE FROM users WHERE id = 1;
CREATE EXTENSION pg_trgm`

func BenchmarkParseCreateExtension(b *testing.B) {
	p := New()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(ctx, benchPGCreateExtension)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseGrant(b *testing.B) {
	p := New()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(ctx, benchPGGrant)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseAlterTable(b *testing.B) {
	p := New()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(ctx, benchPGAlterTable)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseCreateTable(b *testing.B) {
	p := New()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(ctx, benchPGCreateTable)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseMultiStatement(b *testing.B) {
	p := New()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(ctx, benchPGMultiStatement)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseRepeatIdentical measures cost of parsing the same SQL string
// repeatedly — the pattern a cache would eliminate.
func BenchmarkParseRepeatIdentical(b *testing.B) {
	p := New()
	ctx := context.Background()
	sql := benchPGCreateTable
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(ctx, sql)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseRepeatVaried measures cost of parsing a small rotation of
// different SQL strings — simulating a realistic multi-statement workload.
func BenchmarkParseRepeatVaried(b *testing.B) {
	p := New()
	ctx := context.Background()
	sqls := []string{benchPGCreateExtension, benchPGGrant, benchPGAlterTable, benchPGCreateTable, benchPGMultiStatement}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(ctx, sqls[i%len(sqls)])
		if err != nil {
			b.Fatal(err)
		}
	}
}
