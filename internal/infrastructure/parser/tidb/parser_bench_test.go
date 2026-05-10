// Package tidbparser benchmarks TiDB/MySQL parser throughput for representative SQL shapes.
// input: deterministic DDL/DML SQL strings and parser benchmark loops
// output: benchmark timing and allocation measurements for parser evaluation
// pos: parser benchmark coverage for cache evaluation
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"testing"
)

// benchDMLSimple is a minimal DELETE statement.
const benchDMLSimple = "DELETE FROM users WHERE id = 1"

// benchDDLAlter is a representative ALTER TABLE statement.
const benchDDLAlter = "ALTER TABLE users ADD COLUMN age INT"

// benchDDLCreate is a typical CREATE TABLE with constraints and indexes.
const benchDDLCreate = `CREATE TABLE orders (
	id INT PRIMARY KEY,
	user_id INT NOT NULL,
	amount DECIMAL(10,2),
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id),
	INDEX idx_user (user_id),
	INDEX idx_amount (amount)
)`

// benchMultiStatement is 5 mixed DDL and DML statements.
const benchMultiStatement = `CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100), email VARCHAR(255) UNIQUE);
ALTER TABLE users ADD COLUMN age INT DEFAULT 0;
INSERT INTO users (id, name, email) VALUES (1, 'test', 'test@example.com');
DELETE FROM users WHERE id = 1;
CREATE INDEX idx_name ON users (name)`

func BenchmarkParseDMLSimple(b *testing.B) {
	p := New()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(ctx, benchDMLSimple)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseDDLAlter(b *testing.B) {
	p := New()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(ctx, benchDDLAlter)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseDDLCreate(b *testing.B) {
	p := New()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(ctx, benchDDLCreate)
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
		_, err := p.Parse(ctx, benchMultiStatement)
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
	sql := benchDDLCreate
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
	sqls := []string{benchDMLSimple, benchDDLAlter, benchDDLCreate, benchMultiStatement}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(ctx, sqls[i%len(sqls)])
		if err != nil {
			b.Fatal(err)
		}
	}
}
