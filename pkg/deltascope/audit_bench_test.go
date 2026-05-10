// Package deltascope benchmarks the public audit API on representative MySQL/TiDB SQL inputs.
// input: deterministic public audit requests without metadata providers
// output: benchmark timing and allocation measurements for public API cache evaluation
// pos: public API benchmark coverage for parser/cache evaluation
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"testing"
)

const benchAuditSimpleDDL = "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100), email VARCHAR(255) UNIQUE)"

const benchAuditMixed = `CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100) NOT NULL, email VARCHAR(255) UNIQUE);
ALTER TABLE users ADD COLUMN age INT DEFAULT 0;
INSERT INTO users (id, name, email) VALUES (1, 'test', 'test@example.com');
DELETE FROM users WHERE id = 1`

func BenchmarkAuditPublicSimpleDDL(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Audit(ctx, Request{
			SQL:     benchAuditSimpleDDL,
			Dialect: DialectMySQL,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuditPublicMixed(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Audit(ctx, Request{
			SQL:     benchAuditMixed,
			Dialect: DialectMySQL,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuditPublicTiDBSimple(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Audit(ctx, Request{
			SQL:     benchAuditSimpleDDL,
			Dialect: DialectTiDB,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuditPublicRepeatIdentical(b *testing.B) {
	ctx := context.Background()
	sql := benchAuditMixed
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Audit(ctx, Request{
			SQL:     sql,
			Dialect: DialectMySQL,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
